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

// PerformanceStats holds timing information for various operations
type PerformanceStats struct {
	CaptureTime    time.Duration
	ConvertTime    time.Duration
	ROIProcessTime time.Duration
	DisplayTime    time.Duration
	TotalDelay     time.Duration
	LastUpdate     time.Time
}

func main() {
	// Lock the OS thread for macOS GUI operations
	runtime.LockOSThread()

	fmt.Println("=== HSV Debug Tool ===")

	// Load cookies from cookie.json
	cookiePath := "cookie.json"
	cookies, err := loadCookies(cookiePath)
	if err != nil {
		fmt.Printf("Warning: failed to load cookies: %v\n", err)
		cookies = make([]Cookie, 0)
	} else if len(cookies) > 0 {
		fmt.Printf("Loaded %d cookies from %s\n", len(cookies), cookiePath)
	} else {
		fmt.Printf("No cookies found in %s\n", cookiePath)
	}

	fmt.Println("Opening browser and navigating to Flyff Universe...")

	// Create browser
	browser := NewDebugBrowser()
	defer browser.Stop()

	// Start browser with cookies
	err = browser.Start(cookies)
	if err != nil {
		fmt.Printf("Failed to start browser: %v\n", err)
		return
	}

	fmt.Println("Browser started successfully. Waiting for page to load...")
	time.Sleep(5 * time.Second)

	// Initialize performance stats
	stats := &PerformanceStats{
		LastUpdate: time.Now(),
	}

	// Create windows
	window := gocv.NewWindow("Original - Adjust HSV & ROI")
	defer window.Close()

	maskWindow := gocv.NewWindow("Mask")
	defer maskWindow.Close()

	resultWindow := gocv.NewWindow("Result")
	defer resultWindow.Close()

	// Wait for first frame to get dimensions
	fmt.Println("Waiting for first frame...")
	var initialImg *image.RGBA
	for i := 0; i < 100; i++ { // Try for up to 10 seconds
		if frame, ok := browser.GetFrame(); ok {
			fmt.Println("Got first frame!")
			initialImg = frame
			break
		}
		if i%10 == 0 {
			fmt.Printf("Still waiting... (%d/100)\n", i)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if initialImg == nil {
		fmt.Printf("Failed to get initial frame after 10 seconds\n")
		return
	}

	imgWidth := initialImg.Bounds().Dx()
	imgHeight := initialImg.Bounds().Dy()
	fmt.Printf("Frame size: %dx%d\n", imgWidth, imgHeight)

	// Create trackbars for HSV
	var minH, maxH, minS, maxS, minV, maxV int
	maxH = 360
	maxS = 255
	maxV = 255

	window.CreateTrackbarWithValue("Min H", &minH, 360)
	window.CreateTrackbarWithValue("Max H", &maxH, 360)
	window.CreateTrackbarWithValue("Min S", &minS, 255)
	window.CreateTrackbarWithValue("Max S", &maxS, 255)
	window.CreateTrackbarWithValue("Min V", &minV, 255)
	window.CreateTrackbarWithValue("Max V", &maxV, 255)

	// Create trackbars for ROI
	var roiMinX, roiMaxX, roiMinY, roiMaxY int
	roiMaxX = imgWidth
	roiMaxY = imgHeight

	window.CreateTrackbarWithValue("ROI MinX", &roiMinX, imgWidth)
	window.CreateTrackbarWithValue("ROI MaxX", &roiMaxX, imgWidth)
	window.CreateTrackbarWithValue("ROI MinY", &roiMinY, imgHeight)
	window.CreateTrackbarWithValue("ROI MaxY", &roiMaxY, imgHeight)

	// Create trackbars for detection point position
	var detectX, detectY int
	detectX = imgWidth / 2
	detectY = imgHeight / 2

	resultWindow.CreateTrackbarWithValue("Detect X", &detectX, imgWidth)
	resultWindow.CreateTrackbarWithValue("Detect Y", &detectY, imgHeight)

	// Create trackbar for update interval (in seconds)
	var updateInterval int = 5 // Default 5 seconds
	window.CreateTrackbarWithValue("Update Interval (s)", &updateInterval, 60)

	fmt.Println("\nControls:")
	fmt.Println("  Adjust HSV sliders to tune color detection")
	fmt.Println("  Adjust ROI sliders to change detection area")
	fmt.Println("  Adjust Detect X/Y sliders to move detection point")
	fmt.Println("  Adjust 'Update Interval' slider to change refresh rate (0-60 seconds, 0=realtime)")
	fmt.Println("  'p': Print current HSV range and ROI")
	fmt.Println("  'r': Refresh now (force immediate update)")
	fmt.Println("  'q' or ESC: Quit")

	// Variables to track update timing
	var lastUpdateTime time.Time
	var mat gocv.Mat
	var captureTime, convertTime time.Duration
	forceUpdate := true // Force first update
	matInitialized := false // Track if mat has been initialized

	for {
		frameStart := time.Now()

		// Check if we need to update (based on interval or force update)
		currentInterval := updateInterval
		if currentInterval < 0 {
			currentInterval = 0 // Minimum 0 seconds (realtime)
		}

		// If interval is 0, always update (realtime mode)
		shouldUpdate := forceUpdate || currentInterval == 0 || time.Since(lastUpdateTime) >= time.Duration(currentInterval)*time.Second

		if shouldUpdate {
			// Get frame from stream
			captureStart := time.Now()
			img, ok := browser.GetFrame()
			captureTime = time.Since(captureStart)
			if !ok {
				// No new frame available, keep processing old frame
				if !matInitialized {
					// No frame at all yet, wait and continue
					key := window.WaitKey(100)
					if key == 'q' || key == 27 {
						fmt.Println("Exiting...")
						return
					}
					continue
				}
			} else {
				// Convert to gocv.Mat
				convertStart := time.Now()
				if matInitialized {
					mat.Close() // Close previous mat
				}
				var err error
				mat, err = gocv.ImageToMatRGB(img)
				convertTime = time.Since(convertStart)
				if err != nil {
					fmt.Printf("Failed to convert image to mat: %v\n", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				matInitialized = true
			}

			lastUpdateTime = time.Now()
			forceUpdate = false
		}

		// If we don't have a valid mat yet, wait and continue
		if !matInitialized {
			key := window.WaitKey(100)
			if key == 'q' || key == 27 {
				fmt.Println("Exiting...")
				if matInitialized {
					mat.Close()
				}
				return
			}
			continue
		}

		// ROI Processing
		roiProcessStart := time.Now()

		// Validate and adjust ROI
		if roiMinX < 0 {
			roiMinX = 0
		}
		if roiMaxX > mat.Cols() {
			roiMaxX = mat.Cols()
		}
		if roiMinY < 0 {
			roiMinY = 0
		}
		if roiMaxY > mat.Rows() {
			roiMaxY = mat.Rows()
		}
		if roiMinX >= roiMaxX {
			roiMinX = 0
			roiMaxX = mat.Cols()
		}
		if roiMinY >= roiMaxY {
			roiMinY = 0
			roiMaxY = mat.Rows()
		}

		// Extract ROI
		roiMat := mat.Region(image.Rect(roiMinX, roiMinY, roiMaxX, roiMaxY))

		// Convert to HSV
		hsv := gocv.NewMat()
		gocv.CvtColor(roiMat, &hsv, gocv.ColorBGRToHSV)

		// Create mask
		lower := gocv.NewScalar(float64(minH), float64(minS), float64(minV), 0)
		upper := gocv.NewScalar(float64(maxH), float64(maxS), float64(maxV), 0)
		mask := gocv.NewMat()
		gocv.InRangeWithScalar(hsv, lower, upper, &mask)

		// Apply mask to get result
		result := gocv.NewMat()
		gocv.BitwiseAndWithMask(roiMat, roiMat, &result, mask)

		roiProcessTime := time.Since(roiProcessStart)

		// Draw ROI rectangle on original image
		displayImg := mat.Clone()
		gocv.Rectangle(&displayImg,
			image.Rect(roiMinX, roiMinY, roiMaxX, roiMaxY),
			color.RGBA{0, 255, 0, 255}, 2)

		// Add ROI label
		roiText := fmt.Sprintf("ROI: (%d,%d) to (%d,%d)", roiMinX, roiMinY, roiMaxX, roiMaxY)
		gocv.PutText(&displayImg, roiText,
			image.Pt(10, 30),
			gocv.FontHersheyPlain, 1.5, color.RGBA{0, 255, 0, 255}, 2)

		// Calculate time until next update
		var nextUpdateText string
		if currentInterval == 0 {
			nextUpdateText = "Realtime Mode"
		} else {
			timeSinceUpdate := time.Since(lastUpdateTime)
			timeUntilNext := time.Duration(currentInterval)*time.Second - timeSinceUpdate
			if timeUntilNext < 0 {
				timeUntilNext = 0
			}
			nextUpdateText = fmt.Sprintf("Next Update In: %.1f s", timeUntilNext.Seconds())
		}

		// Add performance stats and update timer
		statsY := 60
		statsLineHeight := 25
		statsBg := color.RGBA{0, 0, 0, 180}
		statsColor := color.RGBA{255, 255, 0, 255}
		timerColor := color.RGBA{0, 255, 255, 255}

		var intervalText string
		if currentInterval == 0 {
			intervalText = "Update Interval: Realtime"
		} else {
			intervalText = fmt.Sprintf("Update Interval: %d s", currentInterval)
		}

		perfTexts := []string{
			intervalText,
			nextUpdateText,
			fmt.Sprintf("Capture: %.2f ms", stats.CaptureTime.Seconds()*1000),
			fmt.Sprintf("Convert: %.2f ms", stats.ConvertTime.Seconds()*1000),
			fmt.Sprintf("ROI Process: %.2f ms", stats.ROIProcessTime.Seconds()*1000),
			fmt.Sprintf("Display: %.2f ms", stats.DisplayTime.Seconds()*1000),
			fmt.Sprintf("Total Delay: %.2f ms", stats.TotalDelay.Seconds()*1000),
		}
		textColors := []color.RGBA{
			timerColor, // Update interval
			timerColor, // Next update in
			statsColor, // Capture
			statsColor, // Convert
			statsColor, // ROI Process
			statsColor, // Display
			statsColor, // Total Delay
		}

		for i, text := range perfTexts {
			textSize := gocv.GetTextSize(text, gocv.FontHersheyPlain, 1.2, 2)
			y := statsY + i*statsLineHeight

			// Draw background
			gocv.Rectangle(&displayImg,
				image.Rect(5, y-textSize.Y-5, textSize.X+15, y+5),
				statsBg, -1)

			// Draw text with appropriate color
			gocv.PutText(&displayImg, text,
				image.Pt(10, y),
				gocv.FontHersheyPlain, 1.2, textColors[i], 2)
		}

		// Adjust detection point to be within ROI bounds
		currentDetectX := detectX
		currentDetectY := detectY
		if currentDetectX < 0 {
			currentDetectX = 0
		}
		if currentDetectX >= result.Cols() {
			currentDetectX = result.Cols() - 1
		}
		if currentDetectY < 0 {
			currentDetectY = 0
		}
		if currentDetectY >= result.Rows() {
			currentDetectY = result.Rows() - 1
		}

		// Get HSV value at detection point
		hsvVec := hsv.GetVecbAt(currentDetectY, currentDetectX)
		h := hsvVec[0]
		s := hsvVec[1]
		v := hsvVec[2]

		// Display H value in 0-360 range (multiply by 2)
		hDisplay := int(h) * 2

		// Check if pixel is in mask
		maskValue := mask.GetUCharAt(currentDetectY, currentDetectX)
		inMask := maskValue > 0

		// Create text
		text := fmt.Sprintf("Pos:(%d,%d) HSV:(%d,%d,%d) Match:%v",
			currentDetectX, currentDetectY, hDisplay, s, v, inMask)

		// Draw on result image
		textX := currentDetectX + 15
		textY := currentDetectY - 10

		// Adjust text position if near edge
		textSize := gocv.GetTextSize(text, gocv.FontHersheyPlain, 1.0, 2)
		if textX+textSize.X+10 > result.Cols() {
			textX = currentDetectX - textSize.X - 15
		}
		if textY-textSize.Y-5 < 0 {
			textY = currentDetectY + 25
		}

		// Draw background rectangle for text
		gocv.Rectangle(&result,
			image.Rect(textX-5, textY-textSize.Y-5,
				textX+textSize.X+5, textY+5),
			color.RGBA{0, 0, 0, 200}, -1)

		// Draw text
		textColor := color.RGBA{0, 255, 255, 255}
		if inMask {
			textColor = color.RGBA{0, 255, 0, 255} // Green if matched
		}
		gocv.PutText(&result, text,
			image.Pt(textX, textY),
			gocv.FontHersheyPlain, 1.0, textColor, 2)

		// Draw crosshair at detection point
		crosshairSize := 15
		crosshairColor := color.RGBA{255, 0, 0, 255}
		if inMask {
			crosshairColor = color.RGBA{0, 255, 0, 255}
		}
		gocv.Line(&result,
			image.Pt(currentDetectX-crosshairSize, currentDetectY),
			image.Pt(currentDetectX+crosshairSize, currentDetectY),
			crosshairColor, 2)
		gocv.Line(&result,
			image.Pt(currentDetectX, currentDetectY-crosshairSize),
			image.Pt(currentDetectX, currentDetectY+crosshairSize),
			crosshairColor, 2)
		gocv.Circle(&result, image.Pt(currentDetectX, currentDetectY), 3,
			crosshairColor, -1)

		// Show windows
		displayStart := time.Now()
		window.IMShow(displayImg)
		maskWindow.IMShow(mask)
		resultWindow.IMShow(result)
		displayTime := time.Since(displayStart)

		// Calculate total delay
		totalDelay := time.Since(frameStart)

		// Update stats every 5 seconds
		now := time.Now()
		if now.Sub(stats.LastUpdate) >= 5*time.Second {
			stats.CaptureTime = captureTime
			stats.ConvertTime = convertTime
			stats.ROIProcessTime = roiProcessTime
			stats.DisplayTime = displayTime
			stats.TotalDelay = totalDelay
			stats.LastUpdate = now
		}

		// Cleanup temporary mats (but keep mat for reuse)
		displayImg.Close()
		roiMat.Close()
		hsv.Close()
		mask.Close()
		result.Close()

		// Handle key input with a short wait
		key := window.WaitKey(100)

		switch key {
		case 'q', 27: // 'q' or ESC
			fmt.Println("Exiting...")
			if matInitialized {
				mat.Close()
			}
			return
		case 'p': // Print current settings
			fmt.Println("\n=== Current Settings ===")
			fmt.Printf("HSV Range: H(%d-%d) S(%d-%d) V(%d-%d)\n",
				minH, maxH, minS, maxS, minV, maxV)
			fmt.Printf("ROI: MinX=%d, MaxX=%d, MinY=%d, MaxY=%d\n",
				roiMinX, roiMaxX, roiMinY, roiMaxY)
			fmt.Printf("ROI Size: %dx%d\n", roiMaxX-roiMinX, roiMaxY-roiMinY)
			fmt.Printf("Detection Point: (%d, %d)\n", detectX, detectY)
			if updateInterval == 0 {
				fmt.Printf("Update Interval: Realtime\n")
			} else {
				fmt.Printf("Update Interval: %d seconds\n", updateInterval)
			}
			fmt.Println("========================")
		case 'r': // Force refresh
			fmt.Println("Forcing refresh...")
			forceUpdate = true
		}
	}
}
