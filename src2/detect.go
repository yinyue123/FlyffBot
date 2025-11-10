// Package main - detect.go
//
// This file implements player, target, and mobs detection using OpenCV HSV color matching.
package main

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

// ROIArea defines the region of interest for detection
type ROIArea struct {
	MinX int
	MaxX int
	MinY int
	MaxY int
}

// BarKind represents the type of status bar
const (
	BarKindUnused int = iota
	BarKindHP
	BarKindMP
	BarKindFP
	BarKindTargetHP
	BarKindTargetMP
)

// BarInfo represents information about a single status bar
type BarInfo struct {
	BarKind  int // Bar type (HP/MP/FP/TargetHP/TargetMP)
	MinH     int // Hue minimum
	MaxH     int // Hue maximum
	MinS     int // Saturation minimum
	MaxS     int // Saturation maximum
	MinV     int // Value minimum
	MaxV     int // Value maximum
	Value    int // Current percentage (width / maxWidth * 100)
	Width    int // Detected width
	MaxCount int // Counter for stable width (if width doesn't change for 30 times, update maxWidth)
	MaxWidth int // Maximum width (initially 0)
}

// Filter defines filtering constraints for detection
type Filter struct {
	MinWidth   int             // Minimum width for filtering
	MaxWidth   int             // Maximum width for filtering
	MinHeight  int             // Minimum height for filtering
	MaxHeight  int             // Maximum height for filtering
	MorphShape gocv.MorphShape // Morphology shape (default: MorphRect)
	MorphPoint image.Point     // Morphology kernel size (default: image.Pt(5, 5))
	MorphIter  int             // Morphology iterations (default: 3)
}

// StatsBar represents a group of status bars (HP/MP/FP)
type StatsBar struct {
	Open      bool    // Whether the stats are visible
	OpenCount int     // Counter for closed stats (if hp/fp/mp all 0 for 5+ times, not open)
	Alive     bool    // Whether alive (hp > 0)
	NPC       bool    // Whether NPC (hp=100, mp=0, fp not active)
	ROI       ROIArea // Detection region
	Filter    Filter  // Detection filter
	HP        BarInfo // HP bar info
	MP        BarInfo // MP bar info
	FP        BarInfo // FP bar info (target doesn't have this)
}

// MobsInfo represents HSV color information for mob detection
type MobsInfo struct {
	MinH int
	MaxH int
	MinS int
	MaxS int
	MinV int
	MaxV int
}

// MobsPosition represents a detected mob's position
type MobsPosition struct {
	MinX int
	MaxX int
	MinY int
	MaxY int
}

// Mobs represents mob detection data
type Mobs struct {
	ROI            ROIArea        // Detection region
	Filter         Filter         // Detection filter
	AggressiveInfo MobsInfo       // Aggressive mob color info
	PassiveInfo    MobsInfo       // Passive mob color info
	VioletInfo     MobsInfo       // Violet mob color info
	AggressiveMobs []MobsPosition // Detected aggressive mobs
	PassiveMobs    []MobsPosition // Detected passive mobs
	VioletMobs     []MobsPosition // Detected violet mobs
}

// ClientDetect holds all client detection data
type ClientDetect struct {
	Debug   bool      // If true, save detection images and results to current directory
	DebugUI *Debug    // Debug UI manager for displaying images on main thread
	MyStats StatsBar  // Player stats
	Target  StatsBar  // Target stats
	Mobs    Mobs      // Mobs detection
	mat     *gocv.Mat // Current frame image in Mat format (pointer, nil if not initialized)
	Config  *Config   // Config reference for logging
}

// NewClientDetect creates and initializes a new ClientDetect
func NewClientDetect(cfg *Config) *ClientDetect {
	cd := &ClientDetect{
		Debug:  false,
		Config: cfg,
	}

	// Initialize MyStats
	cd.MyStats.ROI = ROIArea{MinX: 0, MinY: 0, MaxX: 500, MaxY: 350}
	cd.MyStats.HP = BarInfo{
		BarKind: BarKindHP,
		MinH:    160, MaxH: 180,
		MinS: 100, MaxS: 240,
		MinV: 100, MaxV: 240,
	}
	cd.MyStats.MP = BarInfo{
		BarKind: BarKindMP,
		MinH:    90, MaxH: 120,
		MinS: 100, MaxS: 240,
		MinV: 100, MaxV: 240,
	}
	cd.MyStats.FP = BarInfo{
		BarKind: BarKindFP,
		// MinH:    45, MaxH: 70,
		MinH: 0, MaxH: 180,
		MinS: 100, MaxS: 240,
		MinV: 100, MaxV: 240,
	}
	cd.MyStats.Filter = Filter{
		MinWidth:   1,
		MaxWidth:   300,
		MinHeight:  12,
		MaxHeight:  30,
		MorphShape: gocv.MorphRect,
		MorphPoint: image.Pt(25, 25),
		MorphIter:  3,
	}

	// Initialize Target
	cd.Target.ROI = ROIArea{MinX: 400, MinY: 200, MaxX: -400, MaxY: 200}
	cd.Target.HP = BarInfo{
		BarKind: BarKindTargetHP,
		MinH:    340, MaxH: 350,
		MinS: 120, MaxS: 200,
		MinV: 150, MaxV: 230,
	}
	cd.Target.MP = BarInfo{
		BarKind: BarKindTargetMP,
		MinH:    198, MaxH: 234,
		MinS: 114, MaxS: 200,
		MinV: 190, MaxV: 240,
	}
	cd.Target.FP = BarInfo{BarKind: BarKindUnused} // Target doesn't have FP
	cd.Target.Filter = Filter{
		MinWidth:   1,
		MaxWidth:   600,
		MinHeight:  12,
		MaxHeight:  30,
		MorphShape: gocv.MorphRect,
		MorphPoint: image.Pt(25, 25),
		MorphIter:  3,
	}

	// Initialize Mobs
	cd.Mobs.ROI = ROIArea{MinX: 0, MinY: 0, MaxX: -1, MaxY: -100} // Full screen except bottom 100px
	cd.Mobs.AggressiveInfo = MobsInfo{
		MinH: 0, MaxH: 10,
		MinS: 200, MaxS: 255,
		MinV: 200, MaxV: 255,
	}
	cd.Mobs.PassiveInfo = MobsInfo{
		MinH: 58, MaxH: 62,
		MinS: 50, MaxS: 90,
		MinV: 180, MaxV: 255,
	}
	cd.Mobs.VioletInfo = MobsInfo{
		MinH: 260, MaxH: 320,
		MinS: 100, MaxS: 255,
		MinV: 100, MaxV: 255,
	}
	cd.Mobs.Filter = Filter{
		MinWidth:   50,
		MaxWidth:   700,
		MinHeight:  10,
		MaxHeight:  30,
		MorphShape: gocv.MorphRect,
		MorphPoint: image.Pt(10, 10),
		MorphIter:  5,
	}
	cd.Mobs.AggressiveMobs = make([]MobsPosition, 0)
	cd.Mobs.PassiveMobs = make([]MobsPosition, 0)
	cd.Mobs.VioletMobs = make([]MobsPosition, 0)

	return cd
}

// UpdateImage converts *image.RGBA to gocv.Mat and stores it internally
func (cd *ClientDetect) UpdateImage(img *image.RGBA) error {
	// Close previous mat if it exists
	if cd.mat != nil {
		cd.mat.Close()
	}

	// Convert image.RGBA to gocv.Mat
	mat, err := gocv.ImageToMatRGB(img)
	if err != nil {
		return err
	}

	cd.mat = &mat
	return nil
}

// Close releases the mat resource
func (cd *ClientDetect) Close() {
	if cd.mat != nil {
		cd.mat.Close()
	}
}

// updateStateDetect detects a single bar and updates its info (uses internal mat)
func (cd *ClientDetect) updateStateDetect(barInfo *BarInfo, roi ROIArea, filter Filter, debug bool, debugName string) {
	// If bar kind is unused, skip detection
	if barInfo.BarKind == BarKindUnused {
		return
	}

	// Adjust ROI for negative values (relative to image size)
	actualROI := roi
	if actualROI.MinX < 0 {
		actualROI.MinX = cd.mat.Cols() + actualROI.MinX
	}
	if actualROI.MaxX < 0 {
		actualROI.MaxX = cd.mat.Cols() + actualROI.MaxX
	}
	if actualROI.MinY < 0 {
		actualROI.MinY = cd.mat.Rows() + actualROI.MinY
	}
	if actualROI.MaxY < 0 {
		actualROI.MaxY = cd.mat.Rows() + actualROI.MaxY
	}

	// Ensure ROI is within image bounds
	if actualROI.MinX < 0 || actualROI.MinY < 0 ||
		actualROI.MaxX > cd.mat.Cols() || actualROI.MaxY > cd.mat.Rows() ||
		actualROI.MinX >= actualROI.MaxX || actualROI.MinY >= actualROI.MaxY {
		return
	}

	// Extract ROI
	roiMat := cd.mat.Region(image.Rect(actualROI.MinX, actualROI.MinY, actualROI.MaxX, actualROI.MaxY))
	defer roiMat.Close()

	// Convert to HSV
	hsvMat := gocv.NewMat()
	defer hsvMat.Close()
	gocv.CvtColor(roiMat, &hsvMat, gocv.ColorBGRToHSV)

	// Create HSV mask
	lower := gocv.NewScalar(float64(barInfo.MinH), float64(barInfo.MinS), float64(barInfo.MinV), 0)
	upper := gocv.NewScalar(float64(barInfo.MaxH), float64(barInfo.MaxS), float64(barInfo.MaxV), 0)

	// Log HSV range
	if cd.Config != nil && debug {
		cd.Config.Log("[HSV] %s - Min(H:%d, S:%d, V:%d) Max(H:%d, S:%d, V:%d)",
			debugName, barInfo.MinH, barInfo.MinS, barInfo.MinV,
			barInfo.MaxH, barInfo.MaxS, barInfo.MaxV)
	}

	mask := gocv.NewMat()
	defer mask.Close()
	gocv.InRangeWithScalar(hsvMat, lower, upper, &mask)

	// Apply morphological operations
	kernel := gocv.GetStructuringElement(filter.MorphShape, filter.MorphPoint)
	defer kernel.Close()

	morphed := gocv.NewMat()
	defer morphed.Close()

	// Apply morphological opening (erosion followed by dilation)
	for i := 0; i < filter.MorphIter; i++ {
		if i == 0 {
			gocv.MorphologyEx(mask, &morphed, gocv.MorphOpen, kernel)
		} else {
			temp := gocv.NewMat()
			gocv.MorphologyEx(morphed, &temp, gocv.MorphOpen, kernel)
			morphed.Close()
			morphed = temp
		}
	}

	// Find contours
	contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	// Find the largest valid contour
	maxWidth := 0
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		rect := gocv.BoundingRect(contour)

		width := rect.Dx()
		height := rect.Dy()

		// Filter by size constraints
		if width >= filter.MinWidth && width <= filter.MaxWidth &&
			height >= filter.MinHeight && height <= filter.MaxHeight {
			if width > maxWidth {
				maxWidth = width
			}
		}
	}

	// Update barInfo
	prevWidth := barInfo.Width
	barInfo.Width = maxWidth

	// Update maxWidth if width has been stable for 30 times
	if barInfo.Width == prevWidth {
		barInfo.MaxCount++
		if barInfo.MaxCount >= 30 && barInfo.Width > barInfo.MaxWidth {
			barInfo.MaxWidth = barInfo.Width
			barInfo.MaxCount = 0
		}
	} else {
		barInfo.MaxCount = 0
	}

	// Update maxWidth if current width is larger
	if barInfo.Width > barInfo.MaxWidth {
		barInfo.MaxWidth = barInfo.Width
	}

	// Calculate percentage
	roiWidth := actualROI.MaxX - actualROI.MinX
	if barInfo.MaxWidth > 0 {
		barInfo.Value = (barInfo.Width * 100) / barInfo.MaxWidth
	} else if roiWidth > 0 {
		barInfo.Value = (barInfo.Width * 100) / roiWidth
	} else {
		barInfo.Value = 0
	}

	// Clamp value to [0, 100]
	if barInfo.Value < 0 {
		barInfo.Value = 0
	}
	if barInfo.Value > 100 {
		barInfo.Value = 100
	}

	// Debug: send images to debug UI
	if debug && debugName != "" && cd.DebugUI != nil {
		// Convert mask to BGR for display
		maskBGR := gocv.NewMat()
		defer maskBGR.Close()
		gocv.CvtColor(morphed, &maskBGR, gocv.ColorGrayToBGR)

		// Create result mat with annotations
		resultMat := roiMat.Clone()
		defer resultMat.Close()

		// Draw detected bar on result
		if maxWidth > 0 {
			for i := 0; i < contours.Size(); i++ {
				contour := contours.At(i)
				rect := gocv.BoundingRect(contour)
				if rect.Dx() == maxWidth {
					gocv.Rectangle(&resultMat, rect, color.RGBA{255, 0, 0, 255}, 2)

					// Add text
					text := fmt.Sprintf("%s: %d%% (w=%d, mw=%d)", debugName, barInfo.Value, barInfo.Width, barInfo.MaxWidth)
					gocv.PutText(&resultMat, text,
						image.Pt(rect.Min.X, rect.Min.Y-5),
						gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 0, 255}, 1)
					break
				}
			}
		}

		// Send images to debug UI (will be displayed on main thread)
		cd.DebugUI.SendUpdate(debugName, roiMat, maskBGR, resultMat)
	}
}

// updateState updates the state of a StatsBar (uses internal mat)
func (cd *ClientDetect) updateState(statsBar *StatsBar, debug bool, namePrefix string) {
	// Update each bar
	cd.updateStateDetect(&statsBar.HP, statsBar.ROI, statsBar.Filter, debug, namePrefix+"HP")
	cd.updateStateDetect(&statsBar.MP, statsBar.ROI, statsBar.Filter, debug, namePrefix+"MP")
	cd.updateStateDetect(&statsBar.FP, statsBar.ROI, statsBar.Filter, debug, namePrefix+"FP")

	// Check if stats are open (if HP, FP, MP all 0 for 5+ times, not open)
	if statsBar.HP.Value == 0 && statsBar.MP.Value == 0 && statsBar.FP.Value == 0 {
		statsBar.OpenCount++
		if statsBar.OpenCount >= 5 {
			statsBar.Open = false
		}
	} else {
		statsBar.OpenCount = 0
		statsBar.Open = true
	}

	// Check if NPC (FP not active, HP=100, MP=0)
	if statsBar.FP.BarKind == BarKindUnused || statsBar.FP.Value == 0 {
		if statsBar.HP.Value == 100 && statsBar.MP.Value == 0 {
			statsBar.NPC = true
		} else {
			statsBar.NPC = false
		}
	} else {
		statsBar.NPC = false
	}

	// Check if alive (HP > 0)
	statsBar.Alive = statsBar.HP.Value > 0
}

// updateMobsDetect detects mobs and updates the mobs list (uses internal mat)
func (cd *ClientDetect) updateMobsDetect(mobsList *[]MobsPosition, mobsInfo *MobsInfo, roi ROIArea, filter Filter, debug bool, debugName string) {
	// Clear mobs list
	*mobsList = (*mobsList)[:0]

	// Adjust ROI for negative values (relative to image size)
	actualROI := roi
	if actualROI.MinX < 0 {
		actualROI.MinX = cd.mat.Cols() + actualROI.MinX
	}
	if actualROI.MaxX < 0 {
		actualROI.MaxX = cd.mat.Cols() + actualROI.MaxX
	}
	if actualROI.MinY < 0 {
		actualROI.MinY = cd.mat.Rows() + actualROI.MinY
	}
	if actualROI.MaxY < 0 {
		actualROI.MaxY = cd.mat.Rows() + actualROI.MaxY
	}

	// Ensure ROI is within image bounds
	if actualROI.MinX < 0 || actualROI.MinY < 0 ||
		actualROI.MaxX > cd.mat.Cols() || actualROI.MaxY > cd.mat.Rows() ||
		actualROI.MinX >= actualROI.MaxX || actualROI.MinY >= actualROI.MaxY {
		return
	}

	// Extract ROI
	roiMat := cd.mat.Region(image.Rect(actualROI.MinX, actualROI.MinY, actualROI.MaxX, actualROI.MaxY))
	defer roiMat.Close()

	// Convert to HSV
	hsvMat := gocv.NewMat()
	defer hsvMat.Close()
	gocv.CvtColor(roiMat, &hsvMat, gocv.ColorBGRToHSV)

	// Create HSV mask
	lower := gocv.NewScalar(float64(mobsInfo.MinH), float64(mobsInfo.MinS), float64(mobsInfo.MinV), 0)
	upper := gocv.NewScalar(float64(mobsInfo.MaxH), float64(mobsInfo.MaxS), float64(mobsInfo.MaxV), 0)

	// Log HSV range
	if cd.Config != nil && debug {
		cd.Config.Log("[HSV] %s Mobs - Min(H:%d, S:%d, V:%d) Max(H:%d, S:%d, V:%d)",
			debugName, mobsInfo.MinH, mobsInfo.MinS, mobsInfo.MinV,
			mobsInfo.MaxH, mobsInfo.MaxS, mobsInfo.MaxV)
	}

	mask := gocv.NewMat()
	defer mask.Close()
	gocv.InRangeWithScalar(hsvMat, lower, upper, &mask)

	// Apply morphological operations
	kernel := gocv.GetStructuringElement(filter.MorphShape, filter.MorphPoint)
	defer kernel.Close()

	morphed := gocv.NewMat()
	defer morphed.Close()

	// Apply morphological opening
	for i := 0; i < filter.MorphIter; i++ {
		if i == 0 {
			gocv.MorphologyEx(mask, &morphed, gocv.MorphOpen, kernel)
		} else {
			temp := gocv.NewMat()
			gocv.MorphologyEx(morphed, &temp, gocv.MorphOpen, kernel)
			morphed.Close()
			morphed = temp
		}
	}

	// Find contours
	contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	// Scan and add valid mobs to the list
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		rect := gocv.BoundingRect(contour)

		width := rect.Dx()
		height := rect.Dy()

		// Filter by size constraints
		if width >= filter.MinWidth && width <= filter.MaxWidth &&
			height >= filter.MinHeight && height <= filter.MaxHeight {
			// Convert to screen coordinates
			mob := MobsPosition{
				MinX: actualROI.MinX + rect.Min.X,
				MaxX: actualROI.MinX + rect.Max.X,
				MinY: actualROI.MinY + rect.Min.Y,
				MaxY: actualROI.MinY + rect.Max.Y,
			}

			// Filter: avoid HP bar region (top-left corner)
			if mob.MinX <= 250 && mob.MinY <= 110 {
				continue
			}

			*mobsList = append(*mobsList, mob)
		}
	}

	// Debug: send images to debug UI
	if debug && debugName != "" && cd.DebugUI != nil {
		// Convert mask to BGR for display
		maskBGR := gocv.NewMat()
		defer maskBGR.Close()
		gocv.CvtColor(morphed, &maskBGR, gocv.ColorGrayToBGR)

		// Create result mat with annotations
		resultMat := roiMat.Clone()
		defer resultMat.Close()

		// Draw detected mobs on result
		for _, mob := range *mobsList {
			// Convert to ROI coordinates
			roiRect := image.Rect(
				mob.MinX-actualROI.MinX,
				mob.MinY-actualROI.MinY,
				mob.MaxX-actualROI.MinX,
				mob.MaxY-actualROI.MinY,
			)
			gocv.Rectangle(&resultMat, roiRect, color.RGBA{0, 255, 255, 255}, 2)
		}

		// Add text
		text := fmt.Sprintf("%s: %d mobs", debugName, len(*mobsList))
		gocv.PutText(&resultMat, text,
			image.Pt(10, 30),
			gocv.FontHersheyPlain, 1.5, color.RGBA{255, 255, 0, 255}, 2)

		// Send images to debug UI (will be displayed on main thread)
		cd.DebugUI.SendUpdate(debugName, roiMat, maskBGR, resultMat)
	}
}

// updateMobs updates all mobs detection (uses internal mat)
func (cd *ClientDetect) updateMobs(debug bool) {
	cd.updateMobsDetect(&cd.Mobs.AggressiveMobs, &cd.Mobs.AggressiveInfo, cd.Mobs.ROI, cd.Mobs.Filter, debug, "Aggressive")
	cd.updateMobsDetect(&cd.Mobs.PassiveMobs, &cd.Mobs.PassiveInfo, cd.Mobs.ROI, cd.Mobs.Filter, debug, "Passive")
	cd.updateMobsDetect(&cd.Mobs.VioletMobs, &cd.Mobs.VioletInfo, cd.Mobs.ROI, cd.Mobs.Filter, debug, "Violet")
}

// UpdateMyStats updates player stats detection
func (cd *ClientDetect) UpdateMyStats() {
	cd.updateState(&cd.MyStats, cd.Debug, "My")
}

// UpdateTargetStats updates target stats detection
func (cd *ClientDetect) UpdateTargetStats() {
	cd.updateState(&cd.Target, cd.Debug, "Target")
}

// UpdateMobs updates mobs detection
func (cd *ClientDetect) UpdateMobs() {
	cd.updateMobs(cd.Debug)
}

// UpdateClientDetect updates all client detection data (uses internal mat)
func (cd *ClientDetect) UpdateClientDetect() {
	cd.updateState(&cd.MyStats, cd.Debug, "My")
	cd.updateState(&cd.Target, cd.Debug, "Target")
	cd.updateMobs(cd.Debug)
}
