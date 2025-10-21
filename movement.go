// Package main - movement.go
//
// This file implements the MovementCoordinator that translates high-level behavior
// commands into platform-specific keyboard and mouse actions.
//
// Key Responsibilities:
//   - Character Movement: Forward, backward, rotation, jumping, circle patterns
//   - Skill Execution: Using numbered hotkey slots (0-9)
//   - Target Management: Clicking mobs, locking/canceling targets
//   - Obstacle Avoidance: Jump maneuvers in random directions
//   - Chat Interaction: Opening chat and sending messages
//   - Party Operations: Opening party menu and selecting members
//   - Camera Control: Random movements to avoid AFK detection
//
// Control Scheme (Flyff Universe):
//   - W/A/S/D: Character movement
//   - Space: Jump
//   - Left/Right Arrow: Camera rotation
//   - 0-9: Skill/item hotkey slots
//   - Z: Target lock / Follow target
//   - Escape: Cancel target
//   - Enter: Open/send chat
//   - P: Party menu
//   - T: Status tray
//
// Movement Patterns:
//   - CircleMove: Forward + jump + strafe creates circular farming pattern
//   - AvoidObstacle: Jump in random direction to get around obstacles
//   - RandomCameraMovement: Small rotation to prevent AFK timeout
//
// Timing Strategy:
// All actions include appropriate delays (10-800ms) to ensure the game
// registers inputs properly and animations can complete.
//
// Action Logging:
// Critical actions (clicks, key presses) are logged to the browser's action
// log for debug overlay display.
package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// MovementCoordinator coordinates character movements and actions.
//
// The coordinator abstracts action execution via JavaScript injection and provides
// high-level movement primitives that behaviors can compose into complex actions.
//
// Architecture:
//   - action: Low-level keyboard/mouse input simulation via JavaScript
//   - browser: Action logging for debug visualization
//   - rng: Random number generator for varied movement patterns
//
// Thread Safety:
// Not thread-safe. Should only be called from the main loop goroutine.
// Multiple simultaneous key operations are supported via HoldKeys/ReleaseKeys.
type MovementCoordinator struct {
	action     *Action
	browser    *Browser
	rng        *rand.Rand
	screenInfo *ScreenInfo
}

// NewMovementCoordinator creates a new movement coordinator
func NewMovementCoordinator(action *Action, browser *Browser) *MovementCoordinator {
	return &MovementCoordinator{
		action:  action,
		browser: browser,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// PressKey presses a single key
func (mc *MovementCoordinator) PressKey(key string) {
	mc.action.SendKey(key, KeyPress)
	if mc.browser != nil {
		mc.browser.LogAction("Press key: " + key)
	}
	time.Sleep(10 * time.Millisecond)
}

// HoldKey holds a key down
func (mc *MovementCoordinator) HoldKey(key string) {
	mc.action.SendKey(key, KeyHold)
}

// ReleaseKey releases a held key
func (mc *MovementCoordinator) ReleaseKey(key string) {
	mc.action.SendKey(key, KeyRelease)
}

// HoldKeys holds multiple keys simultaneously
func (mc *MovementCoordinator) HoldKeys(keys []string) {
	for _, key := range keys {
		mc.HoldKey(key)
		time.Sleep(10 * time.Millisecond)
	}
}

// ReleaseKeys releases multiple held keys
func (mc *MovementCoordinator) ReleaseKeys(keys []string) {
	for _, key := range keys {
		mc.ReleaseKey(key)
		time.Sleep(10 * time.Millisecond)
	}
}

// Wait waits for specified duration
func (mc *MovementCoordinator) Wait(duration time.Duration) {
	time.Sleep(duration)
}

// WaitRandom waits for a random duration between min and max
func (mc *MovementCoordinator) WaitRandom(minMs, maxMs int) {
	duration := time.Duration(minMs+rand.Intn(maxMs-minMs+1)) * time.Millisecond
	time.Sleep(duration)
}

// Jump performs a jump action
func (mc *MovementCoordinator) Jump() {
	mc.HoldKey("space")
	mc.Wait(500 * time.Millisecond)
	mc.ReleaseKey("space")
}

// MoveForward moves character forward for specified duration
func (mc *MovementCoordinator) MoveForward(duration time.Duration) {
	mc.HoldKey("w")
	mc.Wait(duration)
	mc.ReleaseKey("w")
}

// MoveBackward moves character backward for specified duration
func (mc *MovementCoordinator) MoveBackward(duration time.Duration) {
	mc.HoldKey("s")
	mc.Wait(duration)
	mc.ReleaseKey("s")
}

// RotateLeft rotates character left for specified duration
func (mc *MovementCoordinator) RotateLeft(duration time.Duration) {
	mc.HoldKey("left")
	mc.Wait(duration)
	mc.ReleaseKey("left")
}

// RotateRight rotates character right for specified duration
func (mc *MovementCoordinator) RotateRight(duration time.Duration) {
	mc.HoldKey("right")
	mc.Wait(duration)
	mc.ReleaseKey("right")
}

// RotateRandom rotates in a random direction
func (mc *MovementCoordinator) RotateRandom(duration time.Duration) {
	if mc.rng.Float64() < 0.5 {
		mc.RotateLeft(duration)
	} else {
		mc.RotateRight(duration)
	}
}

// CircleMove performs circular movement pattern
// Used to stay in a farming area while searching for mobs
func (mc *MovementCoordinator) CircleMove(rotateDuration time.Duration) {
	LogDebug("Performing circle movement")

	// Move forward + jump + rotate (creates circular pattern)
	mc.HoldKeys([]string{"w", "space", "d"})
	mc.Wait(rotateDuration)

	// Stop rotation
	mc.ReleaseKey("d")
	mc.Wait(20 * time.Millisecond)

	// Stop forward movement and jump
	mc.ReleaseKeys([]string{"space", "w"})

	// Small backward adjustment
	mc.HoldKey("s")
	mc.Wait(50 * time.Millisecond)
	mc.ReleaseKey("s")
}

// AvoidObstacle attempts to avoid obstacle by jumping in random direction
func (mc *MovementCoordinator) AvoidObstacle(attempt int) {
	LogDebug("Avoiding obstacle (attempt %d)", attempt)

	if attempt == 0 {
		// First attempt: target lock + forward jump
		mc.PressKey("z")
		mc.HoldKeys([]string{"w", "space"})
		mc.Wait(800 * time.Millisecond)
		mc.ReleaseKeys([]string{"space", "w"})
	} else {
		// Subsequent attempts: random direction jump
		direction := "a"
		if mc.rng.Float64() < 0.5 {
			direction = "d"
		}

		mc.HoldKeys([]string{"w", "space"})
		mc.HoldKey(direction)
		mc.Wait(200 * time.Millisecond)
		mc.ReleaseKey(direction)
		mc.Wait(600 * time.Millisecond)
		mc.ReleaseKeys([]string{"space", "w"})
		mc.PressKey("z")
	}
}

// ClickTarget clicks on a target at given coordinates
func (mc *MovementCoordinator) ClickTarget(point Point) {
	LogDebug("Clicking target at (%d, %d)", point.X, point.Y)
	mc.action.MouseClick(point.X, point.Y)
	if mc.browser != nil {
		mc.browser.LogAction(fmt.Sprintf("Click at (%d, %d)", point.X, point.Y))
	}
}

// UseSlot uses a skill/item slot (F1-F9 + number 0-9)
func (mc *MovementCoordinator) UseSlot(slotNum int) {
	if slotNum < 0 || slotNum > 9 {
		LogWarn("Invalid slot number: %d", slotNum)
		return
	}

	// Note: In Flyff, F1-F9 are slot bars, and 0-9 are the slots within each bar
	// We'll use F1 as the default bar and numbers 0-9 for slots
	// For full implementation, you'd need bar selection logic

	key := ""
	switch slotNum {
	case 0:
		key = "0"
	case 1:
		key = "1"
	case 2:
		key = "2"
	case 3:
		key = "3"
	case 4:
		key = "4"
	case 5:
		key = "5"
	case 6:
		key = "6"
	case 7:
		key = "7"
	case 8:
		key = "8"
	case 9:
		key = "9"
	}

	if key != "" {
		LogDebug("Using slot %d", slotNum)
		mc.PressKey(key)
	}
}

// UseSkill uses a skill from configured slot list
func (mc *MovementCoordinator) UseSkill(slots []int) bool {
	if len(slots) == 0 {
		return false
	}

	// Use first available slot
	// In a full implementation, you'd track cooldowns
	slot := slots[0]
	mc.UseSlot(slot)
	return true
}

// LockTarget locks onto current target (Z key)
func (mc *MovementCoordinator) LockTarget() {
	LogDebug("Locking target")
	mc.PressKey("z")
}

// CancelTarget cancels current target (Escape key)
func (mc *MovementCoordinator) CancelTarget() {
	LogDebug("Canceling target")
	mc.PressKey("escape")
}

// OpenChat opens chat window
func (mc *MovementCoordinator) OpenChat() {
	mc.PressKey("enter")
	mc.Wait(100 * time.Millisecond)
}

// SendChatMessage sends a message in chat
func (mc *MovementCoordinator) SendChatMessage(message string) {
	LogDebug("Sending chat message: %s", message)
	mc.OpenChat()
	mc.action.SendText(message)
}

// OpenPartyMenu opens party menu (P key)
func (mc *MovementCoordinator) OpenPartyMenu() {
	mc.PressKey("p")
	mc.Wait(150 * time.Millisecond)
}

// ClosePartyMenu closes party menu
func (mc *MovementCoordinator) ClosePartyMenu() {
	mc.PressKey("p")
}

// SelectPartyLeader attempts to select party leader
// This is a simplified version; actual implementation may need screen coordinates
func (mc *MovementCoordinator) SelectPartyLeader(screenInfo *ScreenInfo) {
	LogDebug("Selecting party leader")

	mc.OpenPartyMenu()

	// Click on leader position (scaled based on resolution)
	leaderX, leaderY := screenInfo.Scale(213, 440)
	mc.action.MouseClick(leaderX, leaderY)

	mc.Wait(100 * time.Millisecond)
	mc.LockTarget()
	mc.Wait(10 * time.Millisecond)
	mc.ClosePartyMenu()
	mc.Wait(500 * time.Millisecond)
}

// FollowTarget initiates following current target
func (mc *MovementCoordinator) FollowTarget() {
	mc.LockTarget() // Z key follows the target in Flyff
}

// OpenStatusTray opens the status tray (T key)
func (mc *MovementCoordinator) OpenStatusTray() {
	LogDebug("Opening status tray")
	mc.PressKey("t")
}

// PerformPickup performs pickup action
func (mc *MovementCoordinator) PerformPickup(repeatCount int) {
	LogDebug("Performing pickup (%d times)", repeatCount)

	// Use pickup slot repeatedly
	for i := 0; i < repeatCount; i++ {
		// Assuming pickup is on a specific slot
		// You'd get this from config
		mc.Wait(300 * time.Millisecond)
	}
}

// RandomCameraMovement performs random camera movement to avoid AFK detection
func (mc *MovementCoordinator) RandomCameraMovement() {
	duration := time.Duration(50+mc.rng.Intn(50)) * time.Millisecond
	mc.RotateRight(duration)
	mc.Wait(50 * time.Millisecond)
}

// StopAllMovement stops all movement keys
func (mc *MovementCoordinator) StopAllMovement() {
	keys := []string{"w", "a", "s", "d", "space", "left", "right"}
	for _, key := range keys {
		mc.ReleaseKey(key)
	}
}

// EmergencyStop immediately stops all actions
func (mc *MovementCoordinator) EmergencyStop() {
	LogWarn("Emergency stop triggered")
	mc.StopAllMovement()
	mc.CancelTarget()
}

// TypeText types a text message (for chat)
func (mc *MovementCoordinator) TypeText(text string) {
	// Simulate typing by sending each character
	// This is a simplified version - actual implementation may need action-specific text input
	mc.action.TypeText(text)
	if mc.browser != nil {
		mc.browser.LogAction("Type: " + text)
	}
}

// HoldKeyFor holds a key for a specific duration then releases it
func (mc *MovementCoordinator) HoldKeyFor(key string, duration time.Duration) {
	mc.HoldKey(key)
	time.Sleep(duration)
	mc.ReleaseKey(key)
}
