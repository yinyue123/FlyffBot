// Package main - persistence.go
//
// This file implements data persistence for bot configuration and browser cookies.
// Uses JSON format for human-readable and easily editable storage.
//
// Persistent Data:
//   - Bot Configuration: Mode, skill slots, thresholds, mob colors, behavior settings
//   - Browser Cookies: Session cookies from universe.flyff.com for automatic login
//
// File Format:
// JSON with 2-space indentation for readability. Example structure:
// {
//   "config": {
//     "Mode": "Farming",
//     "AttackSlots": [0],
//     "HealSlots": [1],
//     "HealThreshold": 50,
//     ...
//   },
//   "cookies": [
//     {
//       "name": "session",
//       "value": "...",
//       "domain": "universe.flyff.com",
//       ...
//     }
//   ]
// }
//
// Save Triggers:
//   - User changes configuration via system tray
//   - Graceful shutdown (quit button, signal handler)
//   - Manual SaveState() calls
//
// Load Behavior:
//   - If data.json exists: Load configuration and cookies
//   - If file doesn't exist: Use default configuration, empty cookies
//   - If file is corrupted: Log error, use defaults
//
// Error Handling:
// Load errors are logged but do not prevent bot startup. The bot falls back
// to default configuration and continues running.
package main

import (
	"encoding/json"
	"os"
)

const dataFile = "data.json"

// SaveData saves configuration and cookies to data.json.
//
// Creates or overwrites the data file with current bot state. Uses JSON encoding
// with 2-space indentation for human readability.
//
// Parameters:
//   - data: PersistentData structure containing config and cookies
//
// Returns:
//   - error: File creation or encoding error, nil on success
//
// File Operations:
//   1. Create/truncate data.json file
//   2. Create JSON encoder with 2-space indent
//   3. Encode PersistentData to file
//   4. Close file handle (deferred)
//   5. Log success message
func SaveData(data *PersistentData) error {
	file, err := os.Create(dataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	LogInfo("Data saved to %s", dataFile)
	return nil
}

// LoadData loads configuration and cookies from data.json.
//
// Attempts to read and parse the data file. If the file doesn't exist or is
// corrupted, returns default configuration instead of failing.
//
// Returns:
//   - *PersistentData: Loaded or default configuration
//   - error: Nil on success or when using defaults, non-nil for unexpected errors
//
// Load Algorithm:
//   1. Check if data.json exists
//      - If not: Return new default configuration
//   2. Open file for reading
//   3. Create JSON decoder
//   4. Decode into PersistentData structure
//      - If decode fails: Log error, return defaults
//   5. Close file handle (deferred)
//   6. Log success message
//
// Error Recovery:
// Decode failures are handled gracefully by returning default configuration.
// This prevents the bot from crashing due to corrupted or manually edited files.
func LoadData() (*PersistentData, error) {
	// Check if file exists
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		LogInfo("No existing data file, creating new configuration")
		return NewPersistentData(), nil
	}

	file, err := os.Open(dataFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data PersistentData
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		LogError("Failed to decode data file: %v", err)
		return NewPersistentData(), nil
	}

	LogInfo("Data loaded from %s", dataFile)
	return &data, nil
}
