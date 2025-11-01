// Package main - shout.go
//
// This file implements the Shout Behavior for periodic message broadcasting.
// It handles cycling through configured messages and sending them at intervals.
//
// State Machine States:
//   - Idle: Waiting for next shout interval
//   - Shouting: Currently sending a message
//
// State Transitions:
//   Idle -> Shouting (interval elapsed)
//   Shouting -> Idle (message sent)
//
// Key Features:
//   - Configurable message list with cycling
//   - Adjustable shout interval
//   - Automatic empty message filtering
//   - Chat box automation (open, type, send, close)
//   - Timing variations to appear more natural
package main

import (
	"strings"
	"time"
)

// ShoutState represents the current state of the shout behavior
type ShoutState int

const (
	ShoutStateIdle ShoutState = iota
	ShoutStateShouting
)

// String returns the string representation of the state
func (s ShoutState) String() string {
	switch s {
	case ShoutStateIdle:
		return "Idle"
	case ShoutStateShouting:
		return "Shouting"
	default:
		return "Unknown"
	}
}

// ShoutBehavior implements periodic message broadcasting with state machine
type ShoutBehavior struct {
	// State machine
	state ShoutState

	// Configuration
	shoutMessages []string
	shoutInterval time.Duration

	// Timing
	lastShoutTime time.Time

	// Message cycling
	currentMessageIndex int
}

// NewShoutBehavior creates a new shout behavior
func NewShoutBehavior() *ShoutBehavior {
	return &ShoutBehavior{
		state:               ShoutStateIdle,
		shoutMessages:       make([]string, 0),
		shoutInterval:       30 * time.Second,
		lastShoutTime:       time.Now(),
		currentMessageIndex: 0,
	}
}

// GetState returns the current state name
func (sb *ShoutBehavior) GetState() string {
	return sb.state.String()
}

// Run executes one iteration of shout behavior
func (sb *ShoutBehavior) Run(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, stats *Statistics) error {
	// Update configuration
	sb.updateConfig(config)

	// State machine execution
	sb.state = sb.runStateMachine(movement, config)

	return nil
}

// updateConfig updates configuration from config
func (sb *ShoutBehavior) updateConfig(config *Config) {
	// Update shout messages if changed
	if config.ShoutMessages != nil && len(config.ShoutMessages) > 0 {
		sb.shoutMessages = config.ShoutMessages
	}

	// Update shout interval if changed
	if config.ShoutInterval > 0 {
		sb.shoutInterval = time.Duration(config.ShoutInterval) * time.Millisecond
	}
}

// runStateMachine executes the state machine and returns next state
func (sb *ShoutBehavior) runStateMachine(movement *MovementCoordinator, config *Config) ShoutState {
	switch sb.state {
	case ShoutStateIdle:
		return sb.onIdle(movement, config)
	case ShoutStateShouting:
		return sb.onShouting(movement)
	default:
		return ShoutStateIdle
	}
}

// onIdle handles the idle state
func (sb *ShoutBehavior) onIdle(movement *MovementCoordinator, config *Config) ShoutState {
	// Check if it's time to shout
	if time.Since(sb.lastShoutTime) >= sb.shoutInterval {
		// Check if we have messages
		if len(sb.shoutMessages) == 0 {
			LogWarn("No shout messages configured")
			sb.lastShoutTime = time.Now()
			return ShoutStateIdle
		}

		// Get next message
		message := sb.getNextMessage()
		if message == "" {
			// Empty message, skip and try next
			sb.lastShoutTime = time.Now()
			return ShoutStateIdle
		}

		// Transition to shouting state
		LogDebug("Shouting message: %s", message)
		sb.performShout(movement, message)

		return ShoutStateShouting
	}

	return ShoutStateIdle
}

// onShouting handles the shouting state
func (sb *ShoutBehavior) onShouting(movement *MovementCoordinator) ShoutState {
	// Shouting is performed in onIdle, so we just transition back
	sb.lastShoutTime = time.Now()
	return ShoutStateIdle
}

// getNextMessage gets the next message to shout
func (sb *ShoutBehavior) getNextMessage() string {
	if len(sb.shoutMessages) == 0 {
		return ""
	}

	// Get current message
	message := sb.shoutMessages[sb.currentMessageIndex]

	// Move to next message (cycle)
	sb.currentMessageIndex = (sb.currentMessageIndex + 1) % len(sb.shoutMessages)

	// Trim and return
	return strings.TrimSpace(message)
}

// performShout performs the shout action
func (sb *ShoutBehavior) performShout(movement *MovementCoordinator, message string) {
	// Avoid sending empty messages
	if strings.TrimSpace(message) == "" {
		return
	}

	LogInfo("Broadcasting: %s", message)

	// Open chatbox
	movement.PressKey("Enter")
	movement.WaitRandom(100, 250)

	// Type message
	movement.TypeText(message)
	movement.WaitRandom(100, 200)

	// Send message
	movement.PressKey("Enter")
	movement.WaitRandom(100, 250)

	// Close chatbox
	movement.PressKey("Escape")
	movement.Wait(100 * time.Millisecond)
}

// Stop stops the shout behavior
func (sb *ShoutBehavior) Stop() {
	sb.state = ShoutStateIdle
}
