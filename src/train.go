// Package main - train.go
//
// Training/Testing mode for offline detection debugging.
// Loads train.png, performs detection, draws bounding boxes, saves to result.png.
//
// Usage:
//   1. Place a screenshot as train.png in the current directory
//   2. Run: go run . --train
//   3. Check result.png for visualization
//   4. Check Debug.log for detailed detection info
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

// TrainingMode runs offline detection on train.png
func TrainingMode() error {
	LogInfo("=== Training Mode Started ===")

	// Check if train.png exists
	trainPath := "train.png"
	if _, err := os.Stat(trainPath); os.IsNotExist(err) {
		LogError("train.png not found in current directory")
		LogInfo("Please place a screenshot as train.png and run again")
		return err
	}

	// Load train.png
	LogInfo("Loading %s...", trainPath)
	img, err := loadPNG(trainPath)
	if err != nil {
		LogError("Failed to load train.png: %v", err)
		return err
	}
	LogInfo("Image loaded: %dx%d", img.Bounds().Dx(), img.Bounds().Dy())

	// Load configuration
	data, err := LoadData()
	if err != nil {
		LogError("Failed to load data: %v, using defaults", err)
		data = NewPersistentData()
	}
	config := data.Config

	// Create analyzer with the loaded image
	analyzer := &ImageAnalyzer{
		screenInfo: NewScreenInfo(img.Bounds()),
		stats:      NewClientStats(),
	}
	analyzer.lastImage = img

	// Run detection
	LogInfo("=== Running Detection ===")

	// 1. Detect mobs
	LogInfo("Detecting mobs...")
	mobs := analyzer.IdentifyMobs(config)
	LogInfo("Found %d mobs", len(mobs))

	// 2. Update stats (HP/MP/FP)
	LogInfo("Detecting player stats...")
	analyzer.UpdateStats()
	stats := analyzer.GetStats()
	LogInfo("HP: %d%%, MP: %d%%, FP: %d%%", stats.HP.Value, stats.MP.Value, stats.FP.Value)
	LogInfo("Target HP: %d%%", stats.TargetHP.Value)
	LogInfo("Target on screen: %v", stats.TargetOnScreen)

	// 3. Detect target marker
	LogInfo("Detecting target marker...")
	hasTarget := analyzer.DetectTargetMarker()
	LogInfo("Target marker detected: %v", hasTarget)

	// Create result image with visualizations
	LogInfo("=== Creating Visualization ===")
	resultImg := drawDetectionResults(img, mobs, stats, config)

	// Save result.png
	resultPath := "result.png"
	LogInfo("Saving visualization to %s...", resultPath)
	if err := savePNG(resultPath, resultImg); err != nil {
		LogError("Failed to save result.png: %v", err)
		return err
	}
	LogInfo("Saved successfully!")

	// Verify result.png was saved
	if info, err := os.Stat(resultPath); err == nil {
		LogInfo("result.png size: %d bytes", info.Size())
		LogInfo("=== Training Mode Completed ===")
		LogInfo("Please check result.png for visualization")
		LogInfo("Please check Debug.log for detailed detection info")
	} else {
		LogError("Failed to verify result.png: %v", err)
		return err
	}

	return nil
}

// loadPNG loads a PNG image from file
func loadPNG(filename string) (*image.RGBA, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}

	// Convert to RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	return rgba, nil
}

// savePNG saves an image to PNG file
func savePNG(filename string, img image.Image) error {
	// Create directory if needed
	dir := filepath.Dir(filename)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

// drawDetectionResults draws bounding boxes and labels on the image
func drawDetectionResults(img *image.RGBA, mobs []Target, stats *ClientStats, config *Config) *image.RGBA {
	// Create a copy of the image
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, img, bounds.Min, draw.Src)

	// Draw mob bounding boxes
	for i, mob := range mobs {
		var boxColor color.RGBA
		var label string

		switch mob.Type {
		case MobPassive:
			boxColor = color.RGBA{R: 234, G: 234, B: 149, A: 255} // Yellow
			label = "Passive"
		case MobAggressive:
			boxColor = color.RGBA{R: 179, G: 23, B: 23, A: 255} // Red
			label = "Aggressive"
		case MobViolet:
			boxColor = color.RGBA{R: 182, G: 144, B: 146, A: 255} // Purple
			label = "Violet"
		}

		// Draw bounding box
		drawRect(result, mob.Bounds, boxColor, 2)

		// Draw label with mob index
		labelText := fmt.Sprintf("#%d %s (%dx%d)", i+1, label, mob.Bounds.W, mob.Bounds.H)
		drawText(result, mob.Bounds.X, mob.Bounds.Y-15, labelText, boxColor)
	}

	// Draw HP bar region indicator
	hpBarBounds := Bounds{X: 0, Y: 0, W: 250, H: 110}
	drawRect(result, hpBarBounds, color.RGBA{R: 0, G: 255, B: 255, A: 128}, 1)
	drawText(result, 10, 120, "HP Bar Region (Excluded)", color.RGBA{R: 0, G: 255, B: 255, A: 255})

	// Draw player stats
	statsY := 140
	drawText(result, 10, statsY, fmt.Sprintf("HP: %d%%", stats.HP.Value), color.RGBA{R: 255, G: 0, B: 0, A: 255})
	drawText(result, 10, statsY+20, fmt.Sprintf("MP: %d%%", stats.MP.Value), color.RGBA{R: 0, G: 0, B: 255, A: 255})
	drawText(result, 10, statsY+40, fmt.Sprintf("FP: %d%%", stats.FP.Value), color.RGBA{R: 0, G: 255, B: 0, A: 255})
	drawText(result, 10, statsY+60, fmt.Sprintf("Target HP: %d%%", stats.TargetHP.Value), color.RGBA{R: 255, G: 128, B: 0, A: 255})

	// Draw detection parameters
	paramsY := bounds.Max.Y - 100
	drawText(result, 10, paramsY, fmt.Sprintf("MinWidth: %d, MaxWidth: %d", config.MinMobNameWidth, config.MaxMobNameWidth), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	drawText(result, 10, paramsY+20, fmt.Sprintf("Passive Tol: %d, Aggressive Tol: %d", config.PassiveTolerance, config.AggressiveTolerance), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	drawText(result, 10, paramsY+40, fmt.Sprintf("Total Mobs: %d", len(mobs)), color.RGBA{R: 255, G: 255, B: 0, A: 255})

	return result
}

// drawRect draws a rectangle outline
func drawRect(img *image.RGBA, bounds Bounds, col color.RGBA, thickness int) {
	// Top
	for t := 0; t < thickness; t++ {
		for x := bounds.X; x < bounds.X+bounds.W; x++ {
			if y := bounds.Y + t; y >= 0 && y < img.Bounds().Max.Y {
				img.Set(x, y, col)
			}
		}
	}

	// Bottom
	for t := 0; t < thickness; t++ {
		for x := bounds.X; x < bounds.X+bounds.W; x++ {
			if y := bounds.Y + bounds.H - t - 1; y >= 0 && y < img.Bounds().Max.Y {
				img.Set(x, y, col)
			}
		}
	}

	// Left
	for t := 0; t < thickness; t++ {
		for y := bounds.Y; y < bounds.Y+bounds.H; y++ {
			if x := bounds.X + t; x >= 0 && x < img.Bounds().Max.X {
				img.Set(x, y, col)
			}
		}
	}

	// Right
	for t := 0; t < thickness; t++ {
		for y := bounds.Y; y < bounds.Y+bounds.H; y++ {
			if x := bounds.X + bounds.W - t - 1; x >= 0 && x < img.Bounds().Max.X {
				img.Set(x, y, col)
			}
		}
	}
}

// drawText draws simple text (using pixel font pattern)
func drawText(img *image.RGBA, x, y int, text string, col color.RGBA) {
	// Simple 5x7 pixel font (only digits and basic chars for this use case)
	// For production, consider using a proper font rendering library
	// For now, just draw a filled rectangle as placeholder
	width := len(text) * 6
	height := 12

	// Background
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			px := x + dx
			py := y + dy
			if px >= 0 && px < img.Bounds().Max.X && py >= 0 && py < img.Bounds().Max.Y {
				// Semi-transparent black background
				existing := img.RGBAAt(px, py)
				img.Set(px, py, color.RGBA{
					R: uint8((int(existing.R) + 0) / 2),
					G: uint8((int(existing.G) + 0) / 2),
					B: uint8((int(existing.B) + 0) / 2),
					A: 255,
				})
			}
		}
	}

	// Text in color (simplified - just draw the first pixel of each character)
	for i, ch := range text {
		px := x + i*6 + 2
		py := y + 5
		if px >= 0 && px < img.Bounds().Max.X && py >= 0 && py < img.Bounds().Max.Y {
			img.Set(px, py, col)
			img.Set(px+1, py, col)
			img.Set(px, py+1, col)
			img.Set(px+1, py+1, col)
		}
		_ = ch // Avoid unused variable warning
	}
}
