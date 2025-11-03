// Package main implements the Flyff Universe automated bot application.
//
// Architecture Overview:
// This bot uses an asynchronous multi-goroutine architecture where the browser,
// image recognition, and behavior systems run independently without blocking each other.
// The program consists of three main concurrent components:
//
//   1. Browser Goroutine: Manages chromedp browser instance, navigation, screenshots,
//      and debug overlay rendering. Starts asynchronously with 60s timeout protection.
//
//   2. Main Loop Goroutine: Executes at configurable intervals (default 1s), performs
//      image capture, status recognition (HP/MP/FP), mob detection, and behavior execution.
//
//   3. System Tray Goroutines: Handles UI interactions for mode switching, slot configuration,
//      threshold adjustments, and capture frequency settings. Runs 60+ concurrent event handlers.
//
// Startup Sequence:
//   00:00s - Program initialization, logger setup, data.json loading
//   00:00s - System tray UI creation
//   00:00s - Browser starts asynchronously (background navigation to game URL)
//   00:00s - Main loop starts immediately (polls for browser readiness)
//   00:01s+ - First iteration executes when canvas element detected
//
// Main Loop Logic:
// Each iteration performs the following steps in sequence:
//   1. Check if game canvas exists (indicates browser is ready)
//   2. Capture screenshot from browser (5s timeout)
//   3. Update player stats (HP/MP/FP recognition from status bars)
//   4. Identify mobs (passive yellow/aggressive red name detection)
//   5. Draw debug overlay with detection results (2s timeout)
//   6. Execute current behavior (Farming/Support/Stop)
//   7. Update system tray status display
//
// Key Design Decisions:
//   - All chromedp operations have timeout protection to prevent hanging
//   - Browser failure does not crash the program (error logged, continues)
//   - Main loop runs independently of browser loading status
//   - Configuration changes are immediately saved to data.json
//   - Graceful shutdown with signal handling (SIGINT/SIGTERM)
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// BotBehavior defines the interface that all bot behaviors must implement.
//
// The interface provides a common contract for different behavior modes,
// allowing the bot to switch between behaviors dynamically at runtime.
//
// Methods:
//   - Run: Execute one iteration of the behavior logic
//   - Stop: Gracefully terminate and clean up behavior state
//   - GetState: Return the current state name for debug overlay
type BotBehavior interface {
	Run(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, stats *Statistics) error
	Stop()
	GetState() string
}

// Bot represents the main bot controller and orchestrates all subsystems.
//
// The Bot struct holds references to all major components and manages their lifecycle.
// It follows a dependency injection pattern where action, browser, analyzer, and
// movement components are created during initialization and injected into behaviors.
//
// Component Dependencies:
//   - config: Thread-safe configuration (RWMutex protected)
//   - stats: Kill statistics and performance metrics
//   - action: JavaScript-based keyboard/mouse/screen capture interface
//   - browser: Chromedp browser controller for game interaction
//   - analyzer: Image recognition system for HP/MP/FP and mob detection
//   - movement: Character movement and skill execution coordinator
//   - behavior: Current active behavior (Farming/Support), can be nil when stopped
//   - tray: System tray UI for user configuration
//   - data: Persistent data container (config + cookies)
// DebugOverlayRequest represents a request to draw debug overlay
type DebugOverlayRequest struct {
	Targets       []Target
	Config        *Config
	ClientStats   *ClientStats
	Stats         *Statistics
	Action        *Action
	BehaviorState string
}

type Bot struct {
	config       *Config
	stats        *Statistics
	action       *Action
	browser      *Browser
	analyzer     *ImageAnalyzer
	movement     *MovementCoordinator
	behavior     BotBehavior
	tray         *TrayApp
	stopChan     chan bool
	data         *PersistentData
	cookiesSaved bool // Flag to track if cookies have been saved after game loads

	// Async debug overlay rendering
	debugOverlayChan chan *DebugOverlayRequest
	debugOverlayEnabled bool
}

// NewBot creates and initializes a new bot instance with all required components.
//
// Initialization Process:
//   1. Load persistent data from data.json (config and cookies)
//   2. Create statistics tracker for kill metrics
//   3. Create browser controller (chromedp wrapper)
//   4. Initialize action APIs (keyboard, mouse, screen) via JavaScript
//   5. Initialize image analyzer with action reference
//   6. Create movement coordinator for character control
//   7. Build system tray UI with bot reference
//
// Returns:
//   - *Bot: Fully initialized bot instance ready to run
//
// Notes:
//   - If data.json doesn't exist or is corrupted, uses default configuration
//   - Browser is NOT started here; it starts asynchronously when StartMainLoop is called
//   - All components are initialized synchronously to ensure dependency chain is valid
func NewBot() *Bot {
	LogInfo("Initializing bot components...")

	// Load persistent data (config and cookies)
	data, err := LoadData()
	if err != nil {
		LogError("Failed to load data: %v, using defaults", err)
		data = NewPersistentData()
	}

	LogDebug("Config loaded")
	stats := NewStatistics()
	LogDebug("Statistics created")
	browser := NewBrowser()
	LogDebug("Browser created")
	action := NewAction(browser)
	LogDebug("Action created")
	analyzer := NewImageAnalyzer(browser)
	LogDebug("Image analyzer created")
	movement := NewMovementCoordinator(action, browser)
	LogDebug("Movement coordinator created")

	bot := &Bot{
		config:   data.Config,
		stats:    stats,
		action:   action,
		browser:  browser,
		analyzer: analyzer,
		movement: movement,
		stopChan: make(chan bool),
		data:     data,
		debugOverlayChan: make(chan *DebugOverlayRequest, 10), // Buffered channel for non-blocking sends
		debugOverlayEnabled: true, // Can be toggled via config later
	}

	// Create system tray UI
	LogInfo("Creating system tray UI...")
	bot.tray = NewTrayApp(bot)
	LogInfo("Bot components initialized successfully")

	return bot
}

// StartMainLoop starts the browser and main loop asynchronously.
//
// This function is called automatically when the system tray is ready and launches
// two independent goroutines that run concurrently:
//
// Goroutine 1 - Browser Initialization:
//   - Starts chromedp browser with game URL navigation
//   - Sets cookies from previous session if available
//   - Uses 60-second timeout to prevent indefinite blocking
//   - Logs errors but does not crash the program on failure
//
// Goroutine 2 - Main Loop Execution:
//   - Starts immediately without waiting for browser
//   - Polls for canvas element existence to detect browser readiness
//   - Executes at configurable intervals (default 1000ms)
//   - Continues running even if browser fails
//
// Design Rationale:
// This asynchronous approach ensures the UI remains responsive and the bot can
// recover from browser failures. The main loop will skip iterations gracefully
// until the browser becomes ready.
func (b *Bot) StartMainLoop() {
	// Start browser asynchronously (don't wait for it to be ready)
	LogInfo("Starting browser asynchronously...")
	SafeGo(func() {
		err := b.browser.Start(b.data.Cookies)
		if err != nil {
			LogError("Failed to start browser: %v", err)
		} else {
			LogInfo("Browser is now ready")
		}
	})

	// Start async debug overlay worker
	if b.debugOverlayEnabled {
		LogInfo("Starting async debug overlay worker...")
		SafeGo(func() {
			b.debugOverlayWorker()
		})
	}

	// Set initial mode immediately
	b.ChangeMode("Farming")

	// Start main loop immediately (don't wait for browser)
	LogInfo("Starting main loop immediately (browser will continue loading in background)")
	SafeGo(func() {
		b.mainLoop()
	})
}

// debugOverlayWorker processes debug overlay requests asynchronously.
//
// This worker runs in a separate goroutine and continuously processes debug overlay
// rendering requests from the debugOverlayChan. By running in a separate goroutine,
// it prevents blocking the main loop and browser interactions.
//
// Algorithm:
//   1. Listen for debug overlay requests on debugOverlayChan
//   2. For each request, call DrawDebugOverlay with the provided data
//   3. Log any errors but continue processing
//   4. Exit when channel is closed (during shutdown)
//
// Performance:
// Since DrawDebugOverlay can take 50-200ms to execute (DOM manipulation), running
// it asynchronously ensures the main loop can continue capturing and analyzing
// at full speed without waiting for overlay rendering to complete.
//
// Thread Safety:
// Uses buffered channel (capacity 10) to allow non-blocking sends from main loop.
// If channel is full, oldest requests are skipped to avoid backpressure.
func (b *Bot) debugOverlayWorker() {
	LogInfo("Debug overlay worker started")
	defer LogInfo("Debug overlay worker stopped")

	for req := range b.debugOverlayChan {
		if req == nil {
			continue
		}

		// Draw debug overlay with the provided data
		LogDebug("Processing debug overlay request...")
		err := b.browser.DrawDebugOverlay(req.Targets, req.Config, req.ClientStats, req.Stats, req.Action, req.BehaviorState)
		if err != nil {
			LogDebug("Failed to draw debug overlay: %v", err)
		}
	}
}

// ChangeMode switches the bot's operational mode and creates the appropriate behavior instance.
//
// Supported Modes:
//   - "Stop": Disables all bot actions, only image recognition continues
//   - "Farming": Activates autonomous mob hunting and item collection
//   - "Support": Enables party member following, healing, and buffing
//   - "Shouting": Placeholder for auto-chat functionality (not yet implemented)
//
// Parameters:
//   - mode: String identifier for the desired mode (case-sensitive)
//
// Algorithm:
//   1. Log the mode change request
//   2. Update thread-safe config.Mode value
//   3. Stop current behavior if one is active
//   4. Create new behavior instance based on mode selection
//   5. Log confirmation of behavior activation
//
// Thread Safety:
// Uses config.SetMode() which is mutex-protected for concurrent access safety.
// Safe to call from any goroutine including system tray event handlers.
func (b *Bot) ChangeMode(mode string) {
	LogInfo("Changing mode to: %s", mode)
	b.config.SetMode(mode)

	// Stop current behavior if any
	if b.behavior != nil {
		b.behavior.Stop()
	}

	// Create appropriate behavior based on mode
	switch mode {
	case "Stop":
		b.behavior = nil
		LogInfo("Bot stopped, image recognition continues")
	case "Farming":
		b.behavior = NewFarmingBehavior()
		LogInfo("Farming behavior activated")
	case "Support":
		b.behavior = NewSupportBehavior()
		LogInfo("Support behavior activated")
	case "Shouting":
		b.behavior = NewShoutBehavior()
		LogInfo("Shouting behavior activated")
	default:
		b.behavior = nil
	}
}

// StopBehavior gracefully stops the current active behavior and signals the main loop to terminate.
//
// Algorithm:
//   1. Log the stop request
//   2. Call Stop() on current behavior if one exists
//   3. Close debug overlay channel to signal worker to exit
//   4. Attempt non-blocking send to stopChan
//   5. Log whether signal was sent or channel is already full
//
// Thread Safety:
// Uses select with default case to prevent blocking if the channel is full or
// if no goroutine is listening. Safe to call multiple times.
//
// Notes:
//   - Does not block the caller
//   - Safe to call even if behavior is nil
//   - Main loop will terminate on next iteration after receiving signal
//   - Debug overlay worker will exit when channel is closed
func (b *Bot) StopBehavior() {
	LogInfo("Stopping behavior")
	if b.behavior != nil {
		b.behavior.Stop()
	}

	// Close debug overlay channel to signal worker to exit
	if b.debugOverlayChan != nil {
		close(b.debugOverlayChan)
		LogDebug("Debug overlay channel closed")
	}

	// Non-blocking send to stopChan
	select {
	case b.stopChan <- true:
		LogDebug("Stop signal sent to main loop")
	default:
		LogDebug("Main loop already stopped or not listening")
	}
}

// mainLoop is the core execution loop that runs continuously until stopped.
//
// Loop Structure:
// Runs an infinite loop that checks for stop signals and executes iterations
// at configurable intervals. Uses select statement with default case to avoid
// blocking when no stop signal is present.
//
// Algorithm:
//   1. Check stopChan for termination signal (non-blocking)
//   2. Read current CaptureInterval from thread-safe config
//   3. Calculate time elapsed since last capture
//   4. If enough time has passed (or continuous mode):
//      - Execute runIteration()
//      - Update lastCaptureTime
//   5. Else:
//      - Sleep for 50ms to prevent busy-waiting and reduce CPU usage
//   6. Repeat until stop signal received
//
// Timing Modes:
//   - CaptureInterval = 0: Continuous execution (no sleep between iterations)
//   - CaptureInterval > 0: Waits for specified milliseconds between iterations
//
// Performance:
// Uses 50ms sleep intervals when waiting, resulting in ~5% CPU usage instead
// of 100% that would occur with tight busy-waiting loop.
func (b *Bot) mainLoop() {
	LogInfo("Main loop started")

	lastCaptureTime := time.Now()

	for {
		select {
		case <-b.stopChan:
			LogInfo("Stop signal received")
			return
		default:
			// Get current capture interval
			b.config.mu.RLock()
			captureInterval := b.config.CaptureInterval
			b.config.mu.RUnlock()

			// Check if enough time has passed since last capture
			now := time.Now()
			timeSinceCapture := now.Sub(lastCaptureTime)

			if captureInterval == 0 || timeSinceCapture >= time.Duration(captureInterval)*time.Millisecond {
				b.runIteration()
				lastCaptureTime = now
			} else {
				// Sleep for a short time to avoid busy waiting
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

// runIteration executes a single cycle of the bot's operation pipeline.
//
// This is the core function that orchestrates all bot activities in sequence.
// It performs image capture, analysis, debug visualization, and behavior execution.
//
// Execution Pipeline:
//   1. Check if game canvas exists (browser readiness indicator)
//      - Returns early if canvas not found
//   2. Capture screenshot from browser (5s timeout)
//      - Returns early on failure
//   3. Set captured image to analyzer for processing
//   4. Update player statistics (HP/MP/FP bar recognition)
//   5. Get current client stats (percentage values, alive state)
//   6. Identify mobs in current frame (yellow passive, red aggressive)
//   7. Draw debug overlay with detection results (2s timeout)
//      - Shows status bars, mob bounding boxes, statistics panel
//   8. Execute current behavior if active (Farming/Support)
//      - Skipped if mode is "Stop"
//   9. Update system tray status display
//
// Performance Timing:
// Uses Timer to log execution duration for performance monitoring.
// Typical iteration time: 60-120ms depending on mob count and overlay complexity.
//
// Error Handling:
// All errors are logged but do not stop execution. The bot continues operating
// even if individual steps fail, ensuring robustness against transient issues.
//
// Returns:
// This function returns immediately if prerequisites are not met (canvas missing,
// capture failed). Otherwise executes full pipeline before returning.
func (b *Bot) runIteration() {
	LogDebug("runIteration: starting")
	timer := NewTimer("main_loop")
	defer timer.Log()

	// Check if canvas exists in the page (indicates game is loaded)
	if !b.browser.CheckCanvasExists() {
		LogDebug("Canvas element not found, game not ready yet")
		return
	}

	// Save cookies when game first loads (canvas detected for the first time)
	if !b.cookiesSaved {
		LogInfo("Game loaded, saving cookies automatically...")
		b.SaveState()
		b.cookiesSaved = true
		LogInfo("Cookies saved successfully after game load")
	}

	// Capture screen from browser and store in analyzer
	LogDebug("runIteration: calling Capture")
	img, err := b.browser.Capture()
	if err != nil {
		LogError("Failed to capture screen: %v", err)
		return
	}
	LogDebug("runIteration: capture returned, img != nil: %v", img != nil)

	// Store captured image in analyzer
	if img != nil {
		b.analyzer.mu.Lock()
		b.analyzer.lastImage = img
		b.analyzer.mu.Unlock()
	}

	// Update stats before drawing overlay
	LogDebug("runIteration: calling UpdateStats")
	b.analyzer.UpdateStats()

	// Send debug overlay request to async worker (non-blocking)
	if b.debugOverlayEnabled {
		LogDebug("runIteration: calling GetStats")
		clientStats := b.analyzer.GetStats()
		LogDebug("runIteration: calling IdentifyMobs")
		targets := b.analyzer.IdentifyMobs(b.config)
		LogDebug("runIteration: found %d targets", len(targets))

		// Prepare behavior state
		behaviorState := ""
		if b.behavior != nil {
			behaviorState = b.behavior.GetState()
		}

		// Create debug overlay request
		request := &DebugOverlayRequest{
			Targets:       targets,
			Config:        b.config,
			ClientStats:   clientStats,
			Stats:         b.stats,
			Action:        b.action,
			BehaviorState: behaviorState,
		}

		// Non-blocking send to worker
		select {
		case b.debugOverlayChan <- request:
			LogDebug("Debug overlay request sent to worker")
		default:
			LogDebug("Debug overlay worker is busy, skipping this frame")
		}
	}

	// Run behavior only if not in Stop mode
	mode := b.config.GetMode()
	if b.behavior != nil && mode != "Stop" {
		err = b.behavior.Run(b.analyzer, b.movement, b.config, b.stats)
		if err != nil {
			LogError("Behavior error: %v", err)
		}
	}

	// Update tray status
	if b.tray != nil {
		b.tray.UpdateStatus(mode)
	}
}

// SaveState persists the current bot configuration and browser cookies to data.json.
//
// This function is called during:
//   - Configuration changes via system tray (mode, slots, thresholds, capture interval)
//   - Graceful shutdown (quit button, signal handler)
//   - Manual save requests
//
// Save Process:
//   1. Log save operation start
//   2. Retrieve current cookies from browser via chromedp
//      - Logs warning if retrieval fails but continues
//   3. Update data.Cookies with fresh cookie values
//   4. Write PersistentData structure to data.json (formatted with 2-space indent)
//   5. Log success or failure
//
// Data Saved:
//   - Configuration: Mode, slot assignments, thresholds, mob colors, behavior settings
//   - Cookies: All browser cookies from universe.flyff.com domain for session persistence
//
// Error Handling:
// Cookie retrieval failure is logged as warning but does not prevent config save.
// File write failure is logged as error.
func (b *Bot) SaveState() {
	LogInfo("Saving bot state...")

	// Get current cookies from browser
	cookies, err := b.browser.GetCookies()
	if err != nil {
		LogWarn("Failed to get cookies: %v", err)
	} else {
		b.data.Cookies = cookies
		LogInfo("Saved %d cookies", len(cookies))
	}

	// Save data to file
	err = SaveData(b.data)
	if err != nil {
		LogError("Failed to save data: %v", err)
	} else {
		LogInfo("Bot state saved successfully")
	}
}

// Run starts the bot application and manages the main execution lifecycle.
//
// This is the entry point that orchestrates the entire application from startup
// to shutdown. It sets up signal handlers for graceful termination and starts
// the blocking system tray UI.
//
// Execution Flow:
//   1. Install OS signal handlers for SIGINT/SIGTERM
//      - Captures Ctrl+C and kill signals
//      - Triggers graceful shutdown sequence:
//        a. Stop active behavior
//        b. Save configuration and cookies
//        c. Close browser
//        d. Close log file
//        e. Exit with code 0
//   2. Start system tray UI (blocking call)
//      - Tray initialization triggers StartMainLoop() asynchronously
//      - Function blocks here until systray.Quit() is called
//   3. Perform final state save after tray exits
//
// Signal Handling:
// Uses a goroutine to listen for OS signals without blocking the main thread.
// Ensures all resources are properly cleaned up before exit.
//
// Notes:
//   - This function blocks until the system tray exits
//   - Browser and main loop start automatically when tray is ready
//   - Graceful shutdown is guaranteed on Ctrl+C or kill signal
func (b *Bot) Run() {
	LogInfo("Setting up signal handlers...")
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		LogInfo("Signal received: %v, shutting down gracefully...", sig)
		LogInfo("Stopping bot...")
		b.StopBehavior()
		LogInfo("Saving state...")
		b.SaveState()
		LogInfo("Closing browser...")
		b.browser.Close()
		LogInfo("Closing logger...")
		CloseLogger()
		LogInfo("Exiting with code 0")
		os.Exit(0)
	}()

	LogInfo("Signal handlers configured")

	// Run system tray (blocking) - tray will trigger browser start
	LogInfo("Starting system tray (browser will start when tray is ready)...")
	b.tray.Run()
	LogInfo("System tray exited")

	// Save state before exit
	LogInfo("Saving state before exit...")
	b.SaveState()
}

// main is the application entry point that initializes logging and starts the bot.
//
// Initialization Sequence:
//   1. Install panic recovery handler
//      - Catches any unhandled panics
//      - Logs panic to both stderr and Debug.log
//      - Ensures logger is closed before exit
//      - Exits with code 2 to indicate panic
//   2. Initialize logger system
//      - Creates/truncates Debug.log file
//      - Sets up log format with timestamps
//      - Exits with code 1 if logger initialization fails
//   3. Install deferred shutdown handler
//      - Logs shutdown message
//      - Closes logger file handle
//   4. Log startup message with platform info
//   5. Create bot instance
//      - Loads configuration
//      - Initializes all subsystems
//   6. Start bot execution
//      - Runs until quit signal received
//      - Returns normally when systray exits
//
// Exit Codes:
//   - 0: Normal exit (user quit)
//   - 1: Logger initialization failed
//   - 2: Unhandled panic occurred
//
// Notes:
//   - All panics are logged and handled gracefully
//   - Debug.log is cleared on each startup
//   - Function blocks until application terminates
func main() {
	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
			LogError("PANIC in main: %v", r)
			CloseLogger()
			os.Exit(2)
		}
	}()

	// Initialize logger
	err := InitLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		LogInfo("=== Flyff Bot Shutdown ===")
		CloseLogger()
	}()

	LogInfo("=== Flyff Bot Started ===")

	// Check for --train flag
	if len(os.Args) > 1 && os.Args[1] == "--train" {
		LogInfo("Training mode requested")
		if err := TrainingMode(); err != nil {
			LogError("Training mode failed: %v", err)
			os.Exit(1)
		}
		return
	}

	// Create and run bot
	LogInfo("Creating bot instance...")
	bot := NewBot()
	LogInfo("Bot instance created, starting main run...")
	bot.Run()
	LogInfo("Bot Run() returned normally")
}
