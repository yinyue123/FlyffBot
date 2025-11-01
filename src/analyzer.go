// Package main - analyzer.go
//
// Image analysis module for Flyff Universe bot.
// Handles screen capture, pixel detection, and game state recognition.
//
// Key responsibilities:
//   - Screen capture and caching
//   - HP/MP/FP bar detection via color matching
//   - Mob name detection (passive/aggressive/violet)
//   - Target marker detection (red/blue)
//   - Target distance calculation
//   - Parallel pixel scanning for performance
package main

import (
	"image"
	"image/color"
	"math"
	"sync"
	"time"
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

// Note: Point and ScreenInfo are defined in data.go

// ImageAnalyzer handles image analysis
type ImageAnalyzer struct {
	browser    *Browser
	screenInfo *ScreenInfo
	lastImage  *image.RGBA
	stats      *ClientStats
	mu         sync.RWMutex
}

// NewImageAnalyzer creates a new image analyzer
func NewImageAnalyzer(browser *Browser) *ImageAnalyzer {
	bounds := browser.GetScreenBounds()
	return &ImageAnalyzer{
		browser:    browser,
		screenInfo: NewScreenInfo(bounds),
		stats:      NewClientStats(),
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

// UpdateStats updates all client stats from the current image
func (ia *ImageAnalyzer) UpdateStats() {
	img := ia.GetImage()
	if img == nil {
		return
	}

	// Update HP/MP/FP bars
	ia.updateStatusBars(img)

	// Update target marker (stored as a Point, nil if not detected)
	if ia.DetectTargetMarker() {
		// Target marker is on screen - we don't store exact position, just the flag
		ia.stats.TargetOnScreen = true
	} else {
		ia.stats.TargetOnScreen = false
	}

	// Update target HP
	ia.updateTargetHP(img)
}

// updateStatusBars updates HP/MP/FP status bars
func (ia *ImageAnalyzer) updateStatusBars(img *image.RGBA) {
	// Update HP/MP/FP using image-based detection
	// StatInfo.UpdateValue() handles the scanning internally
	ia.stats.HP.UpdateValue(img)
	ia.stats.MP.UpdateValue(img)
	ia.stats.FP.UpdateValue(img)
}

// updateTargetHP updates target HP
func (ia *ImageAnalyzer) updateTargetHP(img *image.RGBA) {
	// Update target HP using image-based detection
	// StatInfo.UpdateValue() handles the scanning internally
	changed := ia.stats.TargetHP.UpdateValue(img)
	if changed {
		ia.stats.TargetIsAlive = ia.stats.TargetHP.Value > 0
	}
}

// IdentifyMobs identifies all mobs in the current image
func (ia *ImageAnalyzer) IdentifyMobs(config *Config) []Target {
	img := ia.GetImage()
	if img == nil {
		return nil
	}

	// Scan region - expanded to catch more mobs, filter in post-processing
	// Changed from Y:60 to Y:0 to match Rust version's full-screen scan
	region := Bounds{
		X: 0,
		Y: 0, // Start from top (was 60)
		W: ia.screenInfo.Width,
		H: ia.screenInfo.Height - 100, // Reduced from 170 to 100
	}

	// Detect passive mobs (yellow names)
	passiveColors := []Color{config.PassiveColor}
	passivePoints := ia.scanPixelsForColors(img, region, passiveColors, config.PassiveTolerance)

	// Detect aggressive mobs (red names)
	aggressiveColors := []Color{config.AggressiveColor}
	aggressivePoints := ia.scanPixelsForColors(img, region, aggressiveColors, config.AggressiveTolerance)

	// Detect violet mobs (purple names)
	violetColors := []Color{config.VioletColor}
	violetPoints := ia.scanPixelsForColors(img, region, violetColors, config.VioletTolerance)

	LogDebug("Found %d passive points, %d aggressive points, %d violet points",
		len(passivePoints), len(aggressivePoints), len(violetPoints))

	// Cluster points into mobs
	var mobs []Target

	// Process passive mobs
	passiveClusters := clusterPoints(passivePoints, 50, 3)
	LogDebug("Passive clustering: %d points -> %d clusters", len(passivePoints), len(passiveClusters))
	for _, bounds := range passiveClusters {
		// Filter: width check + avoid HP bar region (y < 110)
		// Matching Rust logic (image_analyzer.rs:164-166): w > min && w < max
		if bounds.W > config.MinMobNameWidth && bounds.W < config.MaxMobNameWidth && bounds.Y >= 110 {
			LogDebug("Passive mob ACCEPTED at (%d,%d) size %dx%d", bounds.X, bounds.Y, bounds.W, bounds.H)
			mobs = append(mobs, Target{
				Type:   MobPassive,
				Bounds: bounds,
			})
		} else {
			LogDebug("Passive cluster REJECTED at (%d,%d) size %dx%d (width must be >%d and <%d, y: %d)",
				bounds.X, bounds.Y, bounds.W, bounds.H, config.MinMobNameWidth, config.MaxMobNameWidth, bounds.Y)
		}
	}

	// Process aggressive mobs
	aggressiveClusters := clusterPoints(aggressivePoints, 50, 3)
	LogDebug("Aggressive clustering: %d points -> %d clusters", len(aggressivePoints), len(aggressiveClusters))
	for _, bounds := range aggressiveClusters {
		// Filter: width check + avoid HP bar region (y < 110)
		// Matching Rust logic (image_analyzer.rs:164-166): w > min && w < max
		if bounds.W > config.MinMobNameWidth && bounds.W < config.MaxMobNameWidth && bounds.Y >= 110 {
			LogDebug("Aggressive mob ACCEPTED at (%d,%d) size %dx%d", bounds.X, bounds.Y, bounds.W, bounds.H)
			mobs = append(mobs, Target{
				Type:   MobAggressive,
				Bounds: bounds,
			})
		} else {
			LogDebug("Aggressive cluster REJECTED at (%d,%d) size %dx%d (width must be >%d and <%d, y: %d)",
				bounds.X, bounds.Y, bounds.W, bounds.H, config.MinMobNameWidth, config.MaxMobNameWidth, bounds.Y)
		}
	}

	// Violet mobs are detected but filtered out
	if len(violetPoints) > 0 {
		violetClusters := clusterPoints(violetPoints, 50, 3)
		for _, bounds := range violetClusters {
			// Matching Rust logic: w > min && w < max
			if bounds.W > config.MinMobNameWidth && bounds.W < config.MaxMobNameWidth {
				LogDebug("Detected violet mob at (%d,%d), filtering out", bounds.X, bounds.Y)
			}
		}
	}

	LogDebug("Identified %d total mobs (passive clusters: %d, aggressive clusters: %d)",
		len(mobs), len(passiveClusters), len(aggressiveClusters))

	return mobs
}

// DetectTargetMarker detects the target marker above selected target
func (ia *ImageAnalyzer) DetectTargetMarker() bool {
	img := ia.GetImage()
	if img == nil {
		return false
	}

	// Search in upper-middle area of screen
	region := Bounds{
		X: ia.screenInfo.Width / 4,
		Y: ia.screenInfo.Height / 6,
		W: ia.screenInfo.Width / 2,
		H: ia.screenInfo.Height / 3,
	}

	// Try blue marker first (for Azria and other zones)
	blueMarkerColors := []Color{
		NewColor(131, 148, 205),
	}
	bluePoints := ia.scanPixelsForColors(img, region, blueMarkerColors, 5)

	if len(bluePoints) > 20 {
		LogDebug("Blue target marker detected (%d points)", len(bluePoints))
		return true
	}

	// Fallback to red marker (normal zones)
	redMarkerColors := []Color{
		NewColor(246, 90, 106),
	}
	redPoints := ia.scanPixelsForColors(img, region, redMarkerColors, 5)

	if len(redPoints) > 20 {
		LogDebug("Red target marker detected (%d points)", len(redPoints))
		return true
	}

	return false
}

// DetectTargetDistance calculates distance to target marker
func (ia *ImageAnalyzer) DetectTargetDistance() int {
	img := ia.GetImage()
	if img == nil {
		return 9999
	}

	// Search for target marker
	region := Bounds{
		X: ia.screenInfo.Width / 4,
		Y: ia.screenInfo.Height / 6,
		W: ia.screenInfo.Width / 2,
		H: ia.screenInfo.Height / 3,
	}

	// Try both colors
	bluePoints := ia.scanPixelsForColors(img, region, []Color{NewColor(131, 148, 205)}, 5)
	redPoints := ia.scanPixelsForColors(img, region, []Color{NewColor(246, 90, 106)}, 5)

	var markerPoints []Point
	if len(bluePoints) > len(redPoints) {
		markerPoints = bluePoints
	} else {
		markerPoints = redPoints
	}

	if len(markerPoints) == 0 {
		return 9999
	}

	// Calculate center of marker
	bounds := pointsToBounds(markerPoints)
	markerX := bounds.X + bounds.W/2
	markerY := bounds.Y + bounds.H/2

	// Calculate distance from screen center
	centerX := ia.screenInfo.Width / 2
	centerY := ia.screenInfo.Height / 2

	dx := float64(markerX - centerX)
	dy := float64(markerY - centerY)
	distance := int(math.Sqrt(dx*dx + dy*dy))

	return distance
}

// scanPixelsForColors scans a region for pixels matching any of the given colors
func (ia *ImageAnalyzer) scanPixelsForColors(img *image.RGBA, region Bounds, colors []Color, tolerance uint8) []Point {
	var points []Point

	bounds := img.Bounds()
	minX := max(region.X, bounds.Min.X)
	minY := max(region.Y, bounds.Min.Y)
	maxX := min(region.X+region.W, bounds.Max.X)
	maxY := min(region.Y+region.H, bounds.Max.Y)

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			// Skip HP bar region (matching Rust logic at line 231-233)
			if x <= 250 && y <= 110 {
				continue
			}

			c := img.RGBAAt(x, y)

			// Check if pixel matches any target color
			for _, targetColor := range colors {
				if colorMatches(c, targetColor, tolerance) {
					points = append(points, Point{X: x, Y: y})
					break
				}
			}
		}
	}

	return points
}

// colorMatches checks if a color matches a target color within tolerance
func colorMatches(c color.RGBA, target Color, tolerance uint8) bool {
	// Allow pixels with alpha >= 250 to handle anti-aliasing and semi-transparent text
	// This matches the Rust version which doesn't check alpha at all
	if c.A < 250 {
		return false
	}

	rDiff := abs(int(c.R) - int(target.R))
	gDiff := abs(int(c.G) - int(target.G))
	bDiff := abs(int(c.B) - int(target.B))

	return rDiff <= int(tolerance) && gDiff <= int(tolerance) && bDiff <= int(tolerance)
}

// clusterPoints clusters nearby points into bounding boxes
func clusterPoints(points []Point, distanceX, distanceY int) []Bounds {
	if len(points) == 0 {
		return nil
	}

	// CRITICAL: Sort points by X axis first (matching Rust's sorted_by at point_cloud.rs:78)
	// Without sorting, clustering will not work correctly!
	sortedPoints := make([]Point, len(points))
	copy(sortedPoints, points)

	// Sort by X coordinate
	for i := 0; i < len(sortedPoints); i++ {
		for j := i + 1; j < len(sortedPoints); j++ {
			if sortedPoints[i].X > sortedPoints[j].X {
				sortedPoints[i], sortedPoints[j] = sortedPoints[j], sortedPoints[i]
			}
		}
	}

	// First cluster by X axis
	xClusters := make([][]Point, 0)
	currentCluster := []Point{sortedPoints[0]}

	for i := 1; i < len(sortedPoints); i++ {
		if abs(sortedPoints[i].X-sortedPoints[i-1].X) <= distanceX {
			currentCluster = append(currentCluster, sortedPoints[i])
		} else {
			xClusters = append(xClusters, currentCluster)
			currentCluster = []Point{sortedPoints[i]}
		}
	}
	xClusters = append(xClusters, currentCluster)

	// Then cluster each X cluster by Y axis
	var bounds []Bounds
	for _, xCluster := range xClusters {
		// Sort by Y
		for i := 0; i < len(xCluster); i++ {
			for j := i + 1; j < len(xCluster); j++ {
				if xCluster[i].Y > xCluster[j].Y {
					xCluster[i], xCluster[j] = xCluster[j], xCluster[i]
				}
			}
		}

		// Cluster by Y distance
		yCluster := []Point{xCluster[0]}
		for i := 1; i < len(xCluster); i++ {
			if abs(xCluster[i].Y-xCluster[i-1].Y) <= distanceY {
				yCluster = append(yCluster, xCluster[i])
			} else {
				bounds = append(bounds, pointsToBounds(yCluster))
				yCluster = []Point{xCluster[i]}
			}
		}
		bounds = append(bounds, pointsToBounds(yCluster))
	}

	return bounds
}

// Note: pointsToBounds is defined in data.go

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

// getStatusBarColors returns the color array for a status bar type
func getStatusBarColors(kind StatusBarKind) []Color {
	switch kind {
	case StatusBarHP:
		return []Color{
			NewColor(174, 18, 55),
			NewColor(188, 24, 62),
			NewColor(204, 30, 70),
			NewColor(220, 36, 78),
		}
	case StatusBarMP:
		return []Color{
			NewColor(20, 84, 196),
			NewColor(36, 132, 220),
			NewColor(44, 164, 228),
			NewColor(56, 188, 232),
		}
	case StatusBarFP:
		return []Color{
			NewColor(45, 230, 29),
			NewColor(28, 172, 28),
			NewColor(44, 124, 52),
			NewColor(20, 146, 20),
		}
	default:
		return nil
	}
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
