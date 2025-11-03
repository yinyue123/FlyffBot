// Package main - stats.go
//
// This file implements player and target statistics detection using OpenCV HSV color matching.
// Uses HSV color space for more robust detection compared to RGB pixel matching.
//
// Key Detection Algorithms:
//
// 1. Status Bar Recognition (OpenCV HSV):
//   - Define ROI (Region of Interest) for each bar type
//   - Convert ROI to HSV color space
//   - Create color masks using HSV ranges for HP/MP/FP bars
//   - Apply morphological operations (erode, dilate) to reduce noise
//   - Find contours in the processed mask
//   - Calculate bounding box from largest contour
//   - Percentage = (contour_width / roi_width) * 100
//
// 2. Target Bar Recognition:
//   - Same HSV approach, but scanned in target region (300,30)-(550,60)
//   - Used to detect target type (NPC vs Mover) and alive status
//
// 3. HSV Color Ranges:
//   - HP: Red colors (H=0-10, S=100-255, V=100-255) - PLACEHOLDER
//   - MP: Blue colors (H=100-130, S=100-255, V=150-255) - PLACEHOLDER
//   - FP: Green colors (H=40-80, S=100-255, V=100-255) - PLACEHOLDER
//
// Thread Safety:
// All status bar operations are thread-safe using sync.RWMutex.
package main

import (
	"image"
	"sync"
	"time"

	"gocv.io/x/gocv"
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

// HSVRange represents a color range in HSV space
type HSVRange struct {
	LowerH uint8 // Hue lower bound (0-180 in OpenCV)
	LowerS uint8 // Saturation lower bound (0-255)
	LowerV uint8 // Value lower bound (0-255)
	UpperH uint8 // Hue upper bound (0-180 in OpenCV)
	UpperS uint8 // Saturation upper bound (0-255)
	UpperV uint8 // Value upper bound (0-255)
}

// StatusBarConfig holds the configuration for detecting a specific status bar using HSV
type StatusBarConfig struct {
	MinX      int      // Minimum X coordinate for ROI
	MinY      int      // Minimum Y coordinate for ROI
	MaxX      int      // Maximum X coordinate for ROI
	MaxY      int      // Maximum Y coordinate for ROI
	HSVRange  HSVRange // HSV color range to match
}

// GetStatusBarConfig returns the HSV-based configuration for a given status bar kind
func GetStatusBarConfig(kind StatusBarKind) StatusBarConfig {
	switch kind {
	case StatusBarHP:
		// HP Bar - Red colors
		// Updated HSV values from actual game screenshots
		return StatusBarConfig{
			MinX: 105,
			MinY: 30,
			MaxX: 225,
			MaxY: 110,
			HSVRange: HSVRange{
				LowerH: 170, // Red hue lower
				LowerS: 120, // Saturation lower
				LowerV: 150, // Value lower
				UpperH: 175, // Red hue upper
				UpperS: 200, // Saturation upper
				UpperV: 230, // Value upper
			},
		}

	case StatusBarMP:
		// MP Bar - Blue colors
		// Updated HSV values from actual game screenshots
		return StatusBarConfig{
			MinX: 105,
			MinY: 30,
			MaxX: 225,
			MaxY: 110,
			HSVRange: HSVRange{
				LowerH: 99,  // Blue hue lower
				LowerS: 114, // Saturation lower
				LowerV: 190, // Value lower
				UpperH: 117, // Blue hue upper
				UpperS: 200, // Saturation upper
				UpperV: 240, // Value upper
			},
		}

	case StatusBarFP:
		// FP Bar - Green colors
		// Updated HSV values from actual game screenshots
		return StatusBarConfig{
			MinX: 105,
			MinY: 30,
			MaxX: 225,
			MaxY: 110,
			HSVRange: HSVRange{
				LowerH: 52,  // Green hue lower
				LowerS: 150, // Saturation lower
				LowerV: 150, // Value lower
				UpperH: 60,  // Green hue upper
				UpperS: 173, // Saturation upper
				UpperV: 230, // Value upper
			},
		}

	case StatusBarTargetHP:
		// Target HP Bar - Same red colors as HP
		// Updated HSV values from actual game screenshots
		return StatusBarConfig{
			MinX: 300,
			MinY: 30,
			MaxX: 550,
			MaxY: 60,
			HSVRange: HSVRange{
				LowerH: 170, // Red hue lower
				LowerS: 120, // Saturation lower
				LowerV: 150, // Value lower
				UpperH: 175, // Red hue upper
				UpperS: 200, // Saturation upper
				UpperV: 230, // Value upper
			},
		}

	case StatusBarTargetMP:
		// Target MP Bar - Same blue colors as MP
		// Updated HSV values from actual game screenshots
		return StatusBarConfig{
			MinX: 300,
			MinY: 50,
			MaxX: 550,
			MaxY: 60,
			HSVRange: HSVRange{
				LowerH: 99,  // Blue hue lower
				LowerS: 114, // Saturation lower
				LowerV: 190, // Value lower
				UpperH: 117, // Blue hue upper
				UpperS: 200, // Saturation upper
				UpperV: 240, // Value upper
			},
		}

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

// UpdateValueOpenCV updates the stat value by detecting pixels using OpenCV HSV
// Returns true if the value changed
func (si *StatInfo) UpdateValueOpenCV(hsvMat *gocv.Mat) bool {
	if hsvMat == nil || hsvMat.Empty() {
		return false
	}

	config := GetStatusBarConfig(si.StatKind)

	// Extract ROI
	roiWidth := config.MaxX - config.MinX
	roiHeight := config.MaxY - config.MinY

	// Ensure ROI is within image bounds
	if config.MinX < 0 || config.MinY < 0 ||
	   config.MaxX > hsvMat.Cols() || config.MaxY > hsvMat.Rows() {
		return false
	}

	roiMat := hsvMat.Region(image.Rect(config.MinX, config.MinY, config.MaxX, config.MaxY))
	defer roiMat.Close()

	// Create HSV color mask
	mask := si.createHSVMask(&roiMat, config.HSVRange)
	defer mask.Close()

	// Apply morphological operations to reduce noise
	morphed := si.applyMorphology(&mask)
	defer morphed.Close()

	// Find contours
	contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	// Find the largest contour width (main bar)
	maxWidth := 0
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		rect := gocv.BoundingRect(contour)
		if rect.Dx() > maxWidth {
			maxWidth = rect.Dx()
		}
	}

	// Update max width and calculate percentage
	si.mu.Lock()
	defer si.mu.Unlock()

	oldMaxW := si.MaxW
	oldValue := si.Value

	// Update max width if current width is larger
	if maxWidth > si.MaxW {
		si.MaxW = maxWidth
	}

	// Calculate percentage based on ROI width
	var newValue int
	if roiWidth > 0 {
		valueFrac := float64(maxWidth) / float64(roiWidth)
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

// createHSVMask creates a binary mask based on HSV color range
func (si *StatInfo) createHSVMask(hsvMat *gocv.Mat, colorRange HSVRange) gocv.Mat {
	// Create lower and upper bound scalars
	lower := gocv.NewScalar(float64(colorRange.LowerH), float64(colorRange.LowerS), float64(colorRange.LowerV), 0)
	upper := gocv.NewScalar(float64(colorRange.UpperH), float64(colorRange.UpperS), float64(colorRange.UpperV), 0)

	// Create mask using inRange
	mask := gocv.NewMat()
	gocv.InRangeWithScalar(*hsvMat, lower, upper, &mask)

	return mask
}

// applyMorphology applies morphological operations to reduce noise
func (si *StatInfo) applyMorphology(mask *gocv.Mat) gocv.Mat {
	// Create structuring element (kernel) for morphological operations
	// closesize = 25 for status bars
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(25, 25))
	defer kernel.Close()

	// Apply morphological closing (dilation followed by erosion)
	// closeiter = 3 iterations for status bars
	// This fills small holes in the bar
	result := mask.Clone()
	for i := 0; i < 3; i++ {
		temp := gocv.NewMat()
		gocv.Dilate(result, &temp, kernel)
		gocv.Erode(temp, &result, kernel)
		temp.Close()
	}

	return result
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

// UpdateOpenCV updates all bar values using OpenCV HSV detection
func (cs *ClientStats) UpdateOpenCV(hsvMat *gocv.Mat) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Update all stat bars using OpenCV
	cs.HP.UpdateValueOpenCV(hsvMat)
	cs.MP.UpdateValueOpenCV(hsvMat)
	cs.FP.UpdateValueOpenCV(hsvMat)
	cs.TargetHP.UpdateValueOpenCV(hsvMat)
	cs.TargetMP.UpdateValueOpenCV(hsvMat)

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
