// Package main - analyzer.go
//
// OpenCV-based image analysis module for Flyff Universe bot.
// Uses HSV color space for more robust detection compared to RGB pixel matching.
//
// Key responsibilities:
//   - Screen capture and caching
//   - HP/MP/FP bar detection via HSV color masking
//   - Mob name detection (passive/aggressive/violet) using HSV
//   - Target marker detection (red/blue) using HSV
//   - Target distance calculation
//   - Contour-based detection for improved accuracy
//
// Detection Pipeline:
//   1. Capture screen image
//   2. Convert to HSV color space
//   3. Define ROI (Region of Interest)
//   4. Create color masks using HSV ranges
//   5. Apply morphological operations (erode, dilate)
//   6. Find contours
//   7. Filter contours by conditions (area, aspect ratio, position)
package main

import (
	"image"
	"math"
	"sync"
	"time"

	"gocv.io/x/gocv"
)

// Note: Color, Bounds, Target, MobType, and other basic types are defined in data.go

// AvoidedArea represents an area to avoid when searching for mobs
type AvoidedArea struct {
	Bounds    Bounds
	CreatedAt time.Time
	Duration  time.Duration
}

// AvoidanceList manages avoided areas
type AvoidanceList struct {
	areas []AvoidedArea
	mu    sync.RWMutex
}

// NewAvoidanceList creates a new avoidance list
func NewAvoidanceList() *AvoidanceList {
	return &AvoidanceList{
		areas: make([]AvoidedArea, 0),
	}
}

// Add adds an area to avoid
func (al *AvoidanceList) Add(bounds Bounds, duration time.Duration) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.areas = append(al.areas, AvoidedArea{
		Bounds:    bounds,
		CreatedAt: time.Now(),
		Duration:  duration,
	})
}

// IsAvoided checks if a bounds overlaps with any avoided area
func (al *AvoidanceList) IsAvoided(bounds Bounds) bool {
	al.mu.RLock()
	defer al.mu.RUnlock()

	now := time.Now()
	for _, area := range al.areas {
		if now.Sub(area.CreatedAt) > area.Duration {
			continue
		}
		if boundsOverlap(bounds, area.Bounds) {
			return true
		}
	}
	return false
}

// CleanExpired removes expired avoided areas
func (al *AvoidanceList) CleanExpired() {
	al.mu.Lock()
	defer al.mu.Unlock()

	now := time.Now()
	active := make([]AvoidedArea, 0)
	for _, area := range al.areas {
		if now.Sub(area.CreatedAt) <= area.Duration {
			active = append(active, area)
		}
	}
	al.areas = active
}

// boundsOverlap checks if two bounds overlap
func boundsOverlap(a, b Bounds) bool {
	return a.X < b.X+b.W &&
		a.X+a.W > b.X &&
		a.Y < b.Y+b.H &&
		a.Y+a.H > b.Y
}

// ROI represents a Region of Interest for image processing
type ROI struct {
	X      int
	Y      int
	Width  int
	Height int
}

// MobColorConfig holds HSV color ranges for mob detection
type MobColorConfig struct {
	PassiveMobRange    HSVRange // Yellow mob names
	AggressiveMobRange HSVRange // Red mob names
	VioletMobRange     HSVRange // Purple mob names
	RedMarkerRange     HSVRange // Red target marker
	BlueMarkerRange    HSVRange // Blue target marker
}

// GetDefaultMobColorConfig returns default HSV color ranges for mobs
// Updated with actual game screenshot values
func GetDefaultMobColorConfig() *MobColorConfig {
	return &MobColorConfig{
		// Passive Mob - Yellow names
		// Updated: H=29-31 (yellow), S=50-90, V=180-255
		PassiveMobRange: HSVRange{
			LowerH: 29, LowerS: 50, LowerV: 180,
			UpperH: 31, UpperS: 90, UpperV: 255,
		},

		// Aggressive Mob - Red names
		// Updated: H=0-5 (red), S=200-255, V=200-255
		AggressiveMobRange: HSVRange{
			LowerH: 0, LowerS: 200, LowerV: 200,
			UpperH: 5, UpperS: 255, UpperV: 255,
		},

		// Violet Mob - Purple names
		// Placeholder: H=130-160 (purple), S=100-255, V=100-255
		VioletMobRange: HSVRange{
			LowerH: 130, LowerS: 100, LowerV: 100,
			UpperH: 160, UpperS: 255, UpperV: 255,
		},

		// Red Target Marker
		// Placeholder: H=0-10 (red), S=100-255, V=200-255
		RedMarkerRange: HSVRange{
			LowerH: 0, LowerS: 100, LowerV: 200,
			UpperH: 10, UpperS: 255, UpperV: 255,
		},

		// Blue Target Marker
		// Placeholder: H=100-130 (blue), S=80-255, V=180-255
		BlueMarkerRange: HSVRange{
			LowerH: 100, LowerS: 80, LowerV: 180,
			UpperH: 130, UpperS: 255, UpperV: 255,
		},
	}
}

// Note: Point and ScreenInfo are defined in data.go

// ImageAnalyzer handles OpenCV-based image analysis
type ImageAnalyzer struct {
	browser         *Browser
	screenInfo      *ScreenInfo
	lastImage       *image.RGBA
	stats           *ClientStats
	mobColorConfig  *MobColorConfig
	mu              sync.RWMutex
}

// NewImageAnalyzer creates a new image analyzer with OpenCV support
func NewImageAnalyzer(browser *Browser) *ImageAnalyzer {
	bounds := browser.GetScreenBounds()
	return &ImageAnalyzer{
		browser:        browser,
		screenInfo:     NewScreenInfo(bounds),
		stats:          NewClientStats(),
		mobColorConfig: GetDefaultMobColorConfig(),
	}
}

// Capture captures the current screen
func (ia *ImageAnalyzer) Capture() error {
	img, err := ia.browser.Capture()
	if err != nil {
		return err
	}
	if img == nil {
		return nil
	}

	ia.mu.Lock()
	ia.lastImage = img
	ia.mu.Unlock()

	return nil
}

// GetImage returns the last captured image
func (ia *ImageAnalyzer) GetImage() *image.RGBA {
	ia.mu.RLock()
	defer ia.mu.RUnlock()
	return ia.lastImage
}

// GetStats returns current client stats
func (ia *ImageAnalyzer) GetStats() *ClientStats {
	return ia.stats
}

// UpdateStats updates all client stats from the current image using OpenCV
func (ia *ImageAnalyzer) UpdateStats() {
	img := ia.GetImage()
	if img == nil {
		return
	}

	// Convert image.RGBA to gocv.Mat
	mat := ia.imageToMat(img)
	if mat.Empty() {
		return
	}
	defer mat.Close()

	// Convert to HSV color space
	hsvMat := gocv.NewMat()
	defer hsvMat.Close()
	gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)

	// Update HP/MP/FP bars using OpenCV HSV detection
	ia.stats.UpdateOpenCV(&hsvMat)

	// Update target marker (stored as a flag)
	if ia.DetectTargetMarkerOpenCV(&hsvMat) {
		ia.stats.TargetOnScreen = true
	} else {
		ia.stats.TargetOnScreen = false
	}
}

// IdentifyMobs identifies all mobs in the current image using OpenCV HSV detection
func (ia *ImageAnalyzer) IdentifyMobs(config *Config) []Target {
	img := ia.GetImage()
	if img == nil {
		return nil
	}

	// Convert to Mat
	mat := ia.imageToMat(img)
	if mat.Empty() {
		return nil
	}
	defer mat.Close()

	// Convert to HSV color space
	hsvMat := gocv.NewMat()
	defer hsvMat.Close()
	gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)

	// Define search region (avoid bottom UI elements)
	searchROI := ROI{
		X:      0,
		Y:      0,
		Width:  ia.screenInfo.Width,
		Height: ia.screenInfo.Height - 100,
	}

	var mobs []Target

	// Detect passive mobs (yellow names) using HSV
	passiveBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.PassiveMobRange, config)
	LogDebug("Found %d passive mob candidates", len(passiveBounds))
	for _, bounds := range passiveBounds {
		// Filter by position (avoid HP bar region at top-left)
		if bounds.Y >= 110 {
			mobs = append(mobs, Target{
				Type:   MobPassive,
				Bounds: bounds,
			})
			LogDebug("Passive mob ACCEPTED at (%d,%d) size %dx%d", bounds.X, bounds.Y, bounds.W, bounds.H)
		} else {
			LogDebug("Passive mob REJECTED at (%d,%d) - too high (y < 110)", bounds.X, bounds.Y)
		}
	}

	// Detect aggressive mobs (red names) using HSV
	aggressiveBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.AggressiveMobRange, config)
	LogDebug("Found %d aggressive mob candidates", len(aggressiveBounds))
	for _, bounds := range aggressiveBounds {
		// Filter by position (avoid HP bar region at top-left)
		if bounds.Y >= 110 {
			mobs = append(mobs, Target{
				Type:   MobAggressive,
				Bounds: bounds,
			})
			LogDebug("Aggressive mob ACCEPTED at (%d,%d) size %dx%d", bounds.X, bounds.Y, bounds.W, bounds.H)
		} else {
			LogDebug("Aggressive mob REJECTED at (%d,%d) - too high (y < 110)", bounds.X, bounds.Y)
		}
	}

	// Detect violet mobs (for logging, filtered out)
	violetBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.VioletMobRange, config)
	if len(violetBounds) > 0 {
		LogDebug("Detected %d violet mobs (filtered out)", len(violetBounds))
	}

	LogDebug("Identified %d total mobs (passive: %d, aggressive: %d)",
		len(mobs), len(passiveBounds), len(aggressiveBounds))

	return mobs
}

// detectMobsByHSV detects mobs using HSV color masking and contour detection
func (ia *ImageAnalyzer) detectMobsByHSV(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange, config *Config) []Bounds {
	// Ensure ROI is within image bounds
	if roi.X < 0 || roi.Y < 0 ||
	   roi.X+roi.Width > hsvMat.Cols() || roi.Y+roi.Height > hsvMat.Rows() {
		return nil
	}

	// Extract ROI
	roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
	defer roiMat.Close()

	// Create HSV color mask
	mask := ia.createHSVMask(&roiMat, colorRange)
	defer mask.Close()

	// Apply morphological operations to reduce noise
	morphed := ia.applyMorphology(&mask)
	defer morphed.Close()

	// Find contours
	contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	// Convert contours to bounds and filter by mob name width constraints
	var bounds []Bounds
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		rect := gocv.BoundingRect(contour)

		// Filter by width (mob name width constraints)
		if rect.Dx() > config.MinMobNameWidth && rect.Dx() < config.MaxMobNameWidth {
			// Convert back to screen coordinates
			screenBounds := Bounds{
				X: roi.X + rect.Min.X,
				Y: roi.Y + rect.Min.Y,
				W: rect.Dx(),
				H: rect.Dy(),
			}

			// Skip HP bar region (top-left corner)
			if screenBounds.X <= 250 && screenBounds.Y <= 110 {
				continue
			}

			bounds = append(bounds, screenBounds)
		}
	}

	return bounds
}

// DetectTargetMarkerOpenCV detects the target marker using OpenCV HSV detection
func (ia *ImageAnalyzer) DetectTargetMarkerOpenCV(hsvMat *gocv.Mat) bool {
	// Search in upper-middle area of screen where target markers appear
	markerROI := ROI{
		X:      ia.screenInfo.Width / 4,
		Y:      ia.screenInfo.Height / 6,
		Width:  ia.screenInfo.Width / 2,
		Height: ia.screenInfo.Height / 3,
	}

	// Try blue marker first (for Azria and other zones)
	blueMarkerDetected := ia.detectMarker(hsvMat, markerROI, ia.mobColorConfig.BlueMarkerRange)
	if blueMarkerDetected {
		LogDebug("Blue target marker detected")
		return true
	}

	// Try red marker (normal zones)
	redMarkerDetected := ia.detectMarker(hsvMat, markerROI, ia.mobColorConfig.RedMarkerRange)
	if redMarkerDetected {
		LogDebug("Red target marker detected")
		return true
	}

	return false
}

// DetectTargetMarker detects the target marker (wrapper for compatibility)
func (ia *ImageAnalyzer) DetectTargetMarker() bool {
	img := ia.GetImage()
	if img == nil {
		return false
	}

	mat := ia.imageToMat(img)
	if mat.Empty() {
		return false
	}
	defer mat.Close()

	hsvMat := gocv.NewMat()
	defer hsvMat.Close()
	gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)

	return ia.DetectTargetMarkerOpenCV(&hsvMat)
}

// detectMarker detects a marker using HSV color masking
func (ia *ImageAnalyzer) detectMarker(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange) bool {
	// Ensure ROI is within image bounds
	if roi.X < 0 || roi.Y < 0 ||
	   roi.X+roi.Width > hsvMat.Cols() || roi.Y+roi.Height > hsvMat.Rows() {
		return false
	}

	// Extract ROI
	roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
	defer roiMat.Close()

	// Create HSV color mask
	mask := ia.createHSVMask(&roiMat, colorRange)
	defer mask.Close()

	// Count non-zero pixels in the mask
	nonZero := gocv.CountNonZero(mask)

	// Threshold: need at least 20 pixels to consider marker detected
	return nonZero > 20
}

// DetectTargetDistance calculates distance to target marker using OpenCV
func (ia *ImageAnalyzer) DetectTargetDistance() int {
	img := ia.GetImage()
	if img == nil {
		return 9999
	}

	mat := ia.imageToMat(img)
	if mat.Empty() {
		return 9999
	}
	defer mat.Close()

	// Convert to HSV
	hsvMat := gocv.NewMat()
	defer hsvMat.Close()
	gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)

	// Search region for target marker
	markerROI := ROI{
		X:      ia.screenInfo.Width / 4,
		Y:      ia.screenInfo.Height / 6,
		Width:  ia.screenInfo.Width / 2,
		Height: ia.screenInfo.Height / 3,
	}

	// Try to find marker center
	markerCenter := ia.findMarkerCenter(&hsvMat, markerROI)
	if markerCenter == nil {
		return 9999
	}

	// Calculate distance from screen center
	centerX := ia.screenInfo.Width / 2
	centerY := ia.screenInfo.Height / 2

	dx := float64(markerCenter.X - centerX)
	dy := float64(markerCenter.Y - centerY)
	distance := int(math.Sqrt(dx*dx + dy*dy))

	return distance
}

// findMarkerCenter finds the center of the target marker
func (ia *ImageAnalyzer) findMarkerCenter(hsvMat *gocv.Mat, roi ROI) *Point {
	// Try blue marker first
	blueCenter := ia.findMarkerCenterByColor(hsvMat, roi, ia.mobColorConfig.BlueMarkerRange)
	if blueCenter != nil {
		return blueCenter
	}

	// Try red marker
	redCenter := ia.findMarkerCenterByColor(hsvMat, roi, ia.mobColorConfig.RedMarkerRange)
	return redCenter
}

// findMarkerCenterByColor finds marker center for a specific color using contours
func (ia *ImageAnalyzer) findMarkerCenterByColor(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange) *Point {
	// Ensure ROI is within image bounds
	if roi.X < 0 || roi.Y < 0 ||
	   roi.X+roi.Width > hsvMat.Cols() || roi.Y+roi.Height > hsvMat.Rows() {
		return nil
	}

	// Extract ROI
	roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
	defer roiMat.Close()

	// Create mask
	mask := ia.createHSVMask(&roiMat, colorRange)
	defer mask.Close()

	// Find contours
	contours := gocv.FindContours(mask, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	if contours.Size() == 0 {
		return nil
	}

	// Find largest contour (main marker shape)
	maxArea := 0.0
	var maxRect image.Rectangle
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		area := gocv.ContourArea(contour)
		if area > maxArea {
			maxArea = area
			maxRect = gocv.BoundingRect(contour)
		}
	}

	// Calculate center (convert back to screen coordinates)
	centerX := roi.X + maxRect.Min.X + maxRect.Dx()/2
	centerY := roi.Y + maxRect.Min.Y + maxRect.Dy()/2

	return &Point{X: centerX, Y: centerY}
}

// FindClosestMob finds the closest mob to the screen center
func (ia *ImageAnalyzer) FindClosestMob(mobs []Target) *Target {
	if len(mobs) == 0 {
		return nil
	}

	centerX := ia.screenInfo.Width / 2
	centerY := ia.screenInfo.Height / 2

	var closest *Target
	minDistance := float64(99999)

	// Maximum distance threshold matching Rust version
	// 325px for normal farming, can be increased for circle pattern
	maxDistance := 325.0

	for i := range mobs {
		mobX := mobs[i].Bounds.X + mobs[i].Bounds.W/2
		mobY := mobs[i].Bounds.Y + mobs[i].Bounds.H/2

		dx := float64(mobX - centerX)
		dy := float64(mobY - centerY)
		distance := math.Sqrt(dx*dx + dy*dy)

		// Filter by max distance to avoid unreachable mobs
		if distance > maxDistance {
			continue
		}

		if distance < minDistance {
			minDistance = distance
			closest = &mobs[i]
		}
	}

	return closest
}

// DetectTargetHP detects the target's HP value (returns 0-100)
func (ia *ImageAnalyzer) DetectTargetHP() int {
	return ia.stats.TargetHP.Value
}

// Helper methods

// imageToMat converts image.RGBA to gocv.Mat (BGR format for OpenCV)
func (ia *ImageAnalyzer) imageToMat(img *image.RGBA) gocv.Mat {
	if img == nil {
		return gocv.NewMat()
	}

	// Convert RGBA to BGR for OpenCV
	mat, err := gocv.ImageToMatRGB(img)
	if err != nil {
		LogError("Failed to convert image to mat: %v", err)
		return gocv.NewMat()
	}

	return mat
}

// createHSVMask creates a binary mask based on HSV color range
func (ia *ImageAnalyzer) createHSVMask(hsvMat *gocv.Mat, colorRange HSVRange) gocv.Mat {
	// Create lower and upper bound scalars for HSV range
	lower := gocv.NewScalar(float64(colorRange.LowerH), float64(colorRange.LowerS), float64(colorRange.LowerV), 0)
	upper := gocv.NewScalar(float64(colorRange.UpperH), float64(colorRange.UpperS), float64(colorRange.UpperV), 0)

	// Create mask using inRange operation
	mask := gocv.NewMat()
	gocv.InRangeWithScalar(*hsvMat, lower, upper, &mask)

	return mask
}

// applyMorphology applies morphological operations to reduce noise
func (ia *ImageAnalyzer) applyMorphology(mask *gocv.Mat) gocv.Mat {
	// Create structuring element (kernel) for morphological operations
	// closesize = 10 for mob detection
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(10, 10))
	defer kernel.Close()

	// Apply morphological closing (dilation followed by erosion)
	// closeiter = 5 iterations for mob detection
	// This fills small holes and connects nearby regions
	result := mask.Clone()
	for i := 0; i < 5; i++ {
		temp := gocv.NewMat()
		gocv.Dilate(result, &temp, kernel)
		gocv.Erode(temp, &result, kernel)
		temp.Close()
	}

	return result
}
