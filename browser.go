// Package main - browser.go
//
// This file implements the Browser controller that manages chromedp for game interaction.
// It provides screen capture, cookie management, and action logging.
//
// Key Responsibilities:
//   - Chromedp browser lifecycle management (start, navigate, close)
//   - Screenshot capture with timeout protection (5s)
//   - Cookie persistence (save/load for session continuation)
//   - Action logging for behavior tracking (used by debug overlay in debug.go)
//
// Browser Architecture:
// The Browser uses nested contexts for proper resource management:
//   - allocCtx: Allocator context for browser process management
//   - ctx: Browser context for page operations
// Both contexts have cancel functions for graceful cleanup.
//
// Timeout Strategy:
//   - Navigation: 60 seconds (slow network tolerance)
//   - Screenshot: 5 seconds (prevent hanging)
//   - Canvas check: 2 seconds (quick validation)
//
// Note: Debug overlay rendering has been moved to debug.go for better code organization.
package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// formatInt converts an integer to string for JavaScript injection.
//
// This helper function is used when building JavaScript code strings to ensure
// proper formatting of integer values without quotes.
//
// Parameters:
//   - i: Integer value to format
//
// Returns:
//   - string: Integer formatted as string (e.g., 123 -> "123")
func formatInt(i int) string {
	return fmt.Sprintf("%d", i)
}

// formatIntSlice converts an integer slice to comma-separated string for JavaScript.
//
// Used to format skill slot arrays for display in the debug overlay panel.
//
// Parameters:
//   - slice: Integer array to format
//
// Returns:
//   - string: Comma-separated values (e.g., [0, 1, 2] -> "0, 1, 2")
//   - string: Empty string if slice is empty
func formatIntSlice(slice []int) string {
	if len(slice) == 0 {
		return ""
	}
	result := fmt.Sprintf("%d", slice[0])
	for i := 1; i < len(slice); i++ {
		result += fmt.Sprintf(", %d", slice[i])
	}
	return result
}

// ActionLog represents a recorded user action for debug overlay display.
//
// ActionLog entries are stored in a ring buffer (last 10 actions) and displayed
// in the debug overlay to help visualize bot behavior in real-time.
//
// Fields:
//   - Message: Human-readable action description (e.g., "Click at (400, 300)")
//   - Timestamp: When the action occurred (used for time-based display)
type ActionLog struct {
	Message   string
	Timestamp time.Time
}

// Browser manages the chromedp browser instance for game interaction.
//
// Lifecycle:
//   1. NewBrowser(): Create instance with empty action log
//   2. Start(): Initialize chromedp contexts and navigate to game URL
//   3. CheckCanvasExists(): Verify game is loaded
//   4. Capture(): Take screenshots repeatedly
//   5. DrawDebugOverlay(): Render detection visualization
//   6. Close(): Clean up contexts and browser process
//
// Concurrency:
// Browser operations are thread-safe for read access to action logs (RWMutex).
// Context operations are protected by chromedp's internal synchronization.
//
// Error Handling:
// All chromedp operations use context timeouts to prevent indefinite blocking.
// Errors are logged but do not crash the application.
type Browser struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCtx    context.Context
	allocCancel context.CancelFunc
	actionLogs  []ActionLog
	logMutex    sync.RWMutex
}

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	return &Browser{
		actionLogs: make([]ActionLog, 0, 10),
	}
}

// LogAction logs an action for debug display (keeps last 10)
func (b *Browser) LogAction(message string) {
	b.logMutex.Lock()
	defer b.logMutex.Unlock()

	b.actionLogs = append(b.actionLogs, ActionLog{
		Message:   message,
		Timestamp: time.Now(),
	})

	// Keep only last 10 actions
	if len(b.actionLogs) > 10 {
		b.actionLogs = b.actionLogs[len(b.actionLogs)-10:]
	}
}

// GetActionLogs returns recent action logs
func (b *Browser) GetActionLogs() []ActionLog {
	b.logMutex.RLock()
	defer b.logMutex.RUnlock()

	logs := make([]ActionLog, len(b.actionLogs))
	copy(logs, b.actionLogs)
	return logs
}

// Start initializes chromedp browser and navigates to the game URL.
//
// This function performs the complete browser startup sequence including context
// creation, cookie restoration, and navigation with timeout protection.
//
// Parameters:
//   - cookies: Previously saved cookies to restore session (can be empty for new session)
//
// Returns:
//   - error: Navigation error (timeout or network failure), nil on success
//
// Algorithm:
//   1. Create exec allocator context with browser options:
//      - headless=false (show browser window)
//      - disable-gpu=false (enable GPU acceleration)
//      - disable automation detection flags
//      - Set window size to 800x600
//   2. Create browser context with custom logger
//   3. If cookies provided, set them before navigation
//   4. Navigate to https://universe.flyff.com/play with 60s timeout
//   5. Log success or error
//
// Timeout Protection:
// Uses 60-second timeout for navigation to handle slow networks or game server issues.
// Timeout error is logged but does not crash the program.
//
// Notes:
//   - Browser window is visible for debugging and user interaction
//   - Automation flags are disabled to avoid detection
//   - Failed navigation can be retried by calling Start() again
func (b *Browser) Start(cookies []CookieData) error {
	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // Show browser window
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(800, 600),
	)

	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	LogInfo("Browser allocator context created")

	// Create context
	b.ctx, b.cancel = chromedp.NewContext(b.allocCtx, chromedp.WithLogf(func(format string, args ...interface{}) {
		LogDebug(format, args...)
	}))
	LogInfo("Browser context created")

	// Navigate to game with timeout (if we have cookies, set them first)
	if len(cookies) > 0 {
		LogInfo("Setting %d cookies before navigation", len(cookies))
		err := b.SetCookies(cookies)
		if err != nil {
			LogWarn("Failed to set cookies before navigation: %v", err)
		}
	}

	LogInfo("Navigating to https://universe.flyff.com/play")

	// Navigate with timeout
	navCtx, navCancel := context.WithTimeout(b.ctx, 60*time.Second)
	defer navCancel()

	err := chromedp.Run(navCtx,
		chromedp.Navigate("https://universe.flyff.com/play"),
	)

	if err != nil {
		LogError("Navigation error: %v", err)
		return err
	}

	LogInfo("Navigation completed successfully")
	return nil
}

// CheckCanvasExists checks if the game canvas element exists in the page
func (b *Browser) CheckCanvasExists() bool {
	// Check if context is still valid
	if b.ctx == nil || b.ctx.Err() != nil {
		LogDebug("Browser context is invalid")
		return false
	}

	var canvasExists bool
	checkCtx, cancel := context.WithTimeout(b.ctx, 2*time.Second)
	defer cancel()

	err := chromedp.Run(checkCtx,
		chromedp.Evaluate(`document.getElementById('canvas') !== null`, &canvasExists),
	)

	if err != nil {
		LogDebug("Failed to check canvas existence: %v", err)
		return false
	}

	return canvasExists
}

// Capture takes a screenshot of the current browser page.
//
// This is the primary function for obtaining game frames for image recognition.
// Uses chromedp's CaptureScreenshot action with timeout protection.
//
// Returns:
//   - *image.RGBA: Screenshot as RGBA image, nil if capture fails
//   - error: Capture error (timeout, invalid context), nil on success
//
// Algorithm:
//   1. Validate browser context is still active
//   2. Create 5-second timeout context
//   3. Capture screenshot via chromedp (returns PNG bytes)
//   4. Decode PNG bytes to image.Image
//   5. Convert to *image.RGBA format (required by analyzer)
//   6. Return RGBA image or error
//
// Performance:
// Typical capture time: 10-50ms depending on screen resolution and content.
// Timeout set to 5 seconds to handle edge cases without blocking indefinitely.
//
// Error Handling:
// Returns nil image and logs error if:
//   - Context is invalid or cancelled
//   - Screenshot operation times out
//   - Image decoding fails
//
// Notes:
//   - Captures entire browser viewport (800x600 default)
//   - Does not capture areas outside the browser window
//   - Thread-safe (uses chromedp's internal synchronization)
func (b *Browser) Capture() (*image.RGBA, error) {
	// Check if context is still valid
	if b.ctx == nil || b.ctx.Err() != nil {
		LogDebug("Browser context is invalid")
		return nil, nil
	}

	var buf []byte
	// Use a timeout for screenshot to avoid hanging
	captureCtx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
	defer cancel()

	err := chromedp.Run(captureCtx,
		chromedp.CaptureScreenshot(&buf),
	)

	if err != nil {
		LogDebug("Screenshot failed: %v", err)
		return nil, err
	}

	// Decode image
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}

	return rgba, nil
}

// GetScreenBounds returns the browser viewport bounds
func (b *Browser) GetScreenBounds() image.Rectangle {
	return image.Rectangle{Max: image.Point{X: 800, Y: 600}}
}

// GetRecentLogs gets recent action logs (last 5)
func (b *Browser) GetRecentLogs() []ActionLog {
	b.logMutex.RLock()
	defer b.logMutex.RUnlock()

	count := 5
	if len(b.actionLogs) < count {
		count = len(b.actionLogs)
	}

	if count == 0 {
		return []ActionLog{}
	}

	// Return last N logs
	result := make([]ActionLog, count)
	copy(result, b.actionLogs[len(b.actionLogs)-count:])
	return result
}


// GetCookies retrieves all cookies from the browser
func (b *Browser) GetCookies() ([]CookieData, error) {
	if b.ctx == nil || b.ctx.Err() != nil {
		return nil, fmt.Errorf("browser context is invalid")
	}

	var cookies []*network.Cookie
	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		LogError("Failed to get cookies: %v", err)
		return nil, err
	}

	// Convert network cookies to CookieData
	cookieData := make([]CookieData, len(cookies))
	for i, c := range cookies {
		cookieData[i] = CookieData{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		}
	}

	LogInfo("Retrieved %d cookies from browser", len(cookieData))
	return cookieData, nil
}

// SetCookies sets cookies in the browser
func (b *Browser) SetCookies(cookies []CookieData) error {
	if len(cookies) == 0 {
		return nil
	}

	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, c := range cookies {
				// Use network.SetCookie to set each cookie
				params := network.SetCookie(c.Name, c.Value).
					WithDomain(c.Domain).
					WithPath(c.Path).
					WithHTTPOnly(c.HTTPOnly).
					WithSecure(c.Secure)

				// Set expires if valid
				if c.Expires > 0 {
					expires := cdp.TimeSinceEpoch(time.Unix(int64(c.Expires), 0))
					params = params.WithExpires(&expires)
				}

				// Set SameSite if valid
				if c.SameSite != "" {
					params = params.WithSameSite(network.CookieSameSite(c.SameSite))
				}

				if err := params.Do(ctx); err != nil {
					LogWarn("Failed to set cookie %s: %v", c.Name, err)
				}
			}
			return nil
		}),
	)

	if err != nil {
		LogError("Failed to set cookies: %v", err)
		return err
	}

	LogInfo("Set %d cookies in browser", len(cookies))
	return nil
}

// Close closes the browser
func (b *Browser) Close() {
	LogInfo("Closing browser...")
	if b.cancel != nil {
		LogDebug("Cancelling browser context")
		b.cancel()
	}
	if b.allocCancel != nil {
		LogDebug("Cancelling allocator context")
		b.allocCancel()
	}
	LogInfo("Browser closed successfully")
}
