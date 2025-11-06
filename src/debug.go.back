// Package main - debug.go
//
// This file implements debug visualization overlay and logging for the game.
// It provides real-time status display, action logging, detection visualization,
// and centralized logging functionality.
//
// Major Components:
//
// 1. Logging System:
//    - Thread-safe file logging to Debug.log
//    - Four log levels: DEBUG, INFO, WARN, ERROR
//    - Microsecond timestamps for performance analysis
//    - File is truncated (cleared) on each startup
//    - Global logger instance accessible via convenience functions
//
// 2. Debug Visualization:
//   - Drawing debug overlay on game canvas via JavaScript injection
//   - Status panel rendering (HP/MP/FP, thresholds, slots)
//   - Action log display (recent 5 actions)
//   - Detection visualization (status bars, mob boxes, target HP)
//   - Mouse position tracking
//   - OCR/text recognition region visualization
//
// Overlay Components:
//   1. Status bar detection region (yellow box, top-left 500x300)
//   2. Detected HP/MP/FP bars (green boxes with percentages)
//   3. Target HP bar (red box with percentage)
//   4. Mob bounding boxes (green boxes labeled MOB1, MOB2, etc.)
//   5. Text recognition regions (thin cyan lines)
//   6. Status panel (left side, semi-transparent background 80% opacity)
//      - Mode, kills, KPM, uptime
//      - Mouse position (canvas-relative coordinates)
//      - HP/MP/FP values with thresholds
//      - Configured skill slots
//   7. Recent action log (below status panel)
//
// Logging Best Practices:
//   - DEBUG: Detailed operation info (pixel counts, coordinates, timing)
//   - INFO: Important events (startup, mode changes, kills)
//   - WARN: Non-critical issues (cookie load failure, timeout warnings)
//   - ERROR: Serious problems (file access errors, recognition failures)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Logger provides thread-safe logging functionality to Debug.log file.
//
// The logger writes all messages to a file with timestamps and log levels.
// Thread safety is ensured via mutex, allowing multiple goroutines to log
// concurrently without race conditions.
//
// File Behavior:
// Debug.log is truncated (O_TRUNC) on each startup to prevent log accumulation.
// This ensures the log file always contains only the current session's messages.
type Logger struct {
	file   *os.File
	logger *log.Logger
	mu     sync.Mutex
}

var globalLogger *Logger

// InitLogger initializes the global logger to write to Debug.log in current directory
// The log file is truncated (cleared) on each startup
func InitLogger() error {
	// Use O_TRUNC to clear the file on startup
	file, err := os.OpenFile("Debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	globalLogger = &Logger{
		file:   file,
		logger: log.New(file, "", log.LstdFlags|log.Lmicroseconds),
	}

	globalLogger.Info("Logger initialized (log file cleared)")
	return nil
}

// CloseLogger closes the log file
func CloseLogger() {
	if globalLogger != nil && globalLogger.file != nil {
		globalLogger.Info("Logger closing")
		globalLogger.file.Close()
	}
}

// Debug logs debug level messages
func (l *Logger) Debug(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("[DEBUG] "+format, v...)
}

// Info logs info level messages
func (l *Logger) Info(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("[INFO] "+format, v...)
}

// Warn logs warning level messages
func (l *Logger) Warn(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("[WARN] "+format, v...)
}

// Error logs error level messages
func (l *Logger) Error(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.Printf("[ERROR] "+format, v...)
}

// LogDebug is a convenience function for debug logging
func LogDebug(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(format, v...)
	}
}

// LogInfo is a convenience function for info logging
func LogInfo(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(format, v...)
	}
}

// LogWarn is a convenience function for warning logging
func LogWarn(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(format, v...)
	}
}

// LogError is a convenience function for error logging
func LogError(format string, v ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, v...)
	}
}

// DrawDebugOverlay renders debug visualization overlay on the game canvas.
//
// This function injects JavaScript code into the browser to draw detection results,
// status information, and bot statistics directly on top of the game canvas.
//
// Parameters:
//   - targets: Detected mob targets with bounding boxes
//   - config: Current bot configuration (slots, thresholds, mode)
//   - stats: Client statistics (HP/MP/FP percentages and bar positions)
//   - botStats: Bot statistics (kills, KPM, uptime)
//   - action: Action instance (currently unused but available for extension)
//
// Returns:
//   - error: JavaScript injection error, nil on success
//
// Overlay Components:
//   1. Status Bar Detection Region (Yellow Box, 0,0-500,300)
//   2. Detected Status Bars (Green Boxes with Percentages):
//      - HP bar with percentage
//      - MP bar with percentage
//      - FP bar with percentage
//   3. Target HP Bar (Red Box if target exists)
//   4. Mob Bounding Boxes (Green boxes labeled MOB1, MOB2, ...)
//   5. Status Panel (Left side, X=5 Y=300, semi-transparent 80% opacity):
//      - Mode and statistics (22px font)
//      - Mouse position (canvas-relative coordinates)
//      - HP/MP/FP values with thresholds
//      - Configured skill slots for all categories
//   6. Action Log (Below status panel, 22px font):
//      - Last 5 actions with timestamps
//
// Algorithm:
//   1. Validate browser context
//   2. Build JavaScript code string with all overlay elements
//   3. Format configuration values for JavaScript injection
//   4. Create 2-second timeout context
//   5. Execute JavaScript via chromedp.Evaluate()
//   6. Log success or failure
//
// Mouse Tracking:
// Installs a persistent mousemove event listener on the canvas that tracks
// cursor position in canvas-relative coordinates (accounting for canvas scaling).
//
// Performance:
// Typical execution time: 50-100ms. Uses 2-second timeout to prevent blocking.
//
// Notes:
//   - Creates new canvas overlay element on each call (removes old one first)
//   - Overlay is non-interactive (pointer-events: none)
//   - Uses absolute positioning relative to game canvas
//   - All drawing done client-side via JavaScript (no server round-trip)
func (b *Browser) DrawDebugOverlay(targets []Target, config *Config, stats *ClientStats, botStats *Statistics, action *Action, behaviorState string) error {
	// Check if context is valid
	if b.ctx == nil || b.ctx.Err() != nil {
		LogDebug("DrawDebugOverlay: browser context invalid")
		return nil
	}

	LogDebug("DrawDebugOverlay: starting to draw overlay")

	// Build status strings
	config.mu.RLock()
	mode := config.Mode
	attackSlots := formatIntSlice(config.AttackSlots)
	healSlots := formatIntSlice(config.HealSlots)
	buffSlots := formatIntSlice(config.BuffSlots)
	mpSlots := formatIntSlice(config.MPRestoreSlots)
	fpSlots := formatIntSlice(config.FPRestoreSlots)
	pickupSlots := formatIntSlice(config.PickupSlots)
	hpThreshold := config.HealThreshold
	mpThreshold := config.MPThreshold
	fpThreshold := config.FPThreshold
	config.mu.RUnlock()

	// Get bot statistics
	kills, kpm, _, uptime := botStats.GetStats()

	// Get client stats and detected bar positions
	hpPercent := 0
	mpPercent := 0
	fpPercent := 0
	var hpBar, mpBar, fpBar DetectedBar
	var targetHPBar DetectedBar
	hasTarget := false

	if stats != nil {
		hpPercent = stats.HP.Value
		mpPercent = stats.MP.Value
		fpPercent = stats.FP.Value
		hpBar = stats.HPBar
		mpBar = stats.MPBar
		fpBar = stats.FPBar
		targetHPBar = stats.TargetHPBar
		hasTarget = stats.TargetOnScreen
	}

	// Get recent action logs
	recentLogs := b.GetRecentLogs()

	// Build JavaScript to draw overlay - use absolute coordinates relative to game canvas
	js := `
		(function() {
			// Get the game canvas
			const gameCanvas = document.getElementById('canvas');
			if (!gameCanvas) {
				console.log('Game canvas not found');
				return;
			}

			// Get canvas absolute position
			const rect = gameCanvas.getBoundingClientRect();
			const offsetX = rect.left;
			const offsetY = rect.top;

			// Get or initialize mouse position tracking
			if (!window.flyffMousePos) {
				window.flyffMousePos = { x: 0, y: 0 };

				// Add mouse move listener to game canvas
				gameCanvas.addEventListener('mousemove', function(e) {
					const canvasRect = gameCanvas.getBoundingClientRect();
					// Calculate mouse position relative to canvas
					const scaleX = gameCanvas.width / canvasRect.width;
					const scaleY = gameCanvas.height / canvasRect.height;
					window.flyffMousePos.x = Math.floor((e.clientX - canvasRect.left) * scaleX);
					window.flyffMousePos.y = Math.floor((e.clientY - canvasRect.top) * scaleY);
				});
			}

			// Remove existing overlay if any
			let overlay = document.getElementById('flyff-debug-overlay');
			if (overlay) {
				overlay.remove();
			}

			// Create new overlay canvas positioned over the game canvas
			overlay = document.createElement('canvas');
			overlay.id = 'flyff-debug-overlay';
			overlay.style.position = 'absolute';
			overlay.style.left = offsetX + 'px';
			overlay.style.top = offsetY + 'px';
			overlay.style.pointerEvents = 'none';
			overlay.style.zIndex = '9999';
			overlay.width = gameCanvas.width;
			overlay.height = gameCanvas.height;
			overlay.style.width = rect.width + 'px';
			overlay.style.height = rect.height + 'px';
			document.body.appendChild(overlay);

			const ctx = overlay.getContext('2d');
			ctx.strokeStyle = 'lime';
			ctx.lineWidth = 2;
			ctx.fillStyle = 'lime';
			ctx.font = 'bold 16px monospace';
			ctx.textAlign = 'center';
			ctx.textBaseline = 'middle';

			// Get current mouse position in canvas coordinates
			const mouseX = window.flyffMousePos.x;
			const mouseY = window.flyffMousePos.y;
	`

	// Draw HP bar with percentage if detected
	if hpBar.Detected {
		js += `
			// Draw HP region
			ctx.strokeRect(` + formatInt(hpBar.Bounds.X) + `, ` + formatInt(hpBar.Bounds.Y) + `, ` + formatInt(hpBar.Bounds.W) + `, ` + formatInt(hpBar.Bounds.H) + `);
			// Draw HP percentage in center of box
			const hpCenterX = ` + formatInt(hpBar.Bounds.X+hpBar.Bounds.W/2) + `;
			const hpCenterY = ` + formatInt(hpBar.Bounds.Y+hpBar.Bounds.H/2) + `;
			ctx.fillText('HP: ` + formatInt(hpPercent) + `%', hpCenterX, hpCenterY);
		`
	}

	// Draw MP bar with percentage if detected
	if mpBar.Detected {
		js += `
			// Draw MP region
			ctx.strokeRect(` + formatInt(mpBar.Bounds.X) + `, ` + formatInt(mpBar.Bounds.Y) + `, ` + formatInt(mpBar.Bounds.W) + `, ` + formatInt(mpBar.Bounds.H) + `);
			// Draw MP percentage in center of box
			const mpCenterX = ` + formatInt(mpBar.Bounds.X+mpBar.Bounds.W/2) + `;
			const mpCenterY = ` + formatInt(mpBar.Bounds.Y+mpBar.Bounds.H/2) + `;
			ctx.fillText('MP: ` + formatInt(mpPercent) + `%', mpCenterX, mpCenterY);
		`
	}

	// Draw FP bar with percentage if detected
	if fpBar.Detected {
		js += `
			// Draw FP region
			ctx.strokeRect(` + formatInt(fpBar.Bounds.X) + `, ` + formatInt(fpBar.Bounds.Y) + `, ` + formatInt(fpBar.Bounds.W) + `, ` + formatInt(fpBar.Bounds.H) + `);
			// Draw FP percentage in center of box
			const fpCenterX = ` + formatInt(fpBar.Bounds.X+fpBar.Bounds.W/2) + `;
			const fpCenterY = ` + formatInt(fpBar.Bounds.Y+fpBar.Bounds.H/2) + `;
			ctx.fillText('FP: ` + formatInt(fpPercent) + `%', fpCenterX, fpCenterY);
		`
	}

	// Draw target HP bar if detected
	if hasTarget && targetHPBar.Detected {
		js += `
			// Draw Target HP region
			ctx.strokeStyle = 'red';
			ctx.strokeRect(` + formatInt(targetHPBar.Bounds.X) + `, ` + formatInt(targetHPBar.Bounds.Y) + `, ` + formatInt(targetHPBar.Bounds.W) + `, ` + formatInt(targetHPBar.Bounds.H) + `);
			// Draw Target HP percentage in center of box
			ctx.fillStyle = 'red';
			const targetCenterX = ` + formatInt(targetHPBar.Bounds.X+targetHPBar.Bounds.W/2) + `;
			const targetCenterY = ` + formatInt(targetHPBar.Bounds.Y+targetHPBar.Bounds.H/2) + `;
			ctx.fillText('Target: ` + formatInt(targetHPBar.Percentage) + `%', targetCenterX, targetCenterY);
			ctx.fillStyle = 'lime';
			ctx.strokeStyle = 'lime';
		`
	}

	// Draw status bar detection region (top-left corner: x < 500, y < 300)
	js += `
			// Draw status bar detection region
			ctx.strokeStyle = 'yellow';
			ctx.lineWidth = 3;
			ctx.strokeRect(0, 0, 500, 300);
			ctx.fillStyle = 'yellow';
			ctx.font = '16px monospace';
			ctx.textAlign = 'left';
			ctx.fillText('Status Bar Region', 10, 20);

			// Reset stroke style for mobs
			ctx.strokeStyle = 'lime';
			ctx.lineWidth = 2;
	`

	// Add mob target rectangles
	js += `
			// Reset text alignment for mob labels
			ctx.textAlign = 'left';
			ctx.font = '14px monospace';
	`
	for i, target := range targets {
		js += `
			ctx.strokeRect(` + formatInt(target.Bounds.X) + `, ` + formatInt(target.Bounds.Y) + `, ` + formatInt(target.Bounds.W) + `, ` + formatInt(target.Bounds.H) + `);
			ctx.fillText('MOB` + formatInt(i+1) + `', ` + formatInt(target.Bounds.X) + `, ` + formatInt(target.Bounds.Y-2) + `);
		`
	}

	// Draw text recognition regions with thin cyan lines
	js += `
			// Draw text recognition/OCR regions with thin cyan lines
			ctx.strokeStyle = 'cyan';
			ctx.lineWidth = 1;
			ctx.setLineDash([5, 3]); // Dashed line pattern
	`

	// Draw mob name detection region (excluding top/bottom UI)
	if stats != nil && stats.HPBar.Detected {
		// Calculate the mob detection area based on screen dimensions
		// This matches the region used in IdentifyMobs (analyzer.go)
		screenHeight := 1080 // Default, will be overridden if browser available
		screenWidth := 1920  // Default
		if b != nil {
			bounds := b.GetScreenBounds()
			screenWidth = bounds.Dx()
			screenHeight = bounds.Dy()
		}

		topIgnore := 100    // Top UI area to ignore
		bottomIgnore := 150 // Bottom UI area to ignore

		mobDetectionY := topIgnore
		mobDetectionH := screenHeight - topIgnore - bottomIgnore

		js += `
			// Mob name detection region
			ctx.strokeRect(0, ` + formatInt(mobDetectionY) + `, ` + formatInt(screenWidth) + `, ` + formatInt(mobDetectionH) + `);
			ctx.fillStyle = 'cyan';
			ctx.font = '12px monospace';
			ctx.textAlign = 'right';
			ctx.fillText('Mob Detection Region', ` + formatInt(screenWidth-10) + `, ` + formatInt(mobDetectionY+15) + `);
		`
	}

	// Draw target marker detection region (upper-middle area)
	js += `
			// Target marker detection region (upper-middle area)
			const markerX = Math.floor(overlay.width / 4);
			const markerY = Math.floor(overlay.height / 6);
			const markerW = Math.floor(overlay.width / 2);
			const markerH = Math.floor(overlay.height / 3);
			ctx.strokeRect(markerX, markerY, markerW, markerH);
			ctx.fillStyle = 'cyan';
			ctx.font = '12px monospace';
			ctx.textAlign = 'center';
			ctx.fillText('Target Marker Region', markerX + markerW / 2, markerY + 12);

			// Reset line dash
			ctx.setLineDash([]);
			ctx.strokeStyle = 'lime';
			ctx.lineWidth = 2;
			ctx.fillStyle = 'lime';
	`

	// Draw combined status and action info panel on left side at 300px from top
	// Panel position: left edge of screen, 300px from top
	panelX := 5
	panelY := 300
	panelWidth := 500

	// Calculate panel height based on content (using 22px font + spacing)
	baseHeight := 320 // Base height for status info (increased for 22px font)
	if behaviorState != "" {
		baseHeight += 24 // Add space for state line
	}
	logHeight := 0
	if len(recentLogs) > 0 {
		logHeight = 50 + len(recentLogs)*24 // Title + logs (increased for 22px font)
	}
	panelHeight := baseHeight + logHeight

	js += `
			// Draw semi-transparent background for combined status and action panel (80% opacity)
			ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
			ctx.fillRect(` + formatInt(panelX) + `, ` + formatInt(panelY) + `, ` + formatInt(panelWidth) + `, ` + formatInt(panelHeight) + `);

			// Draw status section
			ctx.fillStyle = 'lime';
			ctx.font = 'bold 22px monospace';
			ctx.textAlign = 'left';
			let y = ` + formatInt(panelY+28) + `;
			const lineHeight = 24;

			ctx.fillText('=== Status ===', ` + formatInt(panelX+10) + `, y); y += lineHeight + 5;
			ctx.font = '22px monospace';
	`

	js += `ctx.fillText('Mode: ` + mode + `', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	// Add behavior state if provided
	if behaviorState != "" {
		js += `ctx.fillText('State: ` + behaviorState + `', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
		js += "\n"
	}
	js += `ctx.fillText('Kills: ` + formatInt(kills) + ` (` + fmt.Sprintf("%.1f", kpm) + `/min)', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('Uptime: ` + uptime + `', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('Mouse: (' + mouseX + ', ' + mouseY + ')', ` + formatInt(panelX+10) + `, y); y += lineHeight + 3;`
	js += "\n"
	js += `ctx.fillText('HP: ` + formatInt(hpPercent) + `% (Thr: ` + formatInt(hpThreshold) + `%)', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('MP: ` + formatInt(mpPercent) + `% (Thr: ` + formatInt(mpThreshold) + `%)', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('FP: ` + formatInt(fpPercent) + `% (Thr: ` + formatInt(fpThreshold) + `%)', ` + formatInt(panelX+10) + `, y); y += lineHeight + 3;`
	js += "\n"
	js += `ctx.fillText('Attack: [` + attackSlots + `]', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('Heal: [` + healSlots + `]', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('Buff: [` + buffSlots + `]', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('MP: [` + mpSlots + `]', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('FP: [` + fpSlots + `]', ` + formatInt(panelX+10) + `, y); y += lineHeight;`
	js += "\n"
	js += `ctx.fillText('Pickup: [` + pickupSlots + `]', ` + formatInt(panelX+10) + `, y); y += lineHeight;`

	// Draw action logs in the same panel below status info
	if len(recentLogs) > 0 {
		js += `
			// Draw action section separator and title
			y += 10;
			ctx.fillStyle = 'yellow';
			ctx.font = 'bold 22px monospace';
			ctx.fillText('=== Recent Actions ===', ` + formatInt(panelX+10) + `, y); y += lineHeight + 5;

			// Draw action logs
			ctx.fillStyle = 'white';
			ctx.font = '22px monospace';
		`

		for _, log := range recentLogs {
			// Format time as HH:MM:SS
			timeStr := log.Timestamp.Format("15:04:05")
			escapedMsg := log.Message // TODO: escape if needed
			js += `
			ctx.fillText('[` + timeStr + `] ` + escapedMsg + `', ` + formatInt(panelX+10) + `, y);
			y += lineHeight;
			`
		}
	}

	js += `
		})();
	`

	LogDebug("DrawDebugOverlay: executing javascript (length: %d bytes)", len(js))

	// Use timeout to prevent blocking
	ctx, cancel := context.WithTimeout(b.ctx, 2*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Evaluate(js, nil),
	)

	if err != nil {
		LogError("Failed to draw debug overlay: %v", err)
		return err
	}

	LogDebug("DrawDebugOverlay: successfully drawn")
	return nil
}
