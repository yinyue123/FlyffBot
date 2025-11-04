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
	"sync"

	"gocv.io/x/gocv"
)

// Note: Color, Bounds, Target, MobType, and other basic types are defined in data.go
// Note: Point and ScreenInfo are defined in data.go

// ImageAnalyzer handles OpenCV-based image analysis
type ImageAnalyzer struct {
	browser    *Browser
	screenInfo *ScreenInfo
	lastImage  *image.RGBA
	stats      *ClientStats
	mu         sync.RWMutex
}

// NewImageAnalyzer creates a new image analyzer with OpenCV support
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

