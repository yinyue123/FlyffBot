// Package main - main.go
//
// Entry point for the Flyff bot application.
package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func main() {
	// Lock to main thread for UI operations (macOS requirement)
	runtime.LockOSThread()

	// Get config path from command line arguments
	configPath := ""
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Initialize config
	cfg, err := InitConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}
	defer cfg.Close()

	cfg.Log("Flyff Bot starting...")

	// Create debug manager
	debug := NewDebug(&cfg.Stat)
	defer debug.Close()

	// Create debug windows (must be on main thread)
	debug.CreateDebug()

	// Create browser
	browser := NewBrowser()
	defer browser.Stop()

	// Start browser
	err = browser.Start(cfg)
	if err != nil {
		log.Fatalf("Failed to start browser: %v", err)
	}

	// Create client detector with debug support
	detector := NewClientDetect(cfg)
	detector.Debug = cfg.GetDebug()
	detector.DebugUI = debug
	defer detector.Close()

	// Create farming behavior
	farming := NewFarming(cfg, browser, detector)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start farming in a goroutine
	go farming.Start()

	// Main loop: process debug updates while waiting for shutdown
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			cfg.Log("Received shutdown signal, stopping...")
			goto shutdown
		case <-ticker.C:
			// Process debug window updates on main thread
			debug.ProcessUpdates()
		}
	}

shutdown:
	// Save cookies before exit
	if err := browser.SaveCookie(cfg); err != nil {
		cfg.Log("Failed to save cookies: %v", err)
	}

	cfg.Log("Flyff Bot stopped")
}
