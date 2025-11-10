//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"runtime"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"gocv.io/x/gocv"
)

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

// Simple browser for debug
type DebugBrowser struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCtx    context.Context
	allocCancel context.CancelFunc
	frameChan   chan *image.RGBA
}

// loadCookies loads cookies from cookie.json file
func loadCookies(cookiePath string) ([]Cookie, error) {
	data, err := os.ReadFile(cookiePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make([]Cookie, 0), nil
		}
		return nil, fmt.Errorf("failed to read cookie file: %w", err)
	}

	var cookies []Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, fmt.Errorf("failed to parse cookie file: %w", err)
	}

	return cookies, nil
}

func NewDebugBrowser() *DebugBrowser {
	return &DebugBrowser{
		frameChan: make(chan *image.RGBA, 1), // Buffer of 1 to hold latest frame
	}
}

func (b *DebugBrowser) Start(cookies []Cookie) error {
	// Create allocator context - remove headless completely
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(800, 600),
	}

	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
	b.ctx, b.cancel = chromedp.NewContext(b.allocCtx)

	// Start screencast BEFORE navigation
	fmt.Println("Setting up screencast listener...")
	b.setupScreencastListener()

	// Set cookies before navigation if provided
	if len(cookies) > 0 {
		fmt.Printf("Setting %d cookies before navigation\n", len(cookies))
		err := b.setCookies(cookies)
		if err != nil {
			fmt.Printf("Warning: failed to set cookies: %v\n", err)
		}
	}

	// Navigate to game (don't wait for full page load)
	fmt.Println("Navigating to https://universe.flyff.com/play")
	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, _, err := page.Navigate("https://universe.flyff.com/play").Do(ctx)
			return err
		}),
	)

	if err != nil {
		fmt.Printf("Navigation error: %v\n", err)
		return err
	}

	// Give it a moment to start loading
	fmt.Println("Waiting for page to start loading...")
	time.Sleep(2 * time.Second)

	// Start screencast after page loads
	fmt.Println("Starting screencast stream...")
	err = chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return page.StartScreencast().
				WithFormat("jpeg").
				WithQuality(70).
				Do(ctx)
		}),
	)
	if err != nil {
		fmt.Printf("Failed to start screencast: %v\n", err)
		return err
	}

	return nil
}

// setupScreencastListener sets up the event listener for screencast frames
func (b *DebugBrowser) setupScreencastListener() {
	frameCount := 0
	// Listen for screencast frames
	chromedp.ListenTarget(b.ctx, func(ev interface{}) {
		if ev, ok := ev.(*page.EventScreencastFrame); ok {
			frameCount++
			if frameCount%30 == 1 { // Log every 30th frame
				fmt.Printf("Received screencast frame #%d\n", frameCount)
			}

			// Decode the frame
			data, err := base64.StdEncoding.DecodeString(ev.Data)
			if err != nil {
				fmt.Printf("Failed to decode frame: %v\n", err)
				return
			}

			// Decode image
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				fmt.Printf("Failed to decode image: %v\n", err)
				return
			}

			// Convert to RGBA
			bounds := img.Bounds()
			rgba := image.NewRGBA(bounds)
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					rgba.Set(x, y, img.At(x, y))
				}
			}

			// Send frame to channel (non-blocking, drop if full)
			select {
			case b.frameChan <- rgba:
				// Successfully sent frame
			default:
				// Drop frame if channel is full
			}

			// Acknowledge the frame
			go chromedp.Run(b.ctx, page.ScreencastFrameAck(ev.SessionID))
		}
	})
}

// setCookies sets cookies in the browser
func (b *DebugBrowser) setCookies(cookies []Cookie) error {
	if len(cookies) == 0 {
		return nil
	}

	return chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, c := range cookies {
				params := network.SetCookie(c.Name, c.Value).
					WithDomain(c.Domain).
					WithPath(c.Path).
					WithHTTPOnly(c.HTTPOnly).
					WithSecure(c.Secure)

				if c.Expires > 0 {
					expires := cdp.TimeSinceEpoch(time.Unix(int64(c.Expires), 0))
					params = params.WithExpires(&expires)
				}

				if c.SameSite != "" {
					params = params.WithSameSite(network.CookieSameSite(c.SameSite))
				}

				if err := params.Do(ctx); err != nil {
					return err
				}
			}
			return nil
		}),
	)
}

// GetFrame returns the latest frame from the screencast stream
func (b *DebugBrowser) GetFrame() (*image.RGBA, bool) {
	select {
	case frame := <-b.frameChan:
		return frame, true
	default:
		return nil, false
	}
}

func (b *DebugBrowser) Stop() {
	// Stop screencast
	if b.ctx != nil && b.ctx.Err() == nil {
		chromedp.Run(b.ctx, page.StopScreencast())
	}

	// Close frame channel
	if b.frameChan != nil {
		close(b.frameChan)
	}

	if b.cancel != nil {
		b.cancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
}

// detectStatusBars detects status bars and displays the result
func detectStatusBars(mat gocv.Mat, window *gocv.Window, vThreshold, blurSize, morphWidth, morphHeight, minWidth, maxWidth, minHeight, maxHeight int) {
	// Fixed ROI: (0,0) to (468,230)
	roiRect := image.Rect(0, 0, 468, 230)

	// Extract ROI
	roiMat := mat.Region(roiRect)
	defer roiMat.Close()

	// Convert to HSV
	hsv := gocv.NewMat()
	defer hsv.Close()
	gocv.CvtColor(roiMat, &hsv, gocv.ColorBGRToHSV)

	// Split HSV channels
	channels := gocv.Split(hsv)
	defer func() {
		for i := range channels {
			channels[i].Close()
		}
	}()

	// Get V channel (index 2)
	vChannel := channels[2]

	// Threshold V channel: V < vThreshold
	mask := gocv.NewMat()
	defer mask.Close()
	gocv.Threshold(vChannel, &mask, float32(vThreshold), 255, gocv.ThresholdBinaryInv)

	// Invert mask
	invertedMask := gocv.NewMat()
	defer invertedMask.Close()
	gocv.BitwiseNot(mask, &invertedMask)

	// Apply Gaussian blur (ensure blur size is odd)
	blurred := gocv.NewMat()
	defer blurred.Close()
	if blurSize%2 == 0 {
		blurSize++
	}
	if blurSize < 1 {
		blurSize = 1
	}
	gocv.GaussianBlur(invertedMask, &blurred, image.Pt(blurSize, blurSize), 0, 0, gocv.BorderDefault)

	// Morphological operations: closing then opening
	if morphWidth < 1 {
		morphWidth = 1
	}
	if morphHeight < 1 {
		morphHeight = 1
	}
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(morphWidth, morphHeight))
	defer kernel.Close()

	// Closing (dilation followed by erosion) - fills small holes
	closed := gocv.NewMat()
	defer closed.Close()
	gocv.MorphologyEx(blurred, &closed, gocv.MorphClose, kernel)

	// Opening (erosion followed by dilation) - removes small noise
	morphed := gocv.NewMat()
	defer morphed.Close()
	gocv.MorphologyEx(closed, &morphed, gocv.MorphOpen, kernel)

	// Find contours (use RetrievalList to find all contours, not just external)
	contours := gocv.FindContours(morphed, gocv.RetrievalList, gocv.ChainApproxSimple)
	defer contours.Close()

	// Filter contours by size and collect rectangles
	var detectedRects []image.Rectangle
	fmt.Printf("\nFound %d contours\n", contours.Size())
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)
		rect := gocv.BoundingRect(contour)

		fmt.Printf("Contour %d: w=%d h=%d (range: w[%d-%d] h[%d-%d])\n",
			i, rect.Dx(), rect.Dy(), minWidth, maxWidth, minHeight, maxHeight)

		// Check if width and height are in range
		if rect.Dx() >= minWidth && rect.Dx() <= maxWidth && rect.Dy() >= minHeight && rect.Dy() <= maxHeight {
			detectedRects = append(detectedRects, rect)
			fmt.Printf("  -> ACCEPTED\n")
		} else {
			fmt.Printf("  -> REJECTED\n")
		}
	}

	// Create result image with detected rectangles
	resultMat := gocv.NewMat()
	defer resultMat.Close()
	gocv.CvtColor(morphed, &resultMat, gocv.ColorGrayToBGR)

	// Draw detected rectangles (red) on result
	for _, rect := range detectedRects {
		gocv.Rectangle(&resultMat, rect, color.RGBA{0, 0, 255, 255}, 2)
	}

	// Add info text to result
	infoText := fmt.Sprintf("Detected: %d status bars", len(detectedRects))
	gocv.PutText(&resultMat, infoText,
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)

	// Convert grayscale images to BGR for consistent display
	maskBGR := gocv.NewMat()
	defer maskBGR.Close()
	gocv.CvtColor(mask, &maskBGR, gocv.ColorGrayToBGR)

	invertedMaskBGR := gocv.NewMat()
	defer invertedMaskBGR.Close()
	gocv.CvtColor(invertedMask, &invertedMaskBGR, gocv.ColorGrayToBGR)

	morphedBGR := gocv.NewMat()
	defer morphedBGR.Close()
	gocv.CvtColor(morphed, &morphedBGR, gocv.ColorGrayToBGR)

	// Vertically stack the 5 images
	combined1 := gocv.NewMat()
	defer combined1.Close()
	gocv.Vconcat(roiMat, maskBGR, &combined1)

	combined2 := gocv.NewMat()
	defer combined2.Close()
	gocv.Vconcat(combined1, invertedMaskBGR, &combined2)

	combined3 := gocv.NewMat()
	defer combined3.Close()
	gocv.Vconcat(combined2, morphedBGR, &combined3)

	final := gocv.NewMat()
	defer final.Close()
	gocv.Vconcat(combined3, resultMat, &final)

	// Show window
	window.IMShow(final)
}

// StatusBarInfo holds information about a detected status bar
type StatusBarInfo struct {
	Rect       image.Rectangle
	FillWidth  int
	Percentage float64
	Type       string // "HP", "MP", or "FP"
}

// detectStatusBars2 detects status bars with the new algorithm
func detectStatusBars2(mat gocv.Mat, window *gocv.Window, printProgress bool) []StatusBarInfo {
	// ROI: (0,0) to (500, 350)
	roiRect := image.Rect(0, 0, 500, 350)
	roiMat := mat.Region(roiRect)
	defer roiMat.Close()

	// Convert to HSV
	hsv := gocv.NewMat()
	defer hsv.Close()
	gocv.CvtColor(roiMat, &hsv, gocv.ColorBGRToHSV)

	// Split HSV channels
	channels := gocv.Split(hsv)
	defer func() {
		for i := range channels {
			channels[i].Close()
		}
	}()

	// Get V channel and threshold V < 80
	vChannel := channels[2]
	mask := gocv.NewMat()
	defer mask.Close()
	gocv.Threshold(vChannel, &mask, 80, 255, gocv.ThresholdBinaryInv)

	// Morphological operations: width=5, height=3
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(5, 3))
	defer kernel.Close()

	closed := gocv.NewMat()
	defer closed.Close()
	gocv.MorphologyEx(mask, &closed, gocv.MorphClose, kernel)

	morphed := gocv.NewMat()
	defer morphed.Close()
	gocv.MorphologyEx(closed, &morphed, gocv.MorphOpen, kernel)

	// Step 1: Find outer frame using RetrievalExternal (width 400-600, height 180-300)
	outerContours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer outerContours.Close()

	var outerFrame image.Rectangle
	for i := 0; i < outerContours.Size(); i++ {
		contour := outerContours.At(i)
		rect := gocv.BoundingRect(contour)
		if rect.Dx() >= 400 && rect.Dx() <= 600 && rect.Dy() >= 180 && rect.Dy() <= 300 {
			outerFrame = rect
			found = true
			fmt.Printf("Found outer frame: w=%d h=%d at (%d,%d)\n", rect.Dx(), rect.Dy(), rect.Min.X, rect.Min.Y)
			break
		}
	}

	if !found {
    if printProgress {
        fmt.Println("Outer frame not found")
		window.IMShow(roiMat)
		return nil
	}

	// Step 2: Invert mask
	invertedMask := gocv.NewMat()
	defer invertedMask.Close()
	gocv.BitwiseNot(morphed, &invertedMask)

	// Extract the outer frame region from inverted mask
	frameMask := invertedMask.Region(outerFrame)
	defer frameMask.Close()

	// Step 3: Find inner contours using RetrievalList
	innerContours := gocv.FindContours(frameMask, gocv.RetrievalList, gocv.ChainApproxSimple)
	defer innerContours.Close()

	// Find avatar (width 80-200, height 100-300)
	var avatarRect image.Rectangle
	var barRects []image.Rectangle

	fmt.Printf("Found %d inner contours\n", innerContours.Size())
	for i := 0; i < innerContours.Size(); i++ {
		contour := innerContours.At(i)
		rect := gocv.BoundingRect(contour)

		// Debug: print all contour sizes
		fmt.Printf("  Contour %d: w=%d h=%d\n", i, rect.Dx(), rect.Dy())

		// Check for avatar
		if rect.Dx() >= 80 && rect.Dx() <= 200 && rect.Dy() >= 100 && rect.Dy() <= 300 {
			avatarRect = rect
			fmt.Printf("    -> Avatar matched\n")
		}

		// Check for status bars (width 100-300, height 5-30)
		if rect.Dx() >= 100 && rect.Dx() <= 300 && rect.Dy() >= 5 && rect.Dy() <= 30 {
			// Adjust to absolute coordinates
			absoluteRect := image.Rect(
				rect.Min.X+outerFrame.Min.X,
				rect.Min.Y+outerFrame.Min.Y,
				rect.Max.X+outerFrame.Min.X,
				rect.Max.Y+outerFrame.Min.Y,
			)
			barRects = append(barRects, absoluteRect)
			fmt.Printf("    -> Bar matched\n")
		}
	}

	// Sort bars from top to bottom
	for i := 0; i < len(barRects); i++ {
		for j := i + 1; j < len(barRects); j++ {
			if barRects[i].Min.Y > barRects[j].Min.Y {
				barRects[i], barRects[j] = barRects[j], barRects[i]
			}
		}
	}

	// Take top 3 bars as HP, MP, FP
	if len(barRects) < 3 {
		fmt.Printf("Not enough bars found: %d\n", len(barRects))
		window.IMShow(roiMat)
		return nil
	}

	var statusBars []StatusBarInfo
	barTypes := []string{"HP", "MP", "FP"}
	hRanges := [][2]int{{160, 180}, {90, 120}, {45, 70}}

	for i := 0; i < 3; i++ {
		barRect := barRects[i]
		barType := barTypes[i]
		hRange := hRanges[i]

		// Extract bar region from HSV
		barROI := hsv.Region(barRect)
		defer barROI.Close()

		// Create mask for the specific color range
		// H: hRange[0]-hRange[1], S: 100-240, V: 100-240
		lower := gocv.NewScalar(float64(hRange[0]/2), 100, 100, 0) // H is in 0-180 range in OpenCV
		upper := gocv.NewScalar(float64(hRange[1]/2), 240, 240, 0)
		colorMask := gocv.NewMat()
		defer colorMask.Close()
		gocv.InRangeWithScalar(barROI, lower, upper, &colorMask)

		// Find the rightmost white pixel to determine fill width
		fillWidth := 0
		for x := barRect.Dx() - 1; x >= 0; x-- {
			hasWhite := false
			for y := 0; y < barRect.Dy(); y++ {
				if colorMask.GetUCharAt(y, x) > 0 {
					hasWhite = true
					break
				}
			}
			if hasWhite {
				fillWidth = x + 1
				break
			}
		}

		percentage := float64(fillWidth) / float64(barRect.Dx()) * 100

		statusBars = append(statusBars, StatusBarInfo{
			Rect:       barRect,
			FillWidth:  fillWidth,
			Percentage: percentage,
			Type:       barType,
		})

		fmt.Printf("%s: width=%d fill=%d (%.1f%%)\n", barType, barRect.Dx(), fillWidth, percentage)
	}

	// === Prepare visualization images ===

	// 1. Original ROI
	step1Original := roiMat.Clone()
	defer step1Original.Close()

	// 2. Binary mask (V < 80)
	step2Binary := gocv.NewMat()
	defer step2Binary.Close()
	gocv.CvtColor(mask, &step2Binary, gocv.ColorGrayToBGR)

	// 3. Morphed
	step3Morphed := gocv.NewMat()
	defer step3Morphed.Close()
	gocv.CvtColor(morphed, &step3Morphed, gocv.ColorGrayToBGR)

	// 4. Outer frame marked
	step4OuterFrame := roiMat.Clone()
	defer step4OuterFrame.Close()
	gocv.Rectangle(&step4OuterFrame, outerFrame, color.RGBA{0, 255, 0, 255}, 2)
	gocv.PutText(&step4OuterFrame, "Outer Frame",
		image.Pt(outerFrame.Min.X, outerFrame.Min.Y-5),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)

	// 5. Inverted mask
	step5Inverted := gocv.NewMat()
	defer step5Inverted.Close()
	gocv.CvtColor(invertedMask, &step5Inverted, gocv.ColorGrayToBGR)

	// 6. Avatar marked
	step6Avatar := roiMat.Clone()
	defer step6Avatar.Close()
	if avatarRect.Dx() > 0 {
		absoluteAvatar := image.Rect(
			avatarRect.Min.X+outerFrame.Min.X,
			avatarRect.Min.Y+outerFrame.Min.Y,
			avatarRect.Max.X+outerFrame.Min.X,
			avatarRect.Max.Y+outerFrame.Min.Y,
		)
		gocv.Rectangle(&step6Avatar, absoluteAvatar, color.RGBA{255, 0, 0, 255}, 2)
		gocv.PutText(&step6Avatar, "Avatar",
			image.Pt(absoluteAvatar.Min.X, absoluteAvatar.Min.Y-5),
			gocv.FontHersheyPlain, 1.2, color.RGBA{255, 0, 0, 255}, 2)
	}

	// 7. HP, MP, FP marked
	step7Bars := roiMat.Clone()
	defer step7Bars.Close()
	colors := []color.RGBA{
		{255, 0, 0, 255},   // HP - Red
		{0, 0, 255, 255},   // MP - Blue
		{0, 255, 0, 255},   // FP - Green
	}

	for i, bar := range statusBars {
		gocv.Rectangle(&step7Bars, bar.Rect, colors[i], 2)
		text := fmt.Sprintf("%s: %.1f%%", bar.Type, bar.Percentage)
		gocv.PutText(&step7Bars, text,
			image.Pt(bar.Rect.Min.X, bar.Rect.Min.Y-5),
			gocv.FontHersheyPlain, 1.0, colors[i], 2)
	}

	// Vertically concatenate all steps
	combined1 := gocv.NewMat()
	defer combined1.Close()
	gocv.Vconcat(step1Original, step2Binary, &combined1)

	combined2 := gocv.NewMat()
	defer combined2.Close()
	gocv.Vconcat(combined1, step3Morphed, &combined2)

	combined3 := gocv.NewMat()
	defer combined3.Close()
	gocv.Vconcat(combined2, step4OuterFrame, &combined3)

	combined4 := gocv.NewMat()
	defer combined4.Close()
	gocv.Vconcat(combined3, step5Inverted, &combined4)

	combined5 := gocv.NewMat()
	defer combined5.Close()
	gocv.Vconcat(combined4, step6Avatar, &combined5)

	final := gocv.NewMat()
	defer final.Close()
	gocv.Vconcat(combined5, step7Bars, &final)

	window.IMShow(final)

	return statusBars
}

// Parameters holds all detection parameters
type Parameters struct {
	VThreshold  int
	BlurSize    int
	MorphWidth  int
	MorphHeight int
	MinWidth    int
	MaxWidth    int
	MinHeight   int
	MaxHeight   int
}

// setupWindows creates display and control windows with trackbars
func setupWindows() (*gocv.Window, *gocv.Window, *Parameters) {
	window := gocv.NewWindow("Status Bar Detection")
	controlWindow := gocv.NewWindow("Controls")

	params := &Parameters{
		VThreshold:  80,
		BlurSize:    5,
		MorphWidth:  15,
		MorphHeight: 3,
		MinWidth:    100,
		MaxWidth:    300,
		MinHeight:   10,
		MaxHeight:   40,
	}

	controlWindow.CreateTrackbarWithValue("V Threshold", &params.VThreshold, 255)
	controlWindow.CreateTrackbarWithValue("Blur Size", &params.BlurSize, 31)
	controlWindow.CreateTrackbarWithValue("Morph Width", &params.MorphWidth, 50)
	controlWindow.CreateTrackbarWithValue("Morph Height", &params.MorphHeight, 20)
	controlWindow.CreateTrackbarWithValue("Min Width", &params.MinWidth, 500)
	controlWindow.CreateTrackbarWithValue("Max Width", &params.MaxWidth, 500)
	controlWindow.CreateTrackbarWithValue("Min Height", &params.MinHeight, 500)
	controlWindow.CreateTrackbarWithValue("Max Height", &params.MaxHeight, 500)

	return window, controlWindow, params
}

// runDetection1 - Old detection algorithm with trackbars
func runDetection1(useStaticImage bool, staticMat gocv.Mat, browser *DebugBrowser, statusImagePath string) {
	// Setup windows and parameters
	window, controlWindow, params := setupWindows()
	defer window.Close()
	defer controlWindow.Close()

	// Wait for first frame if using browser
	var mat gocv.Mat
	matInitialized := false
	var originalMat gocv.Mat
	var err error

	if !useStaticImage {
		for i := 0; i < 100; i++ {
			if frame, ok := browser.GetFrame(); ok {
				mat, err = gocv.ImageToMatRGB(frame)
				if err != nil {
					fmt.Printf("Failed to convert image: %v\n", err)
					return
				}
				matInitialized = true
				originalMat = mat.Clone()
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if !matInitialized {
			fmt.Println("Failed to get frame")
			return
		}
	}

	fmt.Println("Controls:")
	fmt.Println("  's': Save current frame to status.jpeg (browser mode only)")
	fmt.Println("  'q' or ESC: Quit")

	for {
		if useStaticImage {
			mat = staticMat
		} else {
			if frame, ok := browser.GetFrame(); ok {
				if matInitialized {
					mat.Close()
					originalMat.Close()
				}
				mat, err = gocv.ImageToMatRGB(frame)
				if err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				matInitialized = true
				originalMat = mat.Clone()
			}

			if !matInitialized {
				key := window.WaitKey(100)
				if key == 'q' || key == 27 {
					return
				}
				continue
			}
		}

		// Detect and display status bars
		detectStatusBars(mat, window, params.VThreshold, params.BlurSize, params.MorphWidth, params.MorphHeight, params.MinWidth, params.MaxWidth, params.MinHeight, params.MaxHeight)

		key := window.WaitKey(100)
		if key == 'q' || key == 27 {
			if matInitialized && !useStaticImage {
				mat.Close()
				originalMat.Close()
			}
			return
		} else if key == 's' && !useStaticImage {
			success := gocv.IMWrite(statusImagePath, originalMat)
			if success {
				fmt.Println("Saved frame to status.jpeg")
			} else {
				fmt.Println("Failed to save frame")
			}
		}
	}
}

// runDetection2 - New detection algorithm
func runDetection2(useStaticImage bool, staticMat gocv.Mat, browser *DebugBrowser, statusImagePath string) {
	// Create display window
	window := gocv.NewWindow("Status Bar Detection")
	defer window.Close()

	// Wait for first frame if using browser
	var mat gocv.Mat
	matInitialized := false
	var originalMat gocv.Mat
	var err error

	if !useStaticImage {
		for i := 0; i < 100; i++ {
			if frame, ok := browser.GetFrame(); ok {
				mat, err = gocv.ImageToMatRGB(frame)
				if err != nil {
					fmt.Printf("Failed to convert image: %v\n", err)
					return
				}
				matInitialized = true
				originalMat = mat.Clone()
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if !matInitialized {
			fmt.Println("Failed to get frame")
			return
		}
	}

	fmt.Println("Controls:")
	fmt.Println("  's': Save current frame to status.jpeg (browser mode only)")
	fmt.Println("  'q' or ESC: Quit")

	for {
		if useStaticImage {
			mat = staticMat
		} else {
			if frame, ok := browser.GetFrame(); ok {
				if matInitialized {
					mat.Close()
					originalMat.Close()
				}
				mat, err = gocv.ImageToMatRGB(frame)
				if err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				matInitialized = true
				originalMat = mat.Clone()
			}

			if !matInitialized {
				key := window.WaitKey(100)
				if key == 'q' || key == 27 {
					return
				}
				continue
			}
		}

		// Detect and display status bars
		statusBars := detectStatusBars2(mat, window)
		if statusBars != nil {
			for _, bar := range statusBars {
				fmt.Printf("%s: %.1f%% (fill: %d/%d)\n", bar.Type, bar.Percentage, bar.FillWidth, bar.Rect.Dx())
			}
		}

		key := window.WaitKey(100)
		if key == 'q' || key == 27 {
			if matInitialized && !useStaticImage {
				mat.Close()
				originalMat.Close()
			}
			return
		} else if key == 's' && !useStaticImage {
			success := gocv.IMWrite(statusImagePath, originalMat)
			if success {
				fmt.Println("Saved frame to status.jpeg")
			} else {
				fmt.Println("Failed to save frame")
			}
		}
	}
}

func main() {
	runtime.LockOSThread()

	fmt.Println("=== Status Bar Detection ===")

	// Check if status.jpeg exists
	statusImagePath := "status.jpeg"
	useStaticImage := false
	_, err := os.Stat(statusImagePath)
	if err == nil {
		useStaticImage = true
		fmt.Println("Found status.jpeg, using static image")
	} else {
		fmt.Println("status.jpeg not found, opening browser")
	}

	var browser *DebugBrowser
	var staticMat gocv.Mat

	if useStaticImage {
		// Load static image
		staticMat = gocv.IMRead(statusImagePath, gocv.IMReadColor)
		if staticMat.Empty() {
			fmt.Println("Failed to load status.jpeg")
			return
		}
		defer staticMat.Close()
	} else {
		// Load cookies and start browser
		cookies, err := loadCookies("cookie.json")
		if err != nil {
			fmt.Printf("Warning: failed to load cookies: %v\n", err)
			cookies = make([]Cookie, 0)
		}

		browser = NewDebugBrowser()
		defer browser.Stop()

		err = browser.Start(cookies)
		if err != nil {
			fmt.Printf("Failed to start browser: %v\n", err)
			return
		}

		time.Sleep(5 * time.Second)
	}

	// Use detection algorithm 2 (comment/uncomment to switch)
	// runDetection1(useStaticImage, staticMat, browser, statusImagePath)
	runDetection2(useStaticImage, staticMat, browser, statusImagePath)
}
