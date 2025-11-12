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

	// Start cookie auto-save goroutine
	go b.autoSaveCookies("cookie.json", 2*time.Minute)

	return nil
}

// autoSaveCookies saves cookies periodically in a goroutine
func (b *DebugBrowser) autoSaveCookies(cookiePath string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if b.ctx == nil || b.ctx.Err() != nil {
				// Browser context is closed, stop saving
				return
			}
			err := b.SaveCookies(cookiePath)
			if err != nil {
				fmt.Printf("Auto-save cookies failed: %v\n", err)
			}
		case <-b.ctx.Done():
			// Browser stopped, exit goroutine
			return
		}
	}
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

// SaveCookies saves browser cookies to cookie.json file
func (b *DebugBrowser) SaveCookies(cookiePath string) error {
	if b.ctx == nil || b.ctx.Err() != nil {
		return fmt.Errorf("browser context is invalid")
	}

	var cookies []*network.Cookie
	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		return fmt.Errorf("failed to get cookies: %w", err)
	}

	// Convert to Cookie format
	cookieList := make([]Cookie, len(cookies))
	for i, c := range cookies {
		cookieList[i] = Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		}
	}

	// Save to file
	data, err := json.Marshal(cookieList)
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	err = os.WriteFile(cookiePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write cookie file: %w", err)
	}

	fmt.Printf("Saved %d cookies to %s\n", len(cookies), cookiePath)
	return nil
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

// HSVRange holds HSV color range parameters for a status bar
type HSVRange struct {
	HMin int
	HMax int
	SMin int
	SMax int
	VMin int
	VMax int
}

// MorphParams holds morphology parameters
type MorphParams struct {
	UseAdaptive    int // 0 = fixed threshold, 1 = adaptive threshold
	VThreshold     int // Fixed threshold value
	AdaptiveMethod int // 0 = Mean, 1 = Gaussian
	BlockSize      int // Adaptive block size (must be odd)
	CValue         int // Adaptive C value
	MorphWidth     int
	MorphHeight    int
}

// detectStatusBars2 detects status bars with the new algorithm
func detectStatusBars2(mat gocv.Mat, windowMorph *gocv.Window, windowFrame *gocv.Window, windowBars *gocv.Window, windowHP *gocv.Window, windowMP *gocv.Window, windowFP *gocv.Window, morphParams *MorphParams, hsvRanges [3]*HSVRange, printProgress bool) []StatusBarInfo {
	// Helper function to append image to display - auto-converts to BGR if needed
	appendImage := func(display *gocv.Mat, img gocv.Mat, isHSV bool) {
		var bgrImg gocv.Mat

		// Auto-convert to BGR based on channels
		if img.Channels() == 1 {
			// Grayscale image
			bgrImg = gocv.NewMat()
			gocv.CvtColor(img, &bgrImg, gocv.ColorGrayToBGR)
			defer bgrImg.Close()
		} else if isHSV {
			// HSV image
			bgrImg = gocv.NewMat()
			gocv.CvtColor(img, &bgrImg, gocv.ColorHSVToBGR)
			defer bgrImg.Close()
		} else {
			// Already BGR
			bgrImg = img
		}

		temp := gocv.NewMat()
		defer temp.Close()
		gocv.Hconcat(*display, bgrImg, &temp)
		display.Close()
		*display = temp.Clone()
	}

	// === Step 1: Extract ROI (0,0) to (500,350) ===
	if printProgress {
		fmt.Printf("\n=== Starting detectStatusBars2 ===\n")
		fmt.Printf("Input mat size: %dx%d, channels: %d\n", mat.Cols(), mat.Rows(), mat.Channels())
	}

	img_roi := mat.Region(image.Rect(0, 0, 500, 350))
	defer img_roi.Close()

	if printProgress {
		fmt.Printf("img_roi size: %dx%d, channels: %d, empty: %v\n",
			img_roi.Cols(), img_roi.Rows(), img_roi.Channels(), img_roi.Empty())
	}

	// Start building display
	morphDisplay := img_roi.Clone()
	defer morphDisplay.Close()

	// === Step 2: Convert to HSV ===
	img_hsv := gocv.NewMat()
	defer img_hsv.Close()
	gocv.CvtColor(img_roi, &img_hsv, gocv.ColorBGRToHSV)
	appendImage(&morphDisplay, img_hsv, true) // true = HSV

	// === Step 3: Get V channel and threshold ===
	channels := gocv.Split(img_hsv)
	defer func() {
		for i := range channels {
			channels[i].Close()
		}
	}()
	vChannel := channels[2]

	img_v := gocv.NewMat()
	defer img_v.Close()

	if morphParams.UseAdaptive == 0 {
		// Fixed threshold
		gocv.Threshold(vChannel, &img_v, float32(morphParams.VThreshold), 255, gocv.ThresholdBinaryInv)
	} else {
		// Adaptive threshold
		// Ensure block size is odd and at least 3
		blockSize := morphParams.BlockSize
		if blockSize < 3 {
			blockSize = 3
		}
		if blockSize%2 == 0 {
			blockSize++
		}

		// Choose adaptive method
		var adaptiveMethod gocv.AdaptiveThresholdType
		if morphParams.AdaptiveMethod == 0 {
			adaptiveMethod = gocv.AdaptiveThresholdMean
		} else {
			adaptiveMethod = gocv.AdaptiveThresholdGaussian
		}

		// Apply adaptive threshold (note: outputs white for BELOW threshold with Inv)
		gocv.AdaptiveThreshold(vChannel, &img_v, 255, adaptiveMethod, gocv.ThresholdBinaryInv, blockSize, float32(morphParams.CValue))
	}
	appendImage(&morphDisplay, img_v, false)

	// === Step 4: Invert ===
	img_vr := gocv.NewMat()
	defer img_vr.Close()
	gocv.BitwiseNot(img_v, &img_vr)
	appendImage(&morphDisplay, img_vr, false)

	// === Step 5: Morphological operations ===
	// Ensure kernel dimensions are at least 1
	kernelWidth := morphParams.MorphWidth
	if kernelWidth < 1 {
		kernelWidth = 1
	}
	kernelHeight := morphParams.MorphHeight
	if kernelHeight < 1 {
		kernelHeight = 1
	}
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(kernelWidth, kernelHeight))
	defer kernel.Close()

	closed := gocv.NewMat()
	defer closed.Close()
	gocv.MorphologyEx(img_vr, &closed, gocv.MorphClose, kernel)
	appendImage(&morphDisplay, closed, false)

	img_vrm := gocv.NewMat()
	defer img_vrm.Close()
	gocv.MorphologyEx(closed, &img_vrm, gocv.MorphOpen, kernel)
	appendImage(&morphDisplay, img_vrm, false)

	// === Step 6: Invert again for outer frame detection ===
	img_vrmr := gocv.NewMat()
	defer img_vrmr.Close()
	gocv.BitwiseNot(img_vrm, &img_vrmr)
	appendImage(&morphDisplay, img_vrmr, false)

	// Display morphology window
	windowMorph.IMShow(morphDisplay)

	// === Step 7: Detect outer frame using img_vrmr (width: 400-600, height: 180-300) ===
	outerContours := gocv.FindContours(img_vrmr, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer outerContours.Close()

	var img_outline image.Rectangle
	found := false
	for i := 0; i < outerContours.Size(); i++ {
		contour := outerContours.At(i)
		rect := gocv.BoundingRect(contour)
		if rect.Dx() >= 400 && rect.Dx() <= 600 && rect.Dy() >= 180 && rect.Dy() <= 300 {
			img_outline = rect
			found = true
			if printProgress {
				fmt.Printf("Found outer frame: w=%d h=%d at (%d,%d)\n", rect.Dx(), rect.Dy(), rect.Min.X, rect.Min.Y)
			}
			break
		}
	}

	// Find avatar and bars only if outer frame was found
	var img_avatar image.Rectangle
	var img_bars []image.Rectangle
	var statusBars []StatusBarInfo

	if !found {
		if printProgress {
			fmt.Println("Outer frame not found")
		}
		frameDisplay := img_roi.Clone()
		defer frameDisplay.Close()
		windowFrame.IMShow(frameDisplay)
	} else {
		// Extract the outer frame region from img_vrm (for status bars and avatar)
		frameMask := img_vrm.Region(img_outline)
		defer frameMask.Close()

		// === Step 8: Find avatar using img_vrm within img_outline ===
		innerContours := gocv.FindContours(frameMask, gocv.RetrievalList, gocv.ChainApproxSimple)
		defer innerContours.Close()

		if printProgress {
			fmt.Printf("Found %d inner contours\n", innerContours.Size())
		}

		// First pass: find avatar only
		for i := 0; i < innerContours.Size(); i++ {
			contour := innerContours.At(i)
			rect := gocv.BoundingRect(contour)

			if printProgress {
				fmt.Printf("  Contour %d: w=%d h=%d\n", i, rect.Dx(), rect.Dy())
			}

			// Check for avatar (width: 80-200, height: 100-300)
			if rect.Dx() >= 80 && rect.Dx() <= 200 && rect.Dy() >= 100 && rect.Dy() <= 300 {
				img_avatar = rect
				if printProgress {
					fmt.Printf("    -> Avatar matched\n")
				}
				break
			}
		}

		// === Step 9: Define img_bararea and find bars ===
		var img_bararea image.Rectangle
		var img_bararea_abs image.Rectangle
		if img_avatar.Dx() > 0 {
			// Bar area: to the right of avatar, same Y range as avatar (relative to img_outline)
			img_bararea = image.Rect(
				img_avatar.Max.X, // Start from right edge of avatar
				img_avatar.Min.Y, // Same top as avatar
				img_outline.Dx(), // Extend to right edge of outline
				img_avatar.Max.Y, // Same bottom as avatar
			)

			// Convert img_bararea to absolute coordinates (relative to img_roi)
			img_bararea_abs = image.Rect(
				img_bararea.Min.X+img_outline.Min.X,
				img_bararea.Min.Y+img_outline.Min.Y,
				img_bararea.Max.X+img_outline.Min.X,
				img_bararea.Max.Y+img_outline.Min.Y,
			)

			// Extract bar area from img_vrm using absolute coordinates
			barAreaMask := img_vrm.Region(img_bararea_abs)
			defer barAreaMask.Close()

			// Find contours in bar area
			barContours := gocv.FindContours(barAreaMask, gocv.RetrievalList, gocv.ChainApproxSimple)
			defer barContours.Close()

			if printProgress {
				fmt.Printf("Found %d contours in bar area\n", barContours.Size())
			}

			// Second pass: find bars in bar area
			for i := 0; i < barContours.Size(); i++ {
				contour := barContours.At(i)
				rect := gocv.BoundingRect(contour)

				if printProgress {
					fmt.Printf("  Bar contour %d: w=%d h=%d\n", i, rect.Dx(), rect.Dy())
				}

				// Check for status bars (width: 100-300, height: 5-30)
				if rect.Dx() >= 100 && rect.Dx() <= 300 && rect.Dy() >= 5 && rect.Dy() <= 30 {
					// Adjust to absolute coordinates (relative to img_roi)
					// rect is relative to barAreaMask, which starts at img_bararea_abs
					absoluteRect := image.Rect(
						rect.Min.X+img_bararea_abs.Min.X,
						rect.Min.Y+img_bararea_abs.Min.Y,
						rect.Max.X+img_bararea_abs.Min.X,
						rect.Max.Y+img_bararea_abs.Min.Y,
					)
					img_bars = append(img_bars, absoluteRect)
					if printProgress {
						fmt.Printf("    -> Bar matched\n")
					}
				}
			}
		} else {
			if printProgress {
				fmt.Println("Avatar not found, skipping bar detection")
			}
		}

		// === Window 2: Frame & Avatar Detection ===
		frameDisplay := gocv.NewMat()
		defer frameDisplay.Close()

		// 1. Original ROI with label
		originalROI := img_roi.Clone()
		defer originalROI.Close()
		gocv.PutText(&originalROI, "Original",
			image.Pt(10, 25),
			gocv.FontHersheyPlain, 1.5, color.RGBA{255, 255, 255, 255}, 2)
		frameDisplay = originalROI.Clone()

		// 2. Mask (img_vrmr) with label
		maskForFrame := gocv.NewMat()
		defer maskForFrame.Close()
		gocv.CvtColor(img_vrmr, &maskForFrame, gocv.ColorGrayToBGR)
		gocv.PutText(&maskForFrame, "Mask",
			image.Pt(10, 25),
			gocv.FontHersheyPlain, 1.5, color.RGBA{255, 255, 255, 255}, 2)
		appendImage(&frameDisplay, maskForFrame, false)

		// 3. Result with all annotations
		resultFrame := img_roi.Clone()
		defer resultFrame.Close()

		// Draw outer frame
		gocv.Rectangle(&resultFrame, img_outline, color.RGBA{0, 255, 0, 255}, 2)
		gocv.PutText(&resultFrame, "Frame",
			image.Pt(img_outline.Min.X+5, img_outline.Min.Y+20),
			gocv.FontHersheyPlain, 1.0, color.RGBA{0, 255, 0, 255}, 2)

		// Draw avatar
		if img_avatar.Dx() > 0 {
			absoluteAvatar := image.Rect(
				img_avatar.Min.X+img_outline.Min.X,
				img_avatar.Min.Y+img_outline.Min.Y,
				img_avatar.Max.X+img_outline.Min.X,
				img_avatar.Max.Y+img_outline.Min.Y,
			)
			gocv.Rectangle(&resultFrame, absoluteAvatar, color.RGBA{255, 0, 0, 255}, 2)
			gocv.PutText(&resultFrame, "Avatar",
				image.Pt(absoluteAvatar.Min.X+5, absoluteAvatar.Min.Y+20),
				gocv.FontHersheyPlain, 1.0, color.RGBA{255, 0, 0, 255}, 2)
		}

		// Draw bar area
		if img_bararea_abs.Dx() > 0 {
			gocv.Rectangle(&resultFrame, img_bararea_abs, color.RGBA{255, 255, 0, 255}, 2)
			gocv.PutText(&resultFrame, "Bar Area",
				image.Pt(img_bararea_abs.Min.X+5, img_bararea_abs.Min.Y+15),
				gocv.FontHersheyPlain, 0.8, color.RGBA{255, 255, 0, 255}, 1)
		}

		// Draw all bars
		for i, barRect := range img_bars {
			gocv.Rectangle(&resultFrame, barRect, color.RGBA{0, 255, 255, 255}, 2)
			gocv.PutText(&resultFrame, fmt.Sprintf("Bar%d", i+1),
				image.Pt(barRect.Min.X+5, barRect.Min.Y+12),
				gocv.FontHersheyPlain, 0.8, color.RGBA{0, 255, 255, 255}, 1)
		}

		gocv.PutText(&resultFrame, "Result",
			image.Pt(10, 25),
			gocv.FontHersheyPlain, 1.5, color.RGBA{255, 255, 255, 255}, 2)
		appendImage(&frameDisplay, resultFrame, false)

		windowFrame.IMShow(frameDisplay)

		// Sort bars from top to bottom
		for i := 0; i < len(img_bars); i++ {
			for j := i + 1; j < len(img_bars); j++ {
				if img_bars[i].Min.Y > img_bars[j].Min.Y {
					img_bars[i], img_bars[j] = img_bars[j], img_bars[i]
				}
			}
		}
	}

	// === Step 10-12: Process HP, MP, FP bars ===
	var img_hp_mask, img_mp_mask, img_fp_mask gocv.Mat
	if found && len(img_bars) >= 3 {
		barTypes := []string{"HP", "MP", "FP"}
		masks := []*gocv.Mat{&img_hp_mask, &img_mp_mask, &img_fp_mask}

		for i := 0; i < 3; i++ {
			barRect := img_bars[i]
			barType := barTypes[i]
			hsvRange := hsvRanges[i]

			// Extract bar region from HSV
			barROI := img_hsv.Region(barRect)
			defer barROI.Close()

			// Create mask for the specific color range using trackbar values
			// Trackbar range is already 0-180 for H (OpenCV range), no need to divide
			lower := gocv.NewScalar(float64(hsvRange.HMin), float64(hsvRange.SMin), float64(hsvRange.VMin), 0)
			upper := gocv.NewScalar(float64(hsvRange.HMax), float64(hsvRange.SMax), float64(hsvRange.VMax), 0)
			*masks[i] = gocv.NewMat()
			defer masks[i].Close()
			gocv.InRangeWithScalar(barROI, lower, upper, masks[i])

			// Find the rightmost white pixel to determine fill width
			fillWidth := 0
			for x := barRect.Dx() - 1; x >= 0; x-- {
				hasWhite := false
				for y := 0; y < barRect.Dy(); y++ {
					if masks[i].GetUCharAt(y, x) > 0 {
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

			if printProgress {
				fmt.Printf("%s: width=%d fill=%d (%.1f%%)\n", barType, barRect.Dx(), fillWidth, percentage)
			}
		}
	}

	// === Window 3: Status Bars ===
	barsDisplay := img_roi.Clone()
	defer barsDisplay.Close()

	if printProgress {
		fmt.Printf("\n=== Step 3: Status Bars ===\n")
		fmt.Printf("barsDisplay size: %dx%d, channels: %d, empty: %v\n",
			barsDisplay.Cols(), barsDisplay.Rows(), barsDisplay.Channels(), barsDisplay.Empty())
		fmt.Printf("statusBars count: %d\n", len(statusBars))
	}

	colors := []color.RGBA{
		{255, 0, 0, 255}, // HP - Red
		{0, 0, 255, 255}, // MP - Blue
		{0, 255, 0, 255}, // FP - Green
	}

	for i, bar := range statusBars {
		gocv.Rectangle(&barsDisplay, bar.Rect, colors[i], 2)
		text := fmt.Sprintf("%s: %.1f%%", bar.Type, bar.Percentage)
		gocv.PutText(&barsDisplay, text,
			image.Pt(bar.Rect.Min.X, bar.Rect.Min.Y-5),
			gocv.FontHersheyPlain, 1.0, colors[i], 2)
	}

	// Display main status bars window
	windowBars.IMShow(barsDisplay)

	// Display HP, MP, FP in horizontal layout: Original | Mask | Annotated
	if len(statusBars) >= 3 {
		barMasks := []gocv.Mat{img_hp_mask, img_mp_mask, img_fp_mask}
		windows := []*gocv.Window{windowHP, windowMP, windowFP}

		for i := 0; i < 3; i++ {
			bar := statusBars[i]
			display := gocv.NewMat()
			defer display.Close()

			// 1. Original image with label
			barOriginal := img_roi.Region(bar.Rect)
			defer barOriginal.Close()
			original := barOriginal.Clone()
			defer original.Close()
			gocv.PutText(&original, "Original",
				image.Pt(5, 15),
				gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 255, 255}, 1)
			display = original.Clone()

			// 2. HSV image with label
			barHSV := img_hsv.Region(bar.Rect)
			defer barHSV.Close()
			hsvDisplay := gocv.NewMat()
			defer hsvDisplay.Close()
			gocv.CvtColor(barHSV, &hsvDisplay, gocv.ColorHSVToBGR)
			gocv.PutText(&hsvDisplay, "HSV",
				image.Pt(5, 15),
				gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 255, 255}, 1)
			appendImage(&display, hsvDisplay, false)

			// 3. Mask with label (convert to BGR)
			mask := barMasks[i]
			maskDisplay := gocv.NewMat()
			defer maskDisplay.Close()
			if !mask.Empty() {
				gocv.CvtColor(mask, &maskDisplay, gocv.ColorGrayToBGR)
			} else {
				// Show empty/black image if mask is empty
				maskDisplay = gocv.NewMatWithSize(bar.Rect.Dy(), bar.Rect.Dx(), gocv.MatTypeCV8UC3)
			}
			gocv.PutText(&maskDisplay, "Mask",
				image.Pt(5, 15),
				gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 255, 255}, 1)
			appendImage(&display, maskDisplay, false)

			// 4. Annotated result with label
			annotated := barOriginal.Clone()
			defer annotated.Close()
			// Draw fill width line
			if bar.FillWidth > 0 {
				gocv.Line(&annotated,
					image.Pt(bar.FillWidth-1, 0),
					image.Pt(bar.FillWidth-1, bar.Rect.Dy()),
					color.RGBA{0, 255, 0, 255}, 2)
			}
			// Draw labels
			gocv.PutText(&annotated, "Result",
				image.Pt(5, 15),
				gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 255, 255}, 1)
			text := fmt.Sprintf("%.1f%%", bar.Percentage)
			gocv.PutText(&annotated, text,
				image.Pt(5, bar.Rect.Dy()-5),
				gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 255, 255}, 1)
			appendImage(&display, annotated, false)

			if printProgress {
				fmt.Printf("Displaying %s: %dx%d (mask empty: %v)\n", bar.Type, display.Cols(), display.Rows(), mask.Empty())
			}
			windows[i].IMShow(display)
		}
	}

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
	// Create six display windows
	windowMorph := gocv.NewWindow("Step 1: Morphology")
	windowFrame := gocv.NewWindow("Step 2: Frame & Avatar Detection")
	windowBars := gocv.NewWindow("Step 3: Status Bars")
	windowHP := gocv.NewWindow("HP Mask")
	windowMP := gocv.NewWindow("MP Mask")
	windowFP := gocv.NewWindow("FP Mask")
	defer windowMorph.Close()
	defer windowFrame.Close()
	defer windowBars.Close()
	defer windowHP.Close()
	defer windowMP.Close()
	defer windowFP.Close()

	// Create morphology parameters
	morphParams := &MorphParams{
		UseAdaptive:    0,   // 0 = fixed, 1 = adaptive
		VThreshold:     80,  // Fixed threshold value
		AdaptiveMethod: 1,   // 0 = Mean, 1 = Gaussian
		BlockSize:      100, // Adaptive block size
		CValue:         2,   // Adaptive C value
		MorphWidth:     5,
		MorphHeight:    3,
	}

	// Create trackbars for morphology parameters
	windowMorph.CreateTrackbarWithValue("Use Adaptive (0/1)", &morphParams.UseAdaptive, 1)
	windowMorph.CreateTrackbarWithValue("V Threshold", &morphParams.VThreshold, 255)
	windowMorph.CreateTrackbarWithValue("Adaptive Method (0/1)", &morphParams.AdaptiveMethod, 1)
	windowMorph.CreateTrackbarWithValue("Block Size", &morphParams.BlockSize, 199)
	windowMorph.CreateTrackbarWithValue("C Value", &morphParams.CValue, 50)
	windowMorph.CreateTrackbarWithValue("Morph Width", &morphParams.MorphWidth, 50)
	windowMorph.CreateTrackbarWithValue("Morph Height", &morphParams.MorphHeight, 50)

	// Create HSV range parameters for HP, MP, FP
	hpRange := &HSVRange{HMin: 160, HMax: 180, SMin: 100, SMax: 240, VMin: 100, VMax: 240}
	mpRange := &HSVRange{HMin: 90, HMax: 120, SMin: 100, SMax: 240, VMin: 100, VMax: 240}
	fpRange := &HSVRange{HMin: 45, HMax: 70, SMin: 100, SMax: 240, VMin: 100, VMax: 240}
	hsvRanges := [3]*HSVRange{hpRange, mpRange, fpRange}

	// Create trackbars for HP
	windowHP.CreateTrackbarWithValue("H Min", &hpRange.HMin, 180)
	windowHP.CreateTrackbarWithValue("H Max", &hpRange.HMax, 180)
	windowHP.CreateTrackbarWithValue("S Min", &hpRange.SMin, 255)
	windowHP.CreateTrackbarWithValue("S Max", &hpRange.SMax, 255)
	windowHP.CreateTrackbarWithValue("V Min", &hpRange.VMin, 255)
	windowHP.CreateTrackbarWithValue("V Max", &hpRange.VMax, 255)

	// Create trackbars for MP
	windowMP.CreateTrackbarWithValue("H Min", &mpRange.HMin, 180)
	windowMP.CreateTrackbarWithValue("H Max", &mpRange.HMax, 180)
	windowMP.CreateTrackbarWithValue("S Min", &mpRange.SMin, 255)
	windowMP.CreateTrackbarWithValue("S Max", &mpRange.SMax, 255)
	windowMP.CreateTrackbarWithValue("V Min", &mpRange.VMin, 255)
	windowMP.CreateTrackbarWithValue("V Max", &mpRange.VMax, 255)

	// Create trackbars for FP
	windowFP.CreateTrackbarWithValue("H Min", &fpRange.HMin, 180)
	windowFP.CreateTrackbarWithValue("H Max", &fpRange.HMax, 180)
	windowFP.CreateTrackbarWithValue("S Min", &fpRange.SMin, 255)
	windowFP.CreateTrackbarWithValue("S Max", &fpRange.SMax, 255)
	windowFP.CreateTrackbarWithValue("V Min", &fpRange.VMin, 255)
	windowFP.CreateTrackbarWithValue("V Max", &fpRange.VMax, 255)

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
				key := windowMorph.WaitKey(100)
				if key == 'q' || key == 27 {
					return
				}
				continue
			}
		}

		// Detect and display status bars
		statusBars := detectStatusBars2(mat, windowMorph, windowFrame, windowBars, windowHP, windowMP, windowFP, morphParams, hsvRanges, true)
		if statusBars != nil {
			for _, bar := range statusBars {
				fmt.Printf("%s: %.1f%% (fill: %d/%d)\n", bar.Type, bar.Percentage, bar.FillWidth, bar.Rect.Dx())
			}
		}

		key := windowMorph.WaitKey(100)
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

// BarROI represents the region of interest for a status bar
type BarROI struct {
	MinX  int `json:"minX"`
	MinY  int `json:"minY"`
	MaxX  int `json:"maxX"`
	MaxY  int `json:"maxY"`
	Width int `json:"width"`
}

// BarStatus represents the status of a single bar (HP/MP/FP)
type BarStatus struct {
	Value int    `json:"value"` // Percentage (0-100)
	Width int    `json:"width"` // Fill width in pixels
	ROI   BarROI `json:"roi"`   // Detection region
}

// detectMyStatus stores the detection state across frames
type detectMyStatus struct {
	Open  bool      `json:"open"`
	Count int       `json:"count"`
	Retry int       `json:"retry"`
	HP    BarStatus `json:"hp"`
	MP    BarStatus `json:"mp"`
	FP    BarStatus `json:"fp"`
}

// detectStatusBars3 - Simplified version that returns annotated image only
func detectStatusBars3(mat gocv.Mat, status *detectMyStatus) gocv.Mat {
	// detectBarsValue detects HP/MP/FP fill width using HSV color detection
	// Returns the fill width for HP, MP, FP
	detectBarsValue := func(img_hsv gocv.Mat, status *detectMyStatus) (hpWidth, mpWidth, fpWidth int) {
		// Define H ranges for HP, MP, FP
		hRanges := [][2]int{{160, 180}, {90, 120}, {45, 70}}
		barStatuses := []*BarStatus{&status.HP, &status.MP, &status.FP}
		widths := []int{0, 0, 0}

		for i := 0; i < 3; i++ {
			barStatus := barStatuses[i]
			hRange := hRanges[i]

			// Check if ROI is valid
			if barStatus.ROI.Width <= 0 {
				continue
			}

			// Create rectangle from ROI
			barRect := image.Rect(
				barStatus.ROI.MinX,
				barStatus.ROI.MinY,
				barStatus.ROI.MaxX,
				barStatus.ROI.MaxY,
			)

			// Validate rectangle is within image bounds
			if barRect.Min.X < 0 || barRect.Min.Y < 0 ||
				barRect.Max.X > img_hsv.Cols() || barRect.Max.Y > img_hsv.Rows() {
				continue
			}

			// Extract bar region from HSV
			barROI := img_hsv.Region(barRect)
			defer barROI.Close()

			// Create mask for the specific color range
			lower := gocv.NewScalar(float64(hRange[0]), 100, 100, 0)
			upper := gocv.NewScalar(float64(hRange[1]), 240, 240, 0)
			mask := gocv.NewMat()
			defer mask.Close()
			gocv.InRangeWithScalar(barROI, lower, upper, &mask)

			// Find the rightmost white pixel
			fillWidth := 0
			for x := barRect.Dx() - 1; x >= 0; x-- {
				hasWhite := false
				for y := 0; y < barRect.Dy(); y++ {
					if mask.GetUCharAt(y, x) > 0 {
						hasWhite = true
						break
					}
				}
				if hasWhite {
					fillWidth = x + 1
					break
				}
			}

			widths[i] = fillWidth
		}

		return widths[0], widths[1], widths[2]
	}
	sampleCount := 30
	result := mat.Clone()

	// Check if we should do incremental detection (use cached ROI)
	if status.Open && status.Count%sampleCount != 0 {
		// Incremental detection: only detect HP/MP/FP fill width using cached ROI
		// Convert to HSV
		img_hsv := gocv.NewMat()
		defer img_hsv.Close()
		gocv.CvtColor(mat, &img_hsv, gocv.ColorBGRToHSV)

		// Detect fill widths
		hpWidth, mpWidth, fpWidth := detectBarsValue(img_hsv, status)

		// Check if all bars were detected
		if hpWidth > 0 && mpWidth > 0 && fpWidth > 0 {
			// Update status
			status.HP.Width = hpWidth
			status.HP.Value = int(float64(hpWidth) * 100 / float64(status.HP.ROI.Width))
			if status.HP.Value > 100 {
				status.HP.Value = 100
			}

			status.MP.Width = mpWidth
			status.MP.Value = int(float64(mpWidth) * 100 / float64(status.MP.ROI.Width))
			if status.MP.Value > 100 {
				status.MP.Value = 100
			}

			status.FP.Width = fpWidth
			status.FP.Value = int(float64(fpWidth) * 100 / float64(status.FP.ROI.Width))
			if status.FP.Value > 100 {
				status.FP.Value = 100
			}

			// Draw bars on result (all same color - blue)
			barStatuses := []*BarStatus{&status.HP, &status.MP, &status.FP}
			barTypes := []string{"HP", "MP", "FP"}
			barColor := color.RGBA{0, 0, 255, 255} // Blue for all bars

			for i := 0; i < 3; i++ {
				barRect := image.Rect(
					barStatuses[i].ROI.MinX,
					barStatuses[i].ROI.MinY,
					barStatuses[i].ROI.MaxX,
					barStatuses[i].ROI.MaxY,
				)
				gocv.Rectangle(&result, barRect, barColor, 2)
				text := fmt.Sprintf("%s: %d%%", barTypes[i], barStatuses[i].Value)
				gocv.PutText(&result, text,
					image.Pt(barRect.Min.X, barRect.Min.Y-5),
					gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 255, 255}, 2)
			}

			status.Count++
			status.Retry = 0
			return result
		} else {
			// Detection failed, increment retry counter
			if status.Retry < 5 {
				status.Retry++
				return result
			} else {
				// Too many retries, need full detection
				status.Open = false
				status.Count = 0
				status.Retry = 0
			}
		}
	}

	// Full detection
	// Fixed morphology parameters (tuned from detection2)
	blockSize := 100
	cValue := 2
	morphWidth := 5
	morphHeight := 3

	// === Step 1: Extract ROI ===
	img_roi := mat.Region(image.Rect(0, 0, 500, 350))
	defer img_roi.Close()

	// === Step 2: Convert to HSV ===
	img_hsv := gocv.NewMat()
	defer img_hsv.Close()
	gocv.CvtColor(img_roi, &img_hsv, gocv.ColorBGRToHSV)

	// === Step 3: Adaptive Threshold V channel ===
	channels := gocv.Split(img_hsv)
	defer func() {
		for i := range channels {
			channels[i].Close()
		}
	}()
	vChannel := channels[2]

	// Ensure block size is odd and at least 3
	if blockSize < 3 {
		blockSize = 3
	}
	if blockSize%2 == 0 {
		blockSize++
	}

	img_v := gocv.NewMat()
	defer img_v.Close()
	gocv.AdaptiveThreshold(vChannel, &img_v, 255, gocv.AdaptiveThresholdGaussian, gocv.ThresholdBinaryInv, blockSize, float32(cValue))

	// === Step 4: Invert ===
	img_vr := gocv.NewMat()
	defer img_vr.Close()
	gocv.BitwiseNot(img_v, &img_vr)

	// === Step 5: Morphological operations ===
	kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(morphWidth, morphHeight))
	defer kernel.Close()

	closed := gocv.NewMat()
	defer closed.Close()
	gocv.MorphologyEx(img_vr, &closed, gocv.MorphClose, kernel)

	img_vrm := gocv.NewMat()
	defer img_vrm.Close()
	gocv.MorphologyEx(closed, &img_vrm, gocv.MorphOpen, kernel)

	// === Step 6: Invert for outer frame detection ===
	img_vrmr := gocv.NewMat()
	defer img_vrmr.Close()
	gocv.BitwiseNot(img_vrm, &img_vrmr)

	// === Step 7: Detect outer frame ===
	outerContours := gocv.FindContours(img_vrmr, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer outerContours.Close()

	var img_outline image.Rectangle
	found := false
	for i := 0; i < outerContours.Size(); i++ {
		contour := outerContours.At(i)
		rect := gocv.BoundingRect(contour)
		if rect.Dx() >= 400 && rect.Dx() <= 600 && rect.Dy() >= 180 && rect.Dy() <= 300 {
			img_outline = rect
			found = true
			break
		}
	}

	if !found {
		return result
	}

	// Draw outer frame (GREEN)
	gocv.Rectangle(&result, img_outline, color.RGBA{0, 255, 0, 255}, 2)

	// === Step 8: Find avatar ===
	frameMask := img_vrm.Region(img_outline)
	defer frameMask.Close()

	innerContours := gocv.FindContours(frameMask, gocv.RetrievalList, gocv.ChainApproxSimple)
	defer innerContours.Close()

	var img_avatar image.Rectangle
	for i := 0; i < innerContours.Size(); i++ {
		contour := innerContours.At(i)
		rect := gocv.BoundingRect(contour)
		if rect.Dx() >= 80 && rect.Dx() <= 200 && rect.Dy() >= 100 && rect.Dy() <= 300 {
			img_avatar = rect
			break
		}
	}

	// Draw avatar (BLUE)
	if img_avatar.Dx() > 0 {
		absoluteAvatar := image.Rect(
			img_avatar.Min.X+img_outline.Min.X,
			img_avatar.Min.Y+img_outline.Min.Y,
			img_avatar.Max.X+img_outline.Min.X,
			img_avatar.Max.Y+img_outline.Min.Y,
		)
		gocv.Rectangle(&result, absoluteAvatar, color.RGBA{0, 0, 255, 255}, 2)
	}

	// === Step 9: Define bar area and find bars ===
	var img_bars []image.Rectangle
	if img_avatar.Dx() > 0 {
		// Calculate bar area top boundary (avatar top - 15, but minimum 1)
		barAreaTop := img_avatar.Min.Y - 15
		if barAreaTop < 1 {
			barAreaTop = 1
		}
		img_bararea := image.Rect(
			img_avatar.Max.X,
			barAreaTop,
			img_outline.Dx(),
			img_avatar.Max.Y,
		)

		img_bararea_abs := image.Rect(
			img_bararea.Min.X+img_outline.Min.X,
			img_bararea.Min.Y+img_outline.Min.Y,
			img_bararea.Max.X+img_outline.Min.X,
			img_bararea.Max.Y+img_outline.Min.Y,
		)

		// Draw bar area (GREEN)
		gocv.Rectangle(&result, img_bararea_abs, color.RGBA{0, 255, 0, 255}, 2)

		barAreaMask := img_vrm.Region(img_bararea_abs)
		defer barAreaMask.Close()

		barContours := gocv.FindContours(barAreaMask, gocv.RetrievalList, gocv.ChainApproxSimple)
		defer barContours.Close()

		for i := 0; i < barContours.Size(); i++ {
			contour := barContours.At(i)
			rect := gocv.BoundingRect(contour)
			if rect.Dx() >= 100 && rect.Dx() <= 300 && rect.Dy() >= 5 && rect.Dy() <= 30 {
				absoluteRect := image.Rect(
					rect.Min.X+img_bararea_abs.Min.X,
					rect.Min.Y+img_bararea_abs.Min.Y,
					rect.Max.X+img_bararea_abs.Min.X,
					rect.Max.Y+img_bararea_abs.Min.Y,
				)
				img_bars = append(img_bars, absoluteRect)
			}
		}

		// Sort bars from top to bottom
		for i := 0; i < len(img_bars); i++ {
			for j := i + 1; j < len(img_bars); j++ {
				if img_bars[i].Min.Y > img_bars[j].Min.Y {
					img_bars[i], img_bars[j] = img_bars[j], img_bars[i]
				}
			}
		}

		// === Step 10: Process HP, MP, FP bars ===
		if len(img_bars) >= 3 {
			barTypes := []string{"HP", "MP", "FP"}
			hRanges := [][2]int{{160, 180}, {90, 120}, {45, 70}}
			fillWidths := make([]int, 3)

			for i := 0; i < 3; i++ {
				barRect := img_bars[i]
				barType := barTypes[i]
				hRange := hRanges[i]

				// Draw bar outline (BLUE)
				gocv.Rectangle(&result, barRect, color.RGBA{0, 0, 255, 255}, 2)

				// Extract bar region from HSV
				barROI := img_hsv.Region(barRect)
				defer barROI.Close()

				// Create mask for the specific color range
				lower := gocv.NewScalar(float64(hRange[0]), 100, 100, 0)
				upper := gocv.NewScalar(float64(hRange[1]), 240, 240, 0)
				mask := gocv.NewMat()
				defer mask.Close()
				gocv.InRangeWithScalar(barROI, lower, upper, &mask)

				// Find the rightmost white pixel
				fillWidth := 0
				for x := barRect.Dx() - 1; x >= 0; x-- {
					hasWhite := false
					for y := 0; y < barRect.Dy(); y++ {
						if mask.GetUCharAt(y, x) > 0 {
							hasWhite = true
							break
						}
					}
					if hasWhite {
						fillWidth = x + 1
						break
					}
				}
				fillWidths[i] = fillWidth

				// Draw text (YELLOW) - no percentage during full detection
				text := fmt.Sprintf("%s", barType)
				gocv.PutText(&result, text,
					image.Pt(barRect.Min.X, barRect.Min.Y-5),
					gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 255, 255}, 2)
			}

			// === Validation ===
			valid := true

			// 1. Check horizontal alignment: both MinX and MaxX coordinate difference <= 5
			maxMinX := img_bars[0].Min.X
			minMinX := img_bars[0].Min.X
			maxMaxX := img_bars[0].Max.X
			minMaxX := img_bars[0].Max.X
			for i := 1; i < 3; i++ {
				if img_bars[i].Min.X > maxMinX {
					maxMinX = img_bars[i].Min.X
				}
				if img_bars[i].Min.X < minMinX {
					minMinX = img_bars[i].Min.X
				}
				if img_bars[i].Max.X > maxMaxX {
					maxMaxX = img_bars[i].Max.X
				}
				if img_bars[i].Max.X < minMaxX {
					minMaxX = img_bars[i].Max.X
				}
			}
			minXDiff := maxMinX - minMinX
			maxXDiff := maxMaxX - minMaxX
			if minXDiff > 5 || maxXDiff > 5 {
				valid = false
			}

			// 2. Check height consistency: difference <= 5
			maxHeight := img_bars[0].Dy()
			minHeight := img_bars[0].Dy()
			for i := 1; i < 3; i++ {
				height := img_bars[i].Dy()
				if height > maxHeight {
					maxHeight = height
				}
				if height < minHeight {
					minHeight = height
				}
			}
			heightDiff := maxHeight - minHeight
			if heightDiff > 5 {
				valid = false
			}

			// 3. Check vertical spacing consistency: difference <= 5
			hpMpGap := img_bars[1].Min.Y - img_bars[0].Max.Y
			mpFpGap := img_bars[2].Min.Y - img_bars[1].Max.Y
			gapDiff := hpMpGap - mpFpGap
			if gapDiff < 0 {
				gapDiff = -gapDiff
			}
			if gapDiff > 5 {
				valid = false
			}

			// 4. Check fill width consistency: difference <= 5
			maxFillWidth := fillWidths[0]
			minFillWidth := fillWidths[0]
			for i := 1; i < 3; i++ {
				if fillWidths[i] > maxFillWidth {
					maxFillWidth = fillWidths[i]
				}
				if fillWidths[i] < minFillWidth {
					minFillWidth = fillWidths[i]
				}
			}
			fillWidthDiff := maxFillWidth - minFillWidth
			if fillWidthDiff > 5 {
				valid = false
			}

			// Display validation result at top-left of outer frame
			validationText := ""
			validationColor := color.RGBA{0, 0, 255, 255} // Red
			if valid {
				validationText = "true"
				validationColor = color.RGBA{0, 255, 0, 255} // Green
			} else {
				validationText = fmt.Sprintf("fail(%d,%d,%d,%d,%d)", minXDiff, maxXDiff, heightDiff, gapDiff, fillWidthDiff)
			}

			if valid {
				// Update status with detected bars (using weighted average if previous data exists)
				barStatuses := []*BarStatus{&status.HP, &status.MP, &status.FP}
				for i := 0; i < 3; i++ {
					barRect := img_bars[i]
					fillWidth := fillWidths[i]

					// Create new ROI
					// When validation is true, bars are full, so Width = fillWidth
					newROI := BarROI{
						MinX:  barRect.Min.X,
						MinY:  barRect.Min.Y,
						MaxX:  barRect.Max.X,
						MaxY:  barRect.Max.Y,
						Width: fillWidth,
					}

					// Calculate weighted average if previous data exists
					if status.Open && barStatuses[i].ROI.Width > 0 {
						weight := 0.3    // New data weight
						oldWeight := 0.7 // Old data weight

						// Average the ROI coordinates
						newROI.MinX = int(float64(barStatuses[i].ROI.MinX)*oldWeight + float64(newROI.MinX)*weight)
						newROI.MinY = int(float64(barStatuses[i].ROI.MinY)*oldWeight + float64(newROI.MinY)*weight)
						newROI.MaxX = int(float64(barStatuses[i].ROI.MaxX)*oldWeight + float64(newROI.MaxX)*weight)
						newROI.MaxY = int(float64(barStatuses[i].ROI.MaxY)*oldWeight + float64(newROI.MaxY)*weight)
						// Average the width as well
						newROI.Width = int(float64(barStatuses[i].ROI.Width)*oldWeight + float64(fillWidth)*weight)
					}

					// Update bar status
					barStatuses[i].ROI = newROI
					barStatuses[i].Width = fillWidth
					barStatuses[i].Value = 100 // Validation passed means bars are full (100%)
				}

				status.Open = true
				status.Count++
				status.Retry = 0
			}
			gocv.PutText(&result, validationText,
				image.Pt(img_outline.Min.X+5, img_outline.Min.Y+20),
				gocv.FontHersheyPlain, 1.5, validationColor, 2)
		}
	}

	return result
}

// runDetection3 - Simplified detection with single window
func runDetection3(useStaticImage bool, staticMat gocv.Mat, browser *DebugBrowser, statusImagePath string) {
	window := gocv.NewWindow("Status Detection")
	defer window.Close()

	// Initialize detection status
	status := &detectMyStatus{
		Open:  false,
		Count: 0,
		Retry: 0,
	}

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

		// Detect and display
		result := detectStatusBars3(mat, status)
		defer result.Close()
		window.IMShow(result)

		// Display status as JSON
		statusJSON, err := json.MarshalIndent(status, "", "  ")
		if err == nil {
			fmt.Printf("\n=== Detection Status ===\n%s\n", string(statusJSON))
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

	// Use detection algorithm (comment/uncomment to switch)
	// runDetection1(useStaticImage, staticMat, browser, statusImagePath)
	// runDetection2(useStaticImage, staticMat, browser, statusImagePath)
	runDetection3(useStaticImage, staticMat, browser, statusImagePath)
}
