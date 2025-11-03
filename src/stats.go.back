// Package main - stats.go
//
// This file implements player and target statistics detection using pixel color matching.
// It provides HP/MP/FP bar recognition based on the Rust implementation from stats_info.rs.
//
// Key Detection Algorithms:
//
// 1. Status Bar Recognition:
//   - Uses specific color references for each bar type
//   - HP: Red colors [[174,18,55], [188,24,62], [204,30,70], [220,36,78]]
//   - MP: Blue colors [[20,84,196], [36,132,220], [44,164,228], [56,188,232]]
//   - FP: Green colors [[45,230,29], [28,172,28], [44,124,52], [20,146,20]]
//   - Scans within defined regions with tolerance matching
//   - Groups matching pixels into point cloud
//   - Calculates bounding box to get bar width
//   - Percentage = (current_width / max_width) * 100
//
// 2. Target Bar Recognition:
//   - Target HP: Same red colors as player HP, scanned in region (300,30)-(550,60)
//   - Target MP: Same blue colors as player MP, scanned in region (300,50)-(550,60)
//   - Used to detect target type (NPC vs Mover) and alive status
//
// 3. Adaptive Calibration:
//   - MaxWidth is continuously updated with largest detected value
//   - Handles UI scaling and resolution changes automatically
//
// Thread Safety:
// All status bar operations are thread-safe using sync.RWMutex.
package main

import (
	"image"
	"sync"
	"time"
)

// StatusBarKind represents the type of status bar
type StatusBarKind int

const (
	StatusBarHP StatusBarKind = iota
	StatusBarMP
	StatusBarFP
	StatusBarTargetHP
	StatusBarTargetMP
)

// String returns the string representation of StatusBarKind
func (k StatusBarKind) String() string {
	switch k {
	case StatusBarHP:
		return "HP"
	case StatusBarMP:
		return "MP"
	case StatusBarFP:
		return "FP"
	case StatusBarTargetHP:
		return "enemy HP"
	case StatusBarTargetMP:
		return "enemy MP"
	default:
		return "Unknown"
	}
}

// AliveState represents player's alive status
type AliveState int

const (
	AliveStateUnknown         AliveState = iota
	AliveStateStatsTrayClosed            // Status tray is closed
	AliveStateAlive
	AliveStateDead
)

// StatusBarConfig holds the configuration for detecting a specific status bar
type StatusBarConfig struct {
	MinX   int     // Minimum X coordinate to scan
	MinY   int     // Minimum Y coordinate to scan
	MaxX   int     // Maximum X coordinate to scan
	MaxY   int     // Maximum Y coordinate to scan
	Colors []Color // Reference colors to match
}

// NewStatusBarConfig creates a StatusBarConfig from color array
func NewStatusBarConfig(colors [][3]uint8) StatusBarConfig {
	colorList := make([]Color, len(colors))
	for i, c := range colors {
		colorList[i] = NewColor(c[0], c[1], c[2])
	}
	return StatusBarConfig{
		MinX:   105,
		MinY:   30,
		MaxX:   225,
		MaxY:   110,
		Colors: colorList,
	}
}

// GetStatusBarConfig returns the configuration for a given status bar kind
func GetStatusBarConfig(kind StatusBarKind) StatusBarConfig {
	switch kind {
	case StatusBarHP:
		return NewStatusBarConfig([][3]uint8{
			{174, 18, 55},
			{188, 24, 62},
			{204, 30, 70},
			{220, 36, 78},
		})

	case StatusBarMP:
		return NewStatusBarConfig([][3]uint8{
			{20, 84, 196},
			{36, 132, 220},
			{44, 164, 228},
			{56, 188, 232},
		})

	case StatusBarFP:
		return NewStatusBarConfig([][3]uint8{
			{45, 230, 29},
			{28, 172, 28},
			{44, 124, 52},
			{20, 146, 20},
		})

	case StatusBarTargetHP:
		config := NewStatusBarConfig([][3]uint8{
			{174, 18, 55},
			{188, 24, 62},
			{204, 30, 70},
			{220, 36, 78},
		})
		config.MinX = 300
		config.MinY = 30
		config.MaxX = 550
		config.MaxY = 60
		return config

	case StatusBarTargetMP:
		config := NewStatusBarConfig([][3]uint8{
			{20, 84, 196},
			{36, 132, 220},
			{44, 164, 228},
			{56, 188, 232},
		})
		config.MinX = 300
		config.MinY = 50
		config.MaxX = 550
		config.MaxY = 60
		return config

	default:
		return StatusBarConfig{}
	}
}

// StatInfo represents a single stat bar (HP/MP/FP or target HP/MP)
type StatInfo struct {
	MaxW           int           // Maximum width ever detected (for percentage calculation)
	Value          int           // Current percentage value (0-100)
	StatKind       StatusBarKind // Type of stat bar
	LastValue      int           // Previous value (for change detection)
	LastUpdateTime time.Time     // Time of last update
	mu             sync.RWMutex
}

// NewStatInfo creates a new StatInfo
func NewStatInfo(maxW, value int, kind StatusBarKind) *StatInfo {
	return &StatInfo{
		MaxW:           maxW,
		Value:          value,
		StatKind:       kind,
		LastValue:      100,
		LastUpdateTime: time.Now(),
	}
}

// UpdateValue updates the stat value by detecting pixels in the image
// Returns true if the value changed
func (si *StatInfo) UpdateValue(img *image.RGBA) bool {
	if img == nil {
		return false
	}

	config := GetStatusBarConfig(si.StatKind)

	// Detect pixels matching the status bar colors
	cloud := si.detectPixels(img, config)

	// Calculate bounds from point cloud
	bounds := cloud.ToBounds()

	// Update max width and calculate percentage
	si.mu.Lock()
	defer si.mu.Unlock()

	oldMaxW := si.MaxW
	oldValue := si.Value

	// Update max width if current width is larger
	if bounds.W > si.MaxW {
		si.MaxW = bounds.W
	}

	// Calculate percentage
	var newValue int
	if si.MaxW > 0 {
		valueFrac := float64(bounds.W) / float64(si.MaxW)
		newValue = int(valueFrac * 100)
		if newValue < 0 {
			newValue = 0
		}
		if newValue > 100 {
			newValue = 100
		}
	} else {
		newValue = 0
	}

	// Update value if changed
	changed := false
	if si.MaxW != oldMaxW {
		LogDebug("%s: MaxW updated %d -> %d", si.StatKind.String(), oldMaxW, si.MaxW)
		changed = true
	}

	if newValue != oldValue {
		si.Value = newValue
		si.LastUpdateTime = time.Now()
		LogDebug("%s: Value updated %d%% -> %d%%", si.StatKind.String(), oldValue, newValue)
		changed = true
	}

	return changed
}

// detectPixels scans the image for pixels matching the config colors
func (si *StatInfo) detectPixels(img *image.RGBA, config StatusBarConfig) *PointCloud {
	cloud := NewPointCloud()
	tolerance := uint8(2) // Tolerance for color matching

	bounds := img.Bounds()

	// Scan the configured region
	for y := config.MinY; y < config.MaxY && y < bounds.Max.Y; y++ {
		for x := config.MinX; x < config.MaxX && x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			// Skip fully transparent pixels
			if a>>8 != 255 {
				continue
			}

			// Convert to uint8
			pixel := Color{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
			}

			// Check if pixel matches any reference color
			for _, refColor := range config.Colors {
				if pixel.Matches(refColor, tolerance) {
					cloud.Add(Point{X: x, Y: y})
					break
				}
			}
		}
	}

	return cloud
}

// GetValue returns the current percentage value (thread-safe)
func (si *StatInfo) GetValue() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.Value
}

// ResetLastUpdateTime resets the last update time to now
func (si *StatInfo) ResetLastUpdateTime() {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.LastUpdateTime = time.Now()
}

// ClientStats holds all detected client statistics
type ClientStats struct {
	HasTrayOpen             bool
	HP                      *StatInfo
	MP                      *StatInfo
	FP                      *StatInfo
	TargetHP                *StatInfo
	TargetMP                *StatInfo
	TargetIsMover           bool
	TargetIsNPC             bool
	TargetIsAlive           bool
	TargetOnScreen          bool
	TargetMarker            *Point
	TargetDistance          int
	IsAlive                 AliveState
	StatTryNotDetectedCount int

	// Detected bar positions (for debug visualization)
	HPBar       DetectedBar
	MPBar       DetectedBar
	FPBar       DetectedBar
	TargetHPBar DetectedBar
	TargetMPBar DetectedBar

	mu sync.RWMutex
}

// DetectedBar holds information about a detected status bar (for visualization)
type DetectedBar struct {
	Bounds     Bounds
	Percentage int
	Detected   bool
}

// NewClientStats creates new client statistics
func NewClientStats() *ClientStats {
	return &ClientStats{
		HasTrayOpen:             false,
		HP:                      NewStatInfo(0, 100, StatusBarHP),
		MP:                      NewStatInfo(0, 100, StatusBarMP),
		FP:                      NewStatInfo(0, 100, StatusBarFP),
		TargetHP:                NewStatInfo(0, 0, StatusBarTargetHP),
		TargetMP:                NewStatInfo(0, 0, StatusBarTargetMP),
		IsAlive:                 AliveStateStatsTrayClosed,
		TargetIsMover:           false,
		TargetIsNPC:             false,
		TargetIsAlive:           false,
		TargetOnScreen:          false,
		StatTryNotDetectedCount: 0,
	}
}

// Update updates all bar values at once
func (cs *ClientStats) Update(img *image.RGBA) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Update all stat bars
	cs.HP.UpdateValue(img)
	cs.MP.UpdateValue(img)
	cs.FP.UpdateValue(img)
	cs.TargetHP.UpdateValue(img)
	cs.TargetMP.UpdateValue(img)

	// Detect if stat tray is open
	cs.HasTrayOpen = cs.detectStatTray()

	// Update alive state
	cs.IsAlive = cs.calculateAliveState()

	// Update target information
	hpVal := cs.TargetHP.GetValue()
	mpVal := cs.TargetMP.GetValue()
	cs.TargetIsNPC = hpVal == 100 && mpVal == 0
	cs.TargetIsMover = mpVal > 0
	cs.TargetIsAlive = hpVal > 0
}

// detectStatTray detects whether we can read stats or if the tray is closed
func (cs *ClientStats) detectStatTray() bool {
	hpVal := cs.HP.GetValue()
	mpVal := cs.MP.GetValue()
	fpVal := cs.FP.GetValue()

	// If all bars are 0, stat tray is likely closed
	if hpVal == 0 && mpVal == 0 && fpVal == 0 {
		cs.StatTryNotDetectedCount++

		// After 5 failed detections, we should try to open the tray (T key)
		// This would be handled by the behavior controller
		if cs.StatTryNotDetectedCount >= 5 {
			cs.StatTryNotDetectedCount = 0
			// Signal that tray needs to be opened
		}
		return false
	}

	cs.StatTryNotDetectedCount = 0
	return true
}

// calculateAliveState determines if the player is alive based on stats
func (cs *ClientStats) calculateAliveState() AliveState {
	if !cs.HasTrayOpen {
		return AliveStateStatsTrayClosed
	}

	hpVal := cs.HP.GetValue()
	if hpVal > 0 {
		return AliveStateAlive
	}

	return AliveStateDead
}

// GetHPPercent returns the HP percentage (thread-safe)
func (cs *ClientStats) GetHPPercent() int {
	return cs.HP.GetValue()
}

// GetMPPercent returns the MP percentage (thread-safe)
func (cs *ClientStats) GetMPPercent() int {
	return cs.MP.GetValue()
}

// GetFPPercent returns the FP percentage (thread-safe)
func (cs *ClientStats) GetFPPercent() int {
	return cs.FP.GetValue()
}

// UpdateAliveState updates the alive state (for compatibility)
func (cs *ClientStats) UpdateAliveState() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.IsAlive = cs.calculateAliveState()
}
