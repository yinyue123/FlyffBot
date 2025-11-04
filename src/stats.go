// Package main - stats.go
//
// This file implements player and target statistics detection using OpenCV HSV color matching.
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

// Constraint defines filtering constraints for detection
type Constraint struct {
	MinWidth   int           // Minimum width for filtering
	MaxWidth   int           // Maximum width for filtering
	MinHeight  int           // Minimum height for filtering
	MaxHeight  int           // Maximum height for filtering
	MorphType  gocv.MorphType // Morphology type (default: MorphOpen)
	MorphPoint image.Point   // Morphology kernel size (default: image.Pt(5, 5))
	MorphIter  int           // Morphology iterations (default: 3)
}

// StatsBar represents a group of status bars (HP/MP/FP)
type StatsBar struct {
	Open      bool    // Whether the stats are visible
	OpenCount int     // Counter for closed stats (if hp/fp/mp all 0 for 5+ times, not open)
	Alive     bool    // Whether alive (hp > 0)
	NPC       bool    // Whether NPC (hp=100, mp=0, fp not active)
	ROI       ROIArea // Detection region
	HP        BarInfo // HP bar info
	MP        BarInfo // MP bar info
	FP        BarInfo // FP bar info (target doesn't have this)
	Constraint Constraint // Detection constraint
}

// ClientStats holds all client statistics
type ClientStats struct {
	Debug    bool     // If true, save detection images and results to current directory
	MyStats  StatsBar // Player stats
	Target   StatsBar // Target stats
}

// NewClientStats creates and initializes a new ClientStats
func NewClientStats() *ClientStats {
	cs := &ClientStats{
		Debug: false,
	}

	// Initialize MyStats
	cs.MyStats.ROI = ROIArea{MinX: 0, MinY: 0, MaxX: 500, MaxY: 350}
	cs.MyStats.HP = BarInfo{
		BarKind: BarKindHP,
		MinH: 170, MaxH: 175,
		MinS: 120, MaxS: 200,
		MinV: 150, MaxV: 230,
	}
	cs.MyStats.MP = BarInfo{
		BarKind: BarKindMP,
		MinH: 99, MaxH: 117,
		MinS: 114, MaxS: 200,
		MinV: 190, MaxV: 240,
	}
	cs.MyStats.FP = BarInfo{
		BarKind: BarKindFP,
		MinH: 52, MaxH: 60,
		MinS: 150, MaxS: 173,
		MinV: 150, MaxV: 230,
	}
	cs.MyStats.Constraint = Constraint{
		MinWidth:   1,
		MaxWidth:   300,
		MinHeight:  12,
		MaxHeight:  30,
		MorphType:  gocv.MorphOpen,
		MorphPoint: image.Pt(25, 25),
		MorphIter:  3,
	}

	// Initialize Target
	cs.Target.ROI = ROIArea{MinX: 400, MinY: 200, MaxX: -400, MaxY: 200}
	cs.Target.HP = BarInfo{
		BarKind: BarKindTargetHP,
		MinH: 170, MaxH: 175,
		MinS: 120, MaxS: 200,
		MinV: 150, MaxV: 230,
	}
	cs.Target.MP = BarInfo{
		BarKind: BarKindTargetMP,
		MinH: 99, MaxH: 117,
		MinS: 114, MaxS: 200,
		MinV: 190, MaxV: 240,
	}
	cs.Target.FP = BarInfo{BarKind: BarKindUnused} // Target doesn't have FP
	cs.Target.Constraint = Constraint{
		MinWidth:   1,
		MaxWidth:   600,
		MinHeight:  12,
		MaxHeight:  30,
		MorphType:  gocv.MorphOpen,
		MorphPoint: image.Pt(25, 25),
		MorphIter:  3,
	}

	return cs
}

// UpdateDetect detects a single bar and updates its info
func UpdateDetect(mat gocv.Mat, barInfo *BarInfo, roi ROIArea, constraint Constraint, debug bool, debugName string) {
	// If bar kind is unused, skip detection
	if barInfo.BarKind == BarKindUnused {
		return
	}

	// Adjust ROI for negative values (relative to image size)
	actualROI := roi
	if actualROI.MinX < 0 {
		actualROI.MinX = mat.Cols() + actualROI.MinX
	}
	if actualROI.MaxX < 0 {
		actualROI.MaxX = mat.Cols() + actualROI.MaxX
	}
	if actualROI.MinY < 0 {
		actualROI.MinY = mat.Rows() + actualROI.MinY
	}
	if actualROI.MaxY < 0 {
		actualROI.MaxY = mat.Rows() + actualROI.MaxY
	}

	// Ensure ROI is within image bounds
	if actualROI.MinX < 0 || actualROI.MinY < 0 ||
	   actualROI.MaxX > mat.Cols() || actualROI.MaxY > mat.Rows() ||
	   actualROI.MinX >= actualROI.MaxX || actualROI.MinY >= actualROI.MaxY {
		return
	}

	// Extract ROI
	roiMat := mat.Region(image.Rect(actualROI.MinX, actualROI.MinY, actualROI.MaxX, actualROI.MaxY))
	defer roiMat.Close()

	// Convert to HSV
	hsvMat := gocv.NewMat()
	defer hsvMat.Close()
	gocv.CvtColor(roiMat, &hsvMat, gocv.ColorBGRToHSV)

	// Create HSV mask
	lower := gocv.NewScalar(float64(barInfo.MinH), float64(barInfo.MinS), float64(barInfo.MinV), 0)
	upper := gocv.NewScalar(float64(barInfo.MaxH), float64(barInfo.MaxS), float64(barInfo.MaxV), 0)
	mask := gocv.NewMat()
	defer mask.Close()
	gocv.InRangeWithScalar(hsvMat, lower, upper, &mask)

	// Apply morphological operations
	kernel := gocv.GetStructuringElement(constraint.MorphType, constraint.MorphPoint)
	defer kernel.Close()

	morphed := gocv.NewMat()
	defer morphed.Close()

	// Use morphological operations based on MorphType
	for i := 0; i < constraint.MorphIter; i++ {
		if i == 0 {
			gocv.MorphologyEx(mask, &morphed, constraint.MorphType, kernel)
		} else {
			temp := gocv.NewMat()
			gocv.MorphologyEx(morphed, &temp, constraint.MorphType, kernel)
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
		if width >= constraint.MinWidth && width <= constraint.MaxWidth &&
		   height >= constraint.MinHeight && height <= constraint.MaxHeight {
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

	// Debug: save processed image
	if debug && debugName != "" {
		debugImg := gocv.NewMat()
		defer debugImg.Close()

		// Draw ROI rectangle
		gocv.Rectangle(&mat, image.Rect(actualROI.MinX, actualROI.MinY, actualROI.MaxX, actualROI.MaxY),
			color.RGBA{0, 255, 0, 255}, 2)

		// Draw detected bar
		if maxWidth > 0 {
			for i := 0; i < contours.Size(); i++ {
				contour := contours.At(i)
				rect := gocv.BoundingRect(contour)
				if rect.Dx() == maxWidth {
					// Adjust coordinates to absolute position
					absRect := image.Rect(
						rect.Min.X + actualROI.MinX,
						rect.Min.Y + actualROI.MinY,
						rect.Max.X + actualROI.MinX,
						rect.Max.Y + actualROI.MinY,
					)
					gocv.Rectangle(&mat, absRect, color.RGBA{255, 0, 0, 255}, 2)

					// Add text
					text := fmt.Sprintf("%s: %d%% (w=%d, mw=%d)", debugName, barInfo.Value, barInfo.Width, barInfo.MaxWidth)
					gocv.PutText(&mat, text,
						image.Pt(absRect.Min.X, absRect.Min.Y-5),
						gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 0, 255}, 1)
					break
				}
			}
		}

		// Save the image
		filename := fmt.Sprintf("%s.jpeg", debugName)
		gocv.IMWrite(filename, mat)

		// Save mask
		maskFilename := fmt.Sprintf("%s_mask.jpeg", debugName)
		gocv.IMWrite(maskFilename, morphed)
	}
}

// UpdateState updates the state of a StatsBar
func UpdateState(mat gocv.Mat, statsBar *StatsBar, debug bool, namePrefix string) {
	// Update each bar
	UpdateDetect(mat, &statsBar.HP, statsBar.ROI, statsBar.Constraint, debug, namePrefix+"HP")
	UpdateDetect(mat, &statsBar.MP, statsBar.ROI, statsBar.Constraint, debug, namePrefix+"MP")
	UpdateDetect(mat, &statsBar.FP, statsBar.ROI, statsBar.Constraint, debug, namePrefix+"FP")

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

// UpdateClientStats updates all client statistics
func (cs *ClientStats) UpdateClientStats(mat gocv.Mat) {
	UpdateState(mat, &cs.MyStats, cs.Debug, "My")
	UpdateState(mat, &cs.Target, cs.Debug, "Target")
}
