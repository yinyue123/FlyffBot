// Package main - utils.go
//
// This file provides utility functions and helper structures used throughout the bot.
// Includes performance timing, rate limiting, and math utilities.
//
// Major Components:
//
// 1. Performance Timing:
//    - Timer struct for measuring operation duration
//    - Automatic logging of elapsed time
//    - Used throughout codebase for performance monitoring
//
// 2. Rate Limiting:
//    - RateLimiter enforces minimum time between operations
//    - Thread-safe with mutex protection
//    - Useful for throttling actions to prevent detection
//
// 3. Utility Functions:
//    - FormatDuration: Converts duration to human-readable string (e.g., "2m 30s")
//    - FormatFloat: Formats floats with specified decimal places
//    - Clamp/ClampFloat: Restricts values to min/max range
//    - SafeGo: Launches goroutines with panic recovery
//
// Performance Monitoring:
// Timer objects are used extensively to measure:
//   - Main loop iteration time (target: 60-120ms)
//   - Screenshot capture (target: 10-50ms)
//   - Image recognition (target: 1-5ms)
//   - Behavior execution (varies)
//
// SafeGo Usage:
// All long-running goroutines use SafeGo to prevent panics from crashing
// the entire application. Panics are logged and the goroutine terminates
// gracefully while the rest of the bot continues operating.
//
// Note: Logging functionality has been moved to debug.go
package main

import (
	"fmt"
	"sync"
	"time"
)

// Timer provides performance timing functionality
type Timer struct {
	name      string
	startTime time.Time
}

// NewTimer creates and starts a new timer with given name
func NewTimer(name string) *Timer {
	return &Timer{
		name:      name,
		startTime: time.Now(),
	}
}

// Elapsed returns the elapsed time since timer creation
func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.startTime)
}

// Log logs the elapsed time with the timer name
func (t *Timer) Log() {
	elapsed := t.Elapsed()
	LogDebug("Timer [%s]: %v", t.name, elapsed)
}

// Stop logs the elapsed time and returns the duration
func (t *Timer) Stop() time.Duration {
	elapsed := t.Elapsed()
	LogDebug("Timer [%s] stopped: %v", t.name, elapsed)
	return elapsed
}

// FormatDuration formats a duration into human-readable string
func FormatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// FormatFloat formats a float to specified decimal places
func FormatFloat(value float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	return fmt.Sprintf(format, value)
}

// Clamp restricts a value between min and max
func Clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampFloat restricts a float value between min and max
func ClampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// SafeGo runs a function in a goroutine with panic recovery
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				LogError("Panic recovered in goroutine: %v", r)
			}
		}()
		fn()
	}()
}

// RateLimiter limits execution rate
type RateLimiter struct {
	lastExec time.Time
	interval time.Duration
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter with specified interval
func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{
		interval: interval,
	}
}

// Allow checks if enough time has passed since last execution
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastExec) >= rl.interval {
		rl.lastExec = now
		return true
	}
	return false
}

// Reset resets the rate limiter
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.lastExec = time.Time{}
}
