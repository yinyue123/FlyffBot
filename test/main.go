package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"gocv.io/x/gocv"
)

// Target represents a detected monster with its bounding box
type Target struct {
	Name   string
	X      int
	Y      int
	Width  int
	Height int
	Color  string
}

// Stats represents player HP/MP/FP percentages
type Stats struct {
	HP float64
	MP float64
	FP float64
}

// TargetStats represents target HP/MP percentages
type TargetStats struct {
	HP float64
	MP float64
}

// DirectionInfo represents minimap analysis result
type DirectionInfo struct {
	CurrentAngle float64
	BestAngle    float64
	Found        bool
}

// DetectTargets detects monster names (red and yellow) using gocv color detection
func DetectTargets(img gocv.Mat) []Target {
	start := time.Now()
	defer func() {
		fmt.Printf("DetectTargets took: %v\n", time.Since(start))
	}()

	targets := []Target{}

	// Convert to HSV for better color detection
	hsv := gocv.NewMat()
	defer hsv.Close()
	gocv.CvtColor(img, &hsv, gocv.ColorBGRToHSV)

	// Red monster name color detection (红色名字 - aggressive mobs)
	// Red wraps around in HSV, need two ranges
	redLower1 := gocv.NewScalar(0, 80, 80, 0)
	redUpper1 := gocv.NewScalar(10, 255, 255, 0)
	redLower2 := gocv.NewScalar(170, 80, 80, 0)
	redUpper2 := gocv.NewScalar(180, 255, 255, 0)

	maskRed1 := gocv.NewMat()
	maskRed2 := gocv.NewMat()
	maskRed := gocv.NewMat()
	defer maskRed1.Close()
	defer maskRed2.Close()
	defer maskRed.Close()

	gocv.InRangeWithScalar(hsv, redLower1, redUpper1, &maskRed1)
	gocv.InRangeWithScalar(hsv, redLower2, redUpper2, &maskRed2)
	gocv.BitwiseOr(maskRed1, maskRed2, &maskRed)

	// Yellow monster name color detection (黄色名字 - passive mobs)
	yellowLower := gocv.NewScalar(20, 100, 150, 0)
	yellowUpper := gocv.NewScalar(35, 255, 255, 0)

	maskYellow := gocv.NewMat()
	defer maskYellow.Close()
	gocv.InRangeWithScalar(hsv, yellowLower, yellowUpper, &maskYellow)

	// Process red monsters
	redTargets := extractTargets(maskRed, "Small Mia", "Red")
	targets = append(targets, redTargets...)

	// Process yellow monsters
	yellowTargets := extractTargets(maskYellow, "Small Mia", "Yellow")
	targets = append(targets, yellowTargets...)

	return targets
}

// extractTargets extracts bounding boxes from a binary mask using morphology and contour detection
func extractTargets(mask gocv.Mat, name string, colorType string) []Target {
	targets := []Target{}

	// Apply morphological operations to clean up the mask and connect nearby pixels
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(5, 3))
	defer kernel.Close()

	cleaned := gocv.NewMat()
	defer cleaned.Close()

	// Close operation to fill small gaps
	gocv.MorphologyEx(mask, &cleaned, gocv.MorphClose, kernel)

	// Dilate slightly to connect text characters
	gocv.Dilate(cleaned, &cleaned, kernel)

	// Find contours
	contours := gocv.FindContours(cleaned, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		rect := gocv.BoundingRect(contour)

		// Filter by size - monster names should have reasonable dimensions
		// Width: 35-260px, Height: 8-30px
		if rect.Dx() >= 35 && rect.Dx() <= 260 && rect.Dy() >= 8 && rect.Dy() <= 30 {
			targets = append(targets, Target{
				Name:   name,
				X:      rect.Min.X,
				Y:      rect.Min.Y,
				Width:  rect.Dx(),
				Height: rect.Dy(),
				Color:  colorType,
			})
		}
	}

	return targets
}

// DetectTargetStats detects target HP and MP bars using color-based bar detection
// Based on Rust implementation: HP x=[300-550], y=[30-60]; MP y=[50-60]
func DetectTargetStats(img gocv.Mat) (TargetStats, bool) {
	start := time.Now()
	defer func() {
		fmt.Printf("DetectTargetStats took: %v\n", time.Since(start))
	}()

	height := img.Rows()
	if height < 80 {
		return TargetStats{}, false
	}

	// Target HP bar region (green bar above monster name)
	// From scanning: y=138-155, x=250-380
	hpRoi := img.Region(image.Rect(250, 145, 380, 147))
	if hpRoi.Empty() {
		return TargetStats{}, false
	}
	defer hpRoi.Close()

	// Target MP bar region (need to find - using placeholder for now)
	mpRoi := img.Region(image.Rect(250, 150, 380, 152))
	if mpRoi.Empty() {
		return TargetStats{}, false
	}
	defer mpRoi.Close()

	// Detect HP and MP percentages using target-specific colors
	hpPercentage := detectBarPercentage(hpRoi, "target_hp")
	mpPercentage := detectBarPercentage(mpRoi, "target_mp")

	if hpPercentage == 0 && mpPercentage == 0 {
		return TargetStats{}, false
	}

	return TargetStats{
		HP: hpPercentage,
		MP: mpPercentage,
	}, true
}

// DetectStats detects player HP, MP, FP stats from top-left UI
// Based on Rust implementation: x=[105-225], y=[30-110] for each bar
func DetectStats(img gocv.Mat) (Stats, bool) {
	start := time.Now()
	defer func() {
		fmt.Printf("DetectStats took: %v\n", time.Since(start))
	}()

	height := img.Rows()
	if height < 250 {
		return Stats{}, false
	}

	// Extract ROI for each stat bar individually (based on actual game screenshot)
	// HP bar region (orange-red bar at top)
	hpRoi := img.Region(image.Rect(200, 8, 400, 10))
	if hpRoi.Empty() {
		return Stats{}, false
	}
	defer hpRoi.Close()

	// MP bar region (blue bar below HP)
	mpRoi := img.Region(image.Rect(200, 14, 400, 16))
	if mpRoi.Empty() {
		return Stats{}, false
	}
	defer mpRoi.Close()

	// FP bar region (green bar below MP)
	fpRoi := img.Region(image.Rect(200, 20, 400, 22))
	if fpRoi.Empty() {
		return Stats{}, false
	}
	defer fpRoi.Close()

	// Detect each bar percentage
	hpPercentage := detectBarPercentage(hpRoi, "hp")
	mpPercentage := detectBarPercentage(mpRoi, "mp")
	fpPercentage := detectBarPercentage(fpRoi, "fp")

	if hpPercentage == 0 && mpPercentage == 0 && fpPercentage == 0 {
		return Stats{}, false
	}

	return Stats{
		HP: hpPercentage,
		MP: mpPercentage,
		FP: fpPercentage,
	}, true
}

// pixelMatches checks if a pixel matches a reference color within tolerance
func pixelMatches(pixel, ref [3]uint8, tolerance uint8) bool {
	for i := 0; i < 3; i++ {
		var diff uint8
		if pixel[i] > ref[i] {
			diff = pixel[i] - ref[i]
		} else {
			diff = ref[i] - pixel[i]
		}
		if diff > tolerance {
			return false
		}
	}
	return true
}

// detectBarPercentage detects a colored bar and returns its fill percentage
// Following the Rust implementation algorithm:
// 1. Scan ROI for pixels matching reference colors (with tolerance)
// 2. Find horizontal bounds (minX, maxX) of matched pixels
// 3. Calculate percentage: barWidth / roiWidth * 100
func detectBarPercentage(roi gocv.Mat, barType string) float64 {
	// Define reference colors (RGB format) based on Rust implementation
	var refColors [][3]uint8

	switch barType {
	case "hp":
		// HP bar colors (orange-red gradient from actual game)
		// Sampled from y=8-10, x=200-400
		refColors = [][3]uint8{
			{51, 129, 156},  // BGR order in OpenCV
			{52, 129, 156},
			{53, 130, 157},
			{55, 132, 158},
			{56, 144, 174},  // Brighter shade
			{69, 200, 237},  // Brightest shade
		}
	case "mp":
		// MP bar colors (grayish from actual game)
		// Sampled from y=14-16, x=200-400
		refColors = [][3]uint8{
			{127, 125, 125},  // BGR order
			{131, 129, 129},
			{132, 129, 129},
			{135, 132, 132},
			{85, 83, 83},     // Darker shade
		}
	case "fp":
		// FP bar colors (dark gray from actual game)
		// Sampled from y=20-22, x=200-400
		refColors = [][3]uint8{
			{78, 74, 76},  // BGR order
			{79, 75, 76},
			{80, 76, 77},
			{72, 68, 69},
			{73, 70, 70},
			{27, 23, 24},  // Darkest shade
		}
	case "target_hp":
		// Target HP bar colors (green from actual game)
		// Sampled from y=138-155 (monster HP bar)
		refColors = [][3]uint8{
			{13, 58, 22},  // BGR order - darker green
			{17, 141, 56},  // Main green color
			{17, 152, 60},
			{18, 163, 65},
			{17, 174, 70},
			{19, 185, 74},
			{19, 196, 79},
			{18, 206, 83},
			{18, 216, 88},  // Brightest green
			{100, 159, 107}, // Light green shade
		}
	case "target_mp":
		// Target MP bar - need to find this
		refColors = [][3]uint8{
			{127, 125, 125},  // Placeholder - using same as player MP
		}
	default:
		return 0
	}

	tolerance := uint8(2) // Same as Rust implementation

	// Find all pixels matching any reference color
	minX := roi.Cols()
	maxX := 0
	foundPixels := false

	for y := 0; y < roi.Rows(); y++ {
		for x := 0; x < roi.Cols(); x++ {
			pixel := [3]uint8{
				roi.GetUCharAt(y, x*3+0), // B
				roi.GetUCharAt(y, x*3+1), // G
				roi.GetUCharAt(y, x*3+2), // R
			}

			// Check if pixel matches any reference color
			for _, ref := range refColors {
				if pixelMatches(pixel, ref, tolerance) {
					foundPixels = true
					if x < minX {
						minX = x
					}
					if x > maxX {
						maxX = x
					}
					break
				}
			}
		}
	}

	if !foundPixels || minX >= maxX {
		return 0
	}

	// Calculate percentage: detected_width / roi_width * 100
	barWidth := float64(maxX - minX + 1)
	totalWidth := float64(roi.Cols())
	percentage := (barWidth / totalWidth) * 100.0

	// Cap at 100%
	if percentage > 100 {
		percentage = 100
	}

	return percentage
}

// DetectDirection analyzes the minimap to find the best direction for monster density
func DetectDirection(img gocv.Mat) DirectionInfo {
	start := time.Now()
	defer func() {
		fmt.Printf("DetectDirection took: %v\n", time.Since(start))
	}()

	// Minimap is in the upper right corner
	height := img.Rows()
	width := img.Cols()

	// Extract minimap region (approximate location)
	minimapSize := 150
	minimapX := width - minimapSize - 15
	minimapY := 15

	if minimapX < 0 || minimapY+minimapSize > height {
		return DirectionInfo{Found: false}
	}

	roi := img.Region(image.Rect(minimapX, minimapY, width-15, minimapY+minimapSize))
	defer roi.Close()

	hsv := gocv.NewMat()
	defer hsv.Close()
	gocv.CvtColor(roi, &hsv, gocv.ColorBGRToHSV)

	// Detect orange monsters on minimap
	orangeLower := gocv.NewScalar(5, 100, 150, 0)
	orangeUpper := gocv.NewScalar(25, 255, 255, 0)

	maskOrange := gocv.NewMat()
	defer maskOrange.Close()
	gocv.InRangeWithScalar(hsv, orangeLower, orangeUpper, &maskOrange)

	// Detect white arrow (player direction indicator)
	whiteLower := gocv.NewScalar(0, 0, 200, 0)
	whiteUpper := gocv.NewScalar(180, 30, 255, 0)

	maskWhite := gocv.NewMat()
	defer maskWhite.Close()
	gocv.InRangeWithScalar(hsv, whiteLower, whiteUpper, &maskWhite)

	// Find center of minimap (player position)
	centerX := roi.Cols() / 2
	centerY := roi.Rows() / 2

	// Calculate current arrow direction from white pixels
	currentAngle := calculateArrowDirection(maskWhite, centerX, centerY)

	// Divide minimap into sectors and count monster density
	sectors := 36 // 10-degree sectors
	sectorWeights := make([]float64, sectors)

	// Use FindContours to find monster positions instead of pixel-by-pixel scan
	contoursOrange := gocv.FindContours(maskOrange, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contoursOrange.Close()

	for i := 0; i < contoursOrange.Size(); i++ {
		contour := contoursOrange.At(i)

		// Get bounding rect to find center of each monster blob
		rect := gocv.BoundingRect(contour)

		// Calculate centroid (center of bounding box)
		x := rect.Min.X + rect.Dx()/2
		y := rect.Min.Y + rect.Dy()/2

		// Calculate angle from center
		dx := float64(x - centerX)
		dy := float64(y - centerY)
		distance := math.Sqrt(dx*dx + dy*dy)

		if distance < 5 { // Skip center area
			continue
		}

		angle := math.Atan2(dy, dx)
		degrees := angle * 180 / math.Pi
		if degrees < 0 {
			degrees += 360
		}

		// Determine sector
		sector := int(degrees / (360.0 / float64(sectors)))
		if sector >= sectors {
			sector = sectors - 1
		}

		// Weight by inverse distance (closer monsters are more important)
		weight := 1.0 / (1.0 + distance/20.0)
		sectorWeights[sector] += weight
	}

	// Find sector with highest weight
	maxWeight := 0.0
	bestSector := 0
	for i, weight := range sectorWeights {
		if weight > maxWeight {
			maxWeight = weight
			bestSector = i
		}
	}

	if maxWeight == 0 {
		return DirectionInfo{Found: false}
	}

	// Convert best sector to angle (center of sector)
	bestAngle := float64(bestSector)*(360.0/float64(sectors)) + (180.0/float64(sectors))

	// Normalize angles to -180 to 180
	currentAngle = normalizeAngle(currentAngle)
	bestAngle = normalizeAngle(bestAngle)

	return DirectionInfo{
		CurrentAngle: currentAngle,
		BestAngle:    bestAngle,
		Found:        true,
	}
}

// calculateArrowDirection calculates the direction of the player arrow from white pixels
func calculateArrowDirection(mask gocv.Mat, centerX, centerY int) float64 {
	sumX := 0.0
	sumY := 0.0
	count := 0

	for y := 0; y < mask.Rows(); y++ {
		for x := 0; x < mask.Cols(); x++ {
			if mask.GetUCharAt(y, x) > 0 {
				sumX += float64(x)
				sumY += float64(y)
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}

	avgX := sumX / float64(count)
	avgY := sumY / float64(count)

	dx := avgX - float64(centerX)
	dy := avgY - float64(centerY)

	angle := math.Atan2(dy, dx) * 180 / math.Pi
	return angle
}

// normalizeAngle normalizes angle to -180 to 180 range
func normalizeAngle(angle float64) float64 {
	for angle > 180 {
		angle -= 360
	}
	for angle <= -180 {
		angle += 360
	}
	return angle
}

// DrawResult draws all detection results on the image using gocv drawing functions
func DrawResult(img gocv.Mat, targets []Target, stats Stats, targetStats *TargetStats, direction DirectionInfo) {
	start := time.Now()
	defer func() {
		fmt.Printf("DrawResult took: %v\n", time.Since(start))
	}()

	// Define colors
	red := color.RGBA{0, 0, 255, 0}       // BGR format in OpenCV
	yellow := color.RGBA{0, 255, 255, 0}
	green := color.RGBA{0, 255, 0, 0}
	blue := color.RGBA{255, 0, 0, 0}
	cyan := color.RGBA{255, 255, 0, 0}
	white := color.RGBA{255, 255, 255, 0}

	// Draw monster name boxes and click points
	for _, target := range targets {
		// Choose color based on monster type
		var boxColor color.RGBA
		if target.Color == "Red" {
			boxColor = red
		} else {
			boxColor = yellow
		}

		// Draw bounding box
		rect := image.Rect(target.X, target.Y, target.X+target.Width, target.Y+target.Height)
		gocv.Rectangle(&img, rect, boxColor, 2)

		// Draw click point (center of name, 30px down)
		clickX := target.X + target.Width/2
		clickY := target.Y + target.Height/2 + 30
		gocv.Circle(&img, image.Pt(clickX, clickY), 5, green, -1)
		gocv.Circle(&img, image.Pt(clickX, clickY), 7, white, 1)

		// Draw label
		label := fmt.Sprintf("%s %s", target.Color, target.Name)
		gocv.PutText(&img, label, image.Pt(target.X, target.Y-8),
			gocv.FontHersheyPlain, 1.0, white, 1)
	}

	// Draw player stats on bottom left
	yOffset := img.Rows() - 100
	gocv.PutText(&img, fmt.Sprintf("Player HP: %.1f%%", stats.HP),
		image.Pt(10, yOffset), gocv.FontHersheyPlain, 1.2, red, 2)
	gocv.PutText(&img, fmt.Sprintf("Player MP: %.1f%%", stats.MP),
		image.Pt(10, yOffset+20), gocv.FontHersheyPlain, 1.2, blue, 2)
	gocv.PutText(&img, fmt.Sprintf("Player FP: %.1f%%", stats.FP),
		image.Pt(10, yOffset+40), gocv.FontHersheyPlain, 1.2, yellow, 2)

	// Draw target stats if available
	if targetStats != nil {
		gocv.PutText(&img, fmt.Sprintf("Target HP: %.1f%%", targetStats.HP),
			image.Pt(10, yOffset+70), gocv.FontHersheyPlain, 1.2, red, 2)
		gocv.PutText(&img, fmt.Sprintf("Target MP: %.1f%%", targetStats.MP),
			image.Pt(10, yOffset+90), gocv.FontHersheyPlain, 1.2, blue, 2)
	} else {
		gocv.PutText(&img, "Target: None",
			image.Pt(10, yOffset+70), gocv.FontHersheyPlain, 1.2, cyan, 2)
	}

	// Draw direction info on top left
	if direction.Found {
		gocv.PutText(&img, fmt.Sprintf("Current Direction: %.1f deg", direction.CurrentAngle),
			image.Pt(10, 30), gocv.FontHersheyPlain, 1.2, cyan, 2)
		gocv.PutText(&img, fmt.Sprintf("Best Direction: %.1f deg", direction.BestAngle),
			image.Pt(10, 50), gocv.FontHersheyPlain, 1.2, green, 2)
	}
}

func main() {
	totalStart := time.Now()

	// Load the training image
	img := gocv.IMRead("../train.png", gocv.IMReadColor)
	if img.Empty() {
		fmt.Println("Error: Could not read image '../train.png'")
		return
	}
	defer img.Close()

	fmt.Printf("Image loaded: %dx%d\n", img.Cols(), img.Rows())
	fmt.Println("Starting detection...")

	// Detect targets (monster names)
	targets := DetectTargets(img)
	fmt.Printf("Found %d targets\n", len(targets))

	// Detect player stats
	stats, statsOk := DetectStats(img)
	if statsOk {
		fmt.Printf("Player Stats - HP: %.1f%%, MP: %.1f%%, FP: %.1f%%\n", stats.HP, stats.MP, stats.FP)
	} else {
		fmt.Println("Player stats not detected")
	}

	// Detect target stats
	targetStats, targetOk := DetectTargetStats(img)
	var targetStatsPtr *TargetStats
	if targetOk {
		fmt.Printf("Target Stats - HP: %.1f%%, MP: %.1f%%\n", targetStats.HP, targetStats.MP)
		targetStatsPtr = &targetStats
	} else {
		fmt.Println("No target stats detected")
		targetStatsPtr = nil
	}

	// Detect direction
	direction := DetectDirection(img)
	if direction.Found {
		fmt.Printf("Direction - Current: %.1f°, Best: %.1f°\n", direction.CurrentAngle, direction.BestAngle)
	} else {
		fmt.Println("Direction not detected")
	}

	// Draw results
	DrawResult(img, targets, stats, targetStatsPtr, direction)

	// Save result
	if ok := gocv.IMWrite("result.png", img); !ok {
		fmt.Println("Error: Could not write result image")
		return
	}

	fmt.Printf("\n=== Performance Summary ===\n")
	fmt.Printf("Total execution time: %v\n", time.Since(totalStart))
	fmt.Println("Result saved to result.png")
}
