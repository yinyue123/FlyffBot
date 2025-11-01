// Package main - action.go
//
// This file implements game actions through JavaScript injection into the browser.
// All keyboard and mouse operations are performed via eval.js functions in the browser.
//
// Key Responsibilities:
//   - Keyboard event simulation via JavaScript injection
//   - Mouse event simulation via JavaScript injection
//   - Slot/skill activation
//   - Chat message sending
//
// Architecture:
// Unlike the old platform.go which used robotgo for native system calls,
// this implementation injects JavaScript into the Chromedp browser context
// to dispatch events directly to the game canvas element.
//
// Advantages of JavaScript injection:
//   1. More reliable - events go directly to canvas
//   2. Cross-platform - no native dependencies
//   3. No focus issues - works even when browser is in background
//   4. Consistent timing - JavaScript event loop
//
// JavaScript Dependencies:
// Requires eval.js to be loaded in the browser context with these functions:
//   - keyboardEvent(mode, key, duration)
//   - mouseEvent(type, x, y, options)
//   - sendSlot(slotBarIndex, slotIndex)
//   - setInputChat(text)
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

// KeyMode represents keyboard action type
type KeyMode int

const (
	KeyPress   KeyMode = iota // Press and release
	KeyHold                   // Hold down
	KeyRelease                // Release held key
)

// MouseMode represents mouse action type
type MouseMode int

const (
	MouseMove      MouseMode = iota // Move cursor only
	MouseClick                       // Move and click
	MouseMobClick                    // Move and click only if cursor is over mob
	MousePress                       // Press and hold
	MouseHold                        // Continue holding
	MouseRelease                     // Release held button
)

// Action provides game action capabilities through JavaScript injection.
//
// All operations use the Browser's context to inject JavaScript code that
// dispatches events to the game canvas element.
//
// Lifecycle:
//   1. Create Action with NewAction(browser)
//   2. Call action methods (SendKey, Click, etc.)
//   3. JavaScript is injected and executed in browser context
//   4. Events are dispatched to game canvas
//
// Error Handling:
// All methods use 2-second timeouts to prevent blocking.
// Errors are logged but typically not returned (fire-and-forget model).
type Action struct {
	browser *Browser
}

// NewAction creates a new Action instance
//
// Parameters:
//   - browser: Browser instance with active context
//
// Returns:
//   - *Action: New action controller
func NewAction(browser *Browser) *Action {
	return &Action{
		browser: browser,
	}
}

// SendKey simulates a keyboard event via JavaScript injection.
//
// This function calls the keyboardEvent() function in eval.js which dispatches
// KeyboardEvent to the game canvas element.
//
// Parameters:
//   - key: Key name (e.g., "w", "space", "F1", "escape")
//   - mode: KeyPress (tap), KeyHold (press down), or KeyRelease (release)
//
// Returns:
//   - error: Injection error, nil on success
//
// Algorithm:
//   1. Map KeyMode to JavaScript string ("press", "hold", "release")
//   2. Build JavaScript: keyboardEvent('mode', 'key')
//   3. For KeyPress mode, add optional duration for auto-release
//   4. Inject JavaScript via chromedp.Evaluate()
//   5. Log action for debug overlay
//
// Examples:
//   SendKey("w", KeyPress)           → keyboardEvent('press', 'w')
//   SendKey("F1", KeyPress)          → keyboardEvent('press', 'F1')
//   SendKey("space", KeyHold)        → keyboardEvent('hold', 'space')
//   SendKey("space", KeyRelease)     → keyboardEvent('release', 'space')
//
// Notes:
//   - Key names must match JavaScript KeyboardEvent.key values
//   - Function keys: "F1"-"F9"
//   - Special keys: "escape", "enter", "space"
//   - Letter keys: lowercase "w", "a", "s", "d", etc.
func (a *Action) SendKey(key string, mode KeyMode) error {
	if a.browser.ctx == nil || a.browser.ctx.Err() != nil {
		LogDebug("SendKey: browser context invalid")
		return fmt.Errorf("browser context is invalid")
	}

	var modeStr string
	var duration int // milliseconds

	switch mode {
	case KeyPress:
		modeStr = "press"
		duration = 0 // Auto-release handled by JavaScript
	case KeyHold:
		modeStr = "hold"
		duration = 0 // No auto-release
	case KeyRelease:
		modeStr = "release"
		duration = 0
	default:
		return fmt.Errorf("unsupported key mode: %d", mode)
	}

	// Build JavaScript injection
	var js string
	if duration > 0 {
		js = fmt.Sprintf("keyboardEvent('%s', '%s', %d);", modeStr, key, duration)
	} else {
		js = fmt.Sprintf("keyboardEvent('%s', '%s');", modeStr, key)
	}

	LogDebug("SendKey: injecting JavaScript: %s", js)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(a.browser.ctx, 2*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Evaluate(js, nil),
	)

	if err != nil {
		LogError("Failed to send key %s: %v", key, err)
		return err
	}

	// Log action for debug overlay
	a.browser.LogAction(fmt.Sprintf("Key %s: %s", modeStr, key))

	LogDebug("Key sent: %s (mode: %s)", key, modeStr)
	return nil
}

// SendSlot activates a skill slot via JavaScript injection.
//
// This function calls sendSlot() in eval.js which presses the F-key to select
// the skill bar and then presses the slot number.
//
// Parameters:
//   - slotBarIndex: Skill bar index (0-8, maps to F1-F9)
//   - slotIndex: Slot position within bar (0-9)
//
// Returns:
//   - error: Injection error, nil on success
//
// Algorithm:
//   1. Build JavaScript: sendSlot(slotBarIndex, slotIndex)
//   2. eval.js handles the F-key press and slot number press
//   3. Inject JavaScript via chromedp.Evaluate()
//   4. Log action for debug overlay
//
// Example:
//   SendSlot(0, 0) → Presses F1, then presses 0
//   SendSlot(1, 3) → Presses F2, then presses 3
//
// Notes:
//   - slotBarIndex is 0-based but maps to F1-F9 (F1=0, F2=1, etc.)
//   - slotIndex is typically 0-9 for the slot number
func (a *Action) SendSlot(slotBarIndex, slotIndex int) error {
	if a.browser.ctx == nil || a.browser.ctx.Err() != nil {
		LogDebug("SendSlot: browser context invalid")
		return fmt.Errorf("browser context is invalid")
	}

	// Build JavaScript injection
	js := fmt.Sprintf("sendSlot(%d, %d);", slotBarIndex, slotIndex)

	LogDebug("SendSlot: injecting JavaScript: %s", js)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(a.browser.ctx, 2*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Evaluate(js, nil),
	)

	if err != nil {
		LogError("Failed to send slot F%d-%d: %v", slotBarIndex+1, slotIndex, err)
		return err
	}

	// Log action for debug overlay
	a.browser.LogAction(fmt.Sprintf("Slot F%d-%d", slotBarIndex+1, slotIndex))

	LogDebug("Slot sent: F%d-%d", slotBarIndex+1, slotIndex)
	return nil
}

// Click simulates a mouse click via JavaScript injection.
//
// This function calls mouseEvent() in eval.js which dispatches MouseEvent
// to the game canvas element.
//
// Parameters:
//   - x: X coordinate (canvas-relative)
//   - y: Y coordinate (canvas-relative)
//   - mode: MouseClick (simple click) or MouseMobClick (check if cursor is over mob)
//
// Returns:
//   - error: Injection error, nil on success
//
// Algorithm:
//   1. Map MouseMode to JavaScript type string
//   2. Build JavaScript: mouseEvent('type', x, y, options)
//   3. For MouseMobClick, add checkMob option
//   4. Inject JavaScript via chromedp.Evaluate()
//   5. Log action for debug overlay
//
// Mouse Modes:
//   - MouseClick: Immediate click at position (moveClick with checkMob=false)
//   - MouseMobClick: Only click if cursor is over mob (moveClick with checkMob=true)
//
// Examples:
//   Click(400, 300, MouseClick)    → Clicks at (400, 300)
//   Click(400, 300, MouseMobClick) → Moves to (400, 300), waits 50ms, clicks only if mob detected
//
// Notes:
//   - Coordinates are relative to canvas element (not screen)
//   - MouseMobClick checks cursor style to detect mob hover
//   - Useful for avoiding misclicks when mob detection is uncertain
func (a *Action) Click(x, y int, mode MouseMode) error {
	if a.browser.ctx == nil || a.browser.ctx.Err() != nil {
		LogDebug("Click: browser context invalid")
		return fmt.Errorf("browser context is invalid")
	}

	var js string

	switch mode {
	case MouseClick:
		// Simple click without mob check
		js = fmt.Sprintf("mouseEvent('moveClick', %d, %d);", x, y)
	case MouseMobClick:
		// Click with mob detection check
		js = fmt.Sprintf("mouseEvent('moveClick', %d, %d, {checkMob: true});", x, y)
	case MouseMove:
		// Move only
		js = fmt.Sprintf("mouseEvent('move', %d, %d);", x, y)
	case MousePress:
		// Press and hold
		js = fmt.Sprintf("mouseEvent('press', %d, %d);", x, y)
	case MouseHold:
		// Hold down
		js = fmt.Sprintf("mouseEvent('hold', %d, %d);", x, y)
	case MouseRelease:
		// Release
		js = fmt.Sprintf("mouseEvent('release', %d, %d);", x, y)
	default:
		return fmt.Errorf("unsupported mouse mode: %d", mode)
	}

	LogDebug("Click: injecting JavaScript: %s", js)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(a.browser.ctx, 2*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Evaluate(js, nil),
	)

	if err != nil {
		LogError("Failed to click at (%d, %d): %v", x, y, err)
		return err
	}

	// Log action for debug overlay
	modeStr := "click"
	if mode == MouseMobClick {
		modeStr = "mob-click"
	}
	a.browser.LogAction(fmt.Sprintf("Mouse %s: (%d, %d)", modeStr, x, y))

	LogDebug("Mouse %s at (%d, %d)", modeStr, x, y)
	return nil
}

// MoveMouse moves the mouse cursor via JavaScript injection.
//
// Parameters:
//   - x: X coordinate (canvas-relative)
//   - y: Y coordinate (canvas-relative)
//
// Returns:
//   - error: Injection error, nil on success
func (a *Action) MoveMouse(x, y int) error {
	return a.Click(x, y, MouseMove)
}

// SendMessage sends a chat message via JavaScript injection.
//
// This function calls setInputChat() in eval.js which sets the chat input
// value and selects the text.
//
// Parameters:
//   - text: Message to send
//
// Returns:
//   - error: Injection error, nil on success
//
// Algorithm:
//   1. Build JavaScript: setInputChat('text')
//   2. eval.js sets input.value and calls input.select()
//   3. Inject JavaScript via chromedp.Evaluate()
//   4. Log action for debug overlay
//
// Example:
//   SendMessage("Hello world") → Sets chat input to "Hello world" and selects it
//
// Notes:
//   - Text is set in the input element but NOT automatically sent
//   - User still needs to press Enter to send the message
//   - This is useful for preparing messages or bot communication
func (a *Action) SendMessage(text string) error {
	if a.browser.ctx == nil || a.browser.ctx.Err() != nil {
		LogDebug("SendMessage: browser context invalid")
		return fmt.Errorf("browser context is invalid")
	}

	// Escape single quotes in text
	escapedText := escapeJavaScriptString(text)

	// Build JavaScript injection
	js := fmt.Sprintf("setInputChat('%s');", escapedText)

	LogDebug("SendMessage: injecting JavaScript: %s", js)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(a.browser.ctx, 2*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Evaluate(js, nil),
	)

	if err != nil {
		LogError("Failed to send message: %v", err)
		return err
	}

	// Log action for debug overlay
	a.browser.LogAction(fmt.Sprintf("Chat: %s", text))

	LogDebug("Message sent: %s", text)
	return nil
}

// escapeJavaScriptString escapes special characters in a string for JavaScript injection.
//
// This prevents injection attacks and syntax errors when embedding user-provided
// text into JavaScript code strings.
//
// Parameters:
//   - s: String to escape
//
// Returns:
//   - string: Escaped string safe for JavaScript string literals
//
// Escapes:
//   - Single quote (') → \'
//   - Backslash (\) → \\
//   - Newline (\n) → \\n
//   - Carriage return (\r) → \\r
func escapeJavaScriptString(s string) string {
	result := ""
	for _, char := range s {
		switch char {
		case '\'':
			result += "\\'"
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		case '\r':
			result += "\\r"
		default:
			result += string(char)
		}
	}
	return result
}

// Helper methods for backward compatibility with Platform interface

// MouseClick performs a simple click at the given coordinates
func (a *Action) MouseClick(x, y int) error {
	return a.Click(x, y, MouseClick)
}

// SendText sends text to the chat input
func (a *Action) SendText(text string) error {
	return a.SendMessage(text)
}

// TypeText types text (alias for SendText for compatibility)
func (a *Action) TypeText(text string) error {
	return a.SendText(text)
}
