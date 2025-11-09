// Package main - config.go
//
// This file manages configuration, status, and cookie data for the bot.
// It handles three JSON files:
// - stat.json: Configuration (read-only, read every second)
// - cookie.json: Browser cookies (read at startup, write at exit)
// - status.json: Current status (write-only, write every second)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// SlotType represents the type of slot action
const (
	SlotTypeAttack     = 1  // Attack skill
	SlotTypeBuff       = 2  // Buff skill
	SlotTypeHeal       = 3  // Heal skill
	SlotTypeRescue     = 4  // Rescue/Resurrection skill
	SlotTypeBoard      = 5  // Board/Mount skill
	SlotTypeFood       = 11 // HP food
	SlotTypePill       = 12 // HP pill
	SlotTypeMPRestore  = 21 // MP restore
	SlotTypeFPRestore  = 22 // FP restore
	SlotTypePick       = 31 // Pickup
	SlotTypePet        = 32 // Pet summon
)

// Global cooldown times for each action type (in milliseconds)
const (
	CooldownAttack  = 300   // Attack cooldown
	CooldownHPFood  = 2500  // HP food cooldown
	CooldownHPPill  = 10000 // HP pill cooldown
	CooldownMP      = 1500  // MP restore cooldown
	CooldownFP      = 1500  // FP restore cooldown
)

// Slot represents a skill/item slot configuration
type Slot struct {
	Page      int  `json:"page"`                // Page position, range 1-9
	Slot      int  `json:"slot"`                // Slot position, range 0-9
	Type      int  `json:"type"`                // Slot type (see constants above)
	Threshold *int `json:"threshold,omitempty"` // Threshold to use this slot (%), can be nil
	Cooldown  *int `json:"cooldown,omitempty"`  // Cooldown in milliseconds, can be nil
	Enable    bool `json:"enable"`              // Whether this slot is enabled
}

// AttackSettings holds attack-related configuration
type AttackSettings struct {
	AttackMinHP            int `json:"attackMinHP"`            // Minimum HP to attack
	DefeatInterval         int `json:"defeatInterval"`         // Wait time after killing a mob (ms)
	ObstacleThresholdTime  int `json:"obstacleThresholdTime"`  // Time threshold to detect obstacle (ms)
	ObstacleAvoidCount     int `json:"obstacleAvoidCount"`     // Max obstacle avoidance attempts
	ObstacleCoolDown       int `json:"obstacleCoolDown"`       // Cooldown between obstacle attempts (ms)
	EscapeHP               int `json:"escapeHp"`               // HP threshold to escape (%)
	MaxTime                int `json:"maxTime"`                // Max attack time before giving up (seconds)
}

// Settings holds general bot settings
type Settings struct {
	BuffInterval   int    `json:"buffInterval"`   // Wait time after using a buff (ms)
	DeathConfirm   int    `json:"deathConfirm"`   // Interval to press enter after death (ms)
	ShoutMessage   string `json:"shoutMessage"`   // Shout message content
	ShoutInterval  int    `json:"shoutInterval"`  // Shout interval (seconds)
	WatchDogTime   int    `json:"watchDogTime"`   // Watchdog timeout (seconds)
	WatchDogRetry  int    `json:"watchDogRetry"`  // Max watchdog retry attempts
}

// Stat holds configuration data (read from stat.json)
type Stat struct {
	Enable         bool           `json:"enable"`     // Whether main program is running
	Restorer       bool           `json:"restorer"`   // Whether to perform recovery
	Detect         bool           `json:"detect"`     // Whether to auto-detect mobs
	Navigate       bool           `json:"navigate"`   // Whether navigation is enabled
	Debug          bool           `json:"debug"`      // Whether to save debug screenshots
	Type           int            `json:"type"`       // 0=disable, 1=farming, 2=support, 3=auto shout
	Slots          []Slot         `json:"slots"`      // Slot configurations
	Attack         AttackSettings `json:"attack"`     // Attack settings
	Settings       Settings       `json:"settings"`   // General settings
	StatusPath     string         `json:"status"`     // Status file path
	CookiesPath    string         `json:"cookies"`    // Cookies file path
	LogPath        string         `json:"log"`        // Log file path
	BrowserLogPath string         `json:"browserLog"` // Browser log file path
}

// Cookie represents a browser cookie
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// PlayerStatus holds player status information
type PlayerStatus struct {
	HP             int       `json:"hp"`
	MP             int       `json:"mp"`
	FP             int       `json:"fp"`
	CurrentPage    int       `json:"currentPage"`
	StartTime      time.Time `json:"-"`        // Internal: program start time
	StartTimeMS    int       `json:"startTime"` // JSON: elapsed time since start (ms)
	Killed         int       `json:"killed"`
	LastKilledTime time.Time `json:"-"`            // Internal: last kill time
	LastKilledMS   int       `json:"lastKilledTime"` // JSON: time since last kill (ms)
	Stage          string    `json:"stage"`
}

// TargetStatus holds target (mob) status information
type TargetStatus struct {
	Passive bool `json:"passive"`
	Level   int  `json:"level"`
	HP      int  `json:"hp"`
	MP      int  `json:"mp"`
}

// AttackStatus holds attack-related status
type AttackStatus struct {
	LastUpdateHP       int       `json:"lastUpdateHp"`
	LastUpdateTime     time.Time `json:"-"`              // Internal: last HP update time
	LastUpdateMS       int       `json:"lastUpdateTime"` // JSON: time since last HP update (ms)
	ObstacleAvoidCount int       `json:"obstacleAvoidCount"`
	AttackTime         time.Time `json:"-"`        // Internal: attack start time
	AttackTimeMS       int       `json:"attackTime"` // JSON: time since attack started (ms)
}

// Cooldown holds cooldown information (internal representation uses time.Time)
type Cooldown struct {
	Attack   time.Time            `json:"attack"`   // Next attack available time
	HPFood   time.Time            `json:"hp_food"`  // Next HP food available time
	HPPill   time.Time            `json:"hp_pill"`  // Next HP pill available time
	MP       time.Time            `json:"mp"`       // Next MP restore available time
	FP       time.Time            `json:"fp"`       // Next FP restore available time
	Buff     time.Time            `json:"buff"`     // Next buff available time
	Obstacle time.Time            `json:"obstacle"` // Next obstacle avoidance available time
	Slots    map[string]time.Time `json:"slots"`    // Slot cooldowns (format "page:slot" -> next available time)
}

// CooldownJSON is the JSON representation of Cooldown (in milliseconds remaining)
type CooldownJSON struct {
	Attack   int            `json:"attack"`   // Attack cooldown remaining (ms)
	HPFood   int            `json:"hp_food"`  // HP food cooldown remaining (ms)
	HPPill   int            `json:"hp_pill"`  // HP pill cooldown remaining (ms)
	MP       int            `json:"mp"`       // MP cooldown remaining (ms)
	FP       int            `json:"fp"`       // FP cooldown remaining (ms)
	Buff     int            `json:"buff"`     // Buff cooldown remaining (ms)
	Obstacle int            `json:"obstacle"` // Obstacle avoidance cooldown remaining (ms)
	Slots    map[string]int `json:"slots"`    // Slot cooldowns (format "page:slot" -> remaining ms)
}

// Status holds current bot status (written to status.json)
type Status struct {
	Player       PlayerStatus   `json:"player"`
	Target       *TargetStatus  `json:"target"`   // nil if no target selected
	Attack       AttackStatus   `json:"attack"`
	Actions      []string       `json:"actions"`  // Last 10 actions
	Cooldown     Cooldown       `json:"-"`        // Internal cooldown (not serialized)
	CooldownJSON CooldownJSON   `json:"cooldown"` // JSON representation of cooldown
	Mobs         []string       `json:"mobs"`     // List of detected mobs (format: "(x,y,w,h,type)")
}

// Config is the main configuration object
type Config struct {
	Stat           Stat      // Configuration data
	Status         Status    // Current status
	Cookies        []Cookie  // Browser cookies
	LogFile        *os.File  // Log file handle
	BrowserLogFile *os.File  // Browser log file handle
	StatPath       string    // Path to stat.json
	mu             sync.RWMutex
}

// InitConfig initializes the config object
func InitConfig(path string) (*Config, error) {
	if path == "" {
		path = "stat.json"
	}

	now := time.Now()
	cfg := &Config{
		StatPath: path,
		Status: Status{
			Player: PlayerStatus{
				StartTime:      now,
				LastKilledTime: now, // Initialize to current time to prevent watchdog timeout
				Stage:          "initializing",
			},
			Actions: make([]string, 0, 10),
			Cooldown: Cooldown{
				Slots: make(map[string]time.Time),
			},
			Mobs: make([]string, 0),
		},
		Cookies: make([]Cookie, 0),
	}

	// Check if stat.json exists, if not create default
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg.createDefaultStat()
		if err := cfg.saveStat(); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
	}

	// Load configuration
	if err := cfg.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Open log file if path is specified
	if cfg.Stat.LogPath != "" {
		logFile, err := os.OpenFile(cfg.Stat.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		cfg.LogFile = logFile
		log.SetOutput(logFile)
	}

	// Open browser log file if path is specified
	if cfg.Stat.BrowserLogPath != "" {
		browserLogFile, err := os.OpenFile(cfg.Stat.BrowserLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open browser log file: %w", err)
		}
		cfg.BrowserLogFile = browserLogFile
	}

	return cfg, nil
}

// createDefaultStat creates default configuration
func (c *Config) createDefaultStat() {
	threshold0 := 0
	threshold50 := 50
	threshold30 := 30
	cooldown1500 := 1500
	cooldown3000 := 3000
	cooldown30000 := 30000

	c.Stat = Stat{
		Enable:   true,
		Restorer: true,
		Detect:   true,
		Navigate: true,
		Debug:    false,
		Type:     1, // Farming mode
		Slots: []Slot{
			{Page: 1, Slot: 1, Type: SlotTypeAttack, Threshold: &threshold0, Cooldown: &cooldown1500, Enable: true},
			{Page: 1, Slot: 2, Type: SlotTypeFood, Threshold: &threshold50, Cooldown: &cooldown3000, Enable: true},
			{Page: 1, Slot: 3, Type: SlotTypePill, Threshold: &threshold30, Cooldown: &cooldown30000, Enable: true},
			{Page: 1, Slot: 4, Type: SlotTypeMPRestore, Threshold: &threshold30, Cooldown: &cooldown30000, Enable: true},
		},
		Attack: AttackSettings{
			AttackMinHP:           30,
			DefeatInterval:        1000,
			ObstacleThresholdTime: 10000,
			ObstacleAvoidCount:    20,
			ObstacleCoolDown:      1000,
			EscapeHP:              10,
			MaxTime:               300,
		},
		Settings: Settings{
			BuffInterval:  1000,
			DeathConfirm:  1000,
			ShoutMessage:  "123",
			ShoutInterval: 30,
			WatchDogTime:  600,
			WatchDogRetry: 3,
		},
		StatusPath:     "status.json",
		CookiesPath:    "cookie.json",
		LogPath:        "bot.log",
		BrowserLogPath: "browser.log",
	}
}

// LoadConfig reads and updates the config from stat.json
func (c *Config) LoadConfig() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.StatPath)
	if err != nil {
		return fmt.Errorf("failed to read stat file: %w", err)
	}

	if err := json.Unmarshal(data, &c.Stat); err != nil {
		return fmt.Errorf("failed to parse stat file: %w", err)
	}

	// Load cookies if configured
	if c.Stat.CookiesPath != "" {
		if err := c.loadCookies(); err != nil {
			c.Log("Warning: failed to load cookies: %v", err)
		}
	}

	return nil
}

// saveStat saves the stat configuration to file
func (c *Config) saveStat() error {
	data, err := json.MarshalIndent(c.Stat, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stat: %w", err)
	}

	if err := os.WriteFile(c.StatPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write stat file: %w", err)
	}

	return nil
}

// loadCookies loads cookies from cookie.json
func (c *Config) loadCookies() error {
	data, err := os.ReadFile(c.Stat.CookiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.Cookies = make([]Cookie, 0)
			return nil
		}
		return fmt.Errorf("failed to read cookie file: %w", err)
	}

	if err := json.Unmarshal(data, &c.Cookies); err != nil {
		return fmt.Errorf("failed to parse cookie file: %w", err)
	}

	return nil
}

// SaveCookies saves cookies to cookie.json
func (c *Config) SaveCookies() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.MarshalIndent(c.Cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	if err := os.WriteFile(c.Stat.CookiesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cookie file: %w", err)
	}

	return nil
}

// SaveStatus writes current status to status.json
func (c *Config) SaveStatus() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Update PlayerStatus relative times
	c.Status.Player.StartTimeMS = timeToElapsed(c.Status.Player.StartTime, now)
	c.Status.Player.LastKilledMS = timeToElapsed(c.Status.Player.LastKilledTime, now)

	// Update AttackStatus relative times
	c.Status.Attack.LastUpdateMS = timeToElapsed(c.Status.Attack.LastUpdateTime, now)
	c.Status.Attack.AttackTimeMS = timeToElapsed(c.Status.Attack.AttackTime, now)

	// Update CooldownJSON from Cooldown (convert time.Time to remaining ms)
	c.Status.CooldownJSON = CooldownJSON{
		Attack:   timeToRemaining(c.Status.Cooldown.Attack, now),
		HPFood:   timeToRemaining(c.Status.Cooldown.HPFood, now),
		HPPill:   timeToRemaining(c.Status.Cooldown.HPPill, now),
		MP:       timeToRemaining(c.Status.Cooldown.MP, now),
		FP:       timeToRemaining(c.Status.Cooldown.FP, now),
		Buff:     timeToRemaining(c.Status.Cooldown.Buff, now),
		Obstacle: timeToRemaining(c.Status.Cooldown.Obstacle, now),
		Slots:    make(map[string]int),
	}

	// Convert slot cooldowns to remaining time
	for key, nextAvailable := range c.Status.Cooldown.Slots {
		c.Status.CooldownJSON.Slots[key] = timeToRemaining(nextAvailable, now)
	}

	data, err := json.MarshalIndent(c.Status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	if err := os.WriteFile(c.Stat.StatusPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write status file: %w", err)
	}

	return nil
}

// timeToRemaining converts a future time to remaining milliseconds (for cooldowns)
func timeToRemaining(t time.Time, now time.Time) int {
	if t.IsZero() {
		return 0
	}
	remaining := int(t.Sub(now).Milliseconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// timeToElapsed converts a past time to elapsed milliseconds (for timestamps)
func timeToElapsed(t time.Time, now time.Time) int {
	if t.IsZero() {
		return 0
	}
	elapsed := int(now.Sub(t).Milliseconds())
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// GetAvailableSlot finds an available slot of the given type
// Returns (page, slot) or (-1, -1) if none available
func (c *Config) GetAvailableSlot(slotType int, currentValue int) (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Check global type cooldown first
	switch slotType {
	case SlotTypeAttack:
		if c.Status.Cooldown.Attack.After(now) {
			return -1, -1
		}
	case SlotTypeFood:
		if c.Status.Cooldown.HPFood.After(now) {
			return -1, -1
		}
	case SlotTypePill:
		if c.Status.Cooldown.HPPill.After(now) {
			return -1, -1
		}
	case SlotTypeMPRestore:
		if c.Status.Cooldown.MP.After(now) {
			return -1, -1
		}
	case SlotTypeFPRestore:
		if c.Status.Cooldown.FP.After(now) {
			return -1, -1
		}
	}

	// Clean up expired cooldowns
	for key, nextAvailable := range c.Status.Cooldown.Slots {
		if nextAvailable.Before(now) || nextAvailable.Equal(now) {
			delete(c.Status.Cooldown.Slots, key)
		}
	}

	// Find matching slots
	var candidates []Slot
	for _, slot := range c.Stat.Slots {
		if !slot.Enable {
			continue
		}

		// Check slot type
		if slot.Type != slotType {
			continue
		}

		// Check threshold (if specified)
		if slot.Threshold != nil && currentValue > *slot.Threshold {
			continue
		}

		// Check slot-specific cooldown
		key := fmt.Sprintf("%d:%d", slot.Page, slot.Slot)
		if nextAvailable, exists := c.Status.Cooldown.Slots[key]; exists {
			if nextAvailable.After(now) {
				continue
			}
		}

		candidates = append(candidates, slot)
	}

	// If no candidates, return -1, -1
	if len(candidates) == 0 {
		return -1, -1
	}

	// For HP recovery, find the one with lowest threshold (use lower threshold items first)
	if slotType == SlotTypeFood || slotType == SlotTypePill {
		bestSlot := candidates[0]
		for _, slot := range candidates[1:] {
			// Prefer slots with thresholds, and among those, prefer lower thresholds
			if slot.Threshold != nil && (bestSlot.Threshold == nil || *slot.Threshold < *bestSlot.Threshold) {
				bestSlot = slot
			}
		}

		// Update slot-specific cooldown (if specified)
		if bestSlot.Cooldown != nil {
			key := fmt.Sprintf("%d:%d", bestSlot.Page, bestSlot.Slot)
			c.Status.Cooldown.Slots[key] = now.Add(time.Duration(*bestSlot.Cooldown) * time.Millisecond)
		}

		// Update global type cooldown
		if slotType == SlotTypeFood {
			c.Status.Cooldown.HPFood = now.Add(CooldownHPFood * time.Millisecond)
		} else if slotType == SlotTypePill {
			c.Status.Cooldown.HPPill = now.Add(CooldownHPPill * time.Millisecond)
		}

		// Return page if different from current, otherwise -1
		page := bestSlot.Page
		if page == c.Status.Player.CurrentPage {
			page = -1
		}
		return page, bestSlot.Slot
	}

	// For other types, just return the first available
	slot := candidates[0]

	// Update slot-specific cooldown (if specified)
	if slot.Cooldown != nil {
		key := fmt.Sprintf("%d:%d", slot.Page, slot.Slot)
		c.Status.Cooldown.Slots[key] = now.Add(time.Duration(*slot.Cooldown) * time.Millisecond)
	}

	// Update global type cooldown
	switch slotType {
	case SlotTypeAttack:
		c.Status.Cooldown.Attack = now.Add(CooldownAttack * time.Millisecond)
	case SlotTypeMPRestore:
		c.Status.Cooldown.MP = now.Add(CooldownMP * time.Millisecond)
	case SlotTypeFPRestore:
		c.Status.Cooldown.FP = now.Add(CooldownFP * time.Millisecond)
	}

	page := slot.Page
	if page == c.Status.Player.CurrentPage {
		page = -1
	}
	return page, slot.Slot
}

// UpdateTarget updates the current target information
func (c *Config) UpdateTarget(selected bool, level, hp, mp int, passive bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !selected {
		c.Status.Target = nil
		return
	}

	c.Status.Target = &TargetStatus{
		Passive: passive,
		Level:   level,
		HP:      hp,
		MP:      mp,
	}
}

// AddKilled increments kill count and updates last kill time
func (c *Config) AddKilled() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Status.Player.Killed++
	c.Status.Player.LastKilledTime = time.Now()
}

// UpdateStage updates the current stage
func (c *Config) UpdateStage(stage string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Status.Player.Stage = stage
}

// AddAction adds an action to the action history (max 10)
func (c *Config) AddAction(action string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Status.Actions = append(c.Status.Actions, action)
	if len(c.Status.Actions) > 10 {
		c.Status.Actions = c.Status.Actions[len(c.Status.Actions)-10:]
	}
}

// UpdatePlayerStats updates player HP/MP/FP
func (c *Config) UpdatePlayerStats(hp, mp, fp int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Status.Player.HP = hp
	c.Status.Player.MP = mp
	c.Status.Player.FP = fp
}

// UpdateMobs updates the detected mobs list
func (c *Config) UpdateMobs(mobs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Status.Mobs = mobs
}

// UpdateCurrentPage updates the current page
func (c *Config) UpdateCurrentPage(page int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Status.Player.CurrentPage = page
}

// Log writes a log message
func (c *Config) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Only write to log file if it's open
	if c.LogFile != nil {
		log.Println(msg)
	}

	// Also add to actions
	c.AddAction(fmt.Sprintf("log: %s", msg))
}

// BrowserLog writes a browser log message
func (c *Config) BrowserLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006/01/02 15:04:05")

	// Only write to browser log file if it's open
	if c.BrowserLogFile != nil {
		fmt.Fprintf(c.BrowserLogFile, "%s %s\n", timestamp, msg)
	}
}

// Close closes the config (saves cookies and closes log file)
func (c *Config) Close() error {
	if err := c.SaveCookies(); err != nil {
		// Only log if log file is open
		if c.LogFile != nil {
			log.Printf("Failed to save cookies: %v", err)
		}
	}

	// Close browser log file
	if c.BrowserLogFile != nil {
		c.BrowserLogFile.Close()
	}

	if c.LogFile != nil {
		return c.LogFile.Close()
	}

	return nil
}

// IsEnabled checks if the bot is enabled
func (c *Config) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Stat.Enable
}

// GetDebug checks if debug mode is enabled
func (c *Config) GetDebug() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Stat.Debug
}

// GetType gets the bot type
func (c *Config) GetType() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Stat.Type
}
