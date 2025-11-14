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
	// runDetection3(useStaticImage, staticMat, browser, statusImagePath)
	runDetection4(useStaticImage, staticMat, browser, statusImagePath)
}

// ============================================================================
// Detection Algorithm 4: Target HP/MP Detection with Circle Avatar
// ============================================================================

// TargetBarInfo stores information about detected target HP/MP bars
type TargetBarInfo struct {
	Rect       image.Rectangle
	FillWidth  int
	Percentage float64
	Type       string // "HP" or "MP"
}

// ROIParams4 defines the region of interest for target detection
type ROIParams4 struct {
	MinX int
	MinY int
	MaxX int
	MaxY int
}

// CircleParams defines parameters for avatar contour detection
type CircleParams struct {
	MinSize int // Minimum side length (width or height)
	MaxSize int // Maximum side length (width or height)
}

// HSVRange defines HSV color range for detection
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

// BarAreaParams defines the parameters for bar detection area relative to circle
type BarAreaParams struct {
	LeftOffset   int // Offset from circle right edge
	RightOffset  int // Offset from circle right edge to define area width
	TopOffset    int // Offset from circle top
	BottomOffset int // Offset from circle bottom
}

// BarParams defines constraints for bar detection
type BarParams struct {
	MinWidth  int
	MaxWidth  int
	MinHeight int
	MaxHeight int
}

// detectStatusBars4 detects target's HP and MP bars using circle-based detection
func detectStatusBars4(mat gocv.Mat, window1 *gocv.Window, window2 *gocv.Window, window3 *gocv.Window, window4 *gocv.Window, window5 *gocv.Window, roi *ROIParams4, morphParams *MorphParams, circleParams *CircleParams, barAreaParams *BarAreaParams, barParams *BarParams, hsvRanges [2]*HSVRange, printProgress bool) []TargetBarInfo {
	// Add panic recovery to prevent crashes
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic in detectStatusBars4: %v\n", r)
		}
	}()

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

	// Validate input mat
	if mat.Empty() {
		fmt.Println("Error: input mat is empty")
		return nil
	}

	if printProgress {
		fmt.Printf("\n=== Starting Circle-Based Detection ===\n")
		fmt.Printf("Input mat size: %dx%d, channels: %d\n", mat.Cols(), mat.Rows(), mat.Channels())
	}

	// ============================================================================
	// Window 1: ROI Selection and Binary Thresholding
	// ============================================================================
	// Validate and clamp ROI coordinates
	matWidth := mat.Cols()
	matHeight := mat.Rows()

	if roi.MinX < 0 {
		roi.MinX = 0
	}
	if roi.MinY < 0 {
		roi.MinY = 0
	}
	if roi.MaxX > matWidth {
		roi.MaxX = matWidth
	}
	if roi.MaxY > matHeight {
		roi.MaxY = matHeight
	}
	if roi.MinX >= roi.MaxX {
		roi.MaxX = matWidth
	}
	if roi.MinY >= roi.MaxY {
		roi.MaxY = matHeight
	}

	// Check if ROI is valid
	if roi.MinX >= roi.MaxX || roi.MinY >= roi.MaxY {
		fmt.Printf("Error: invalid ROI coordinates: (%d,%d) to (%d,%d)\n", roi.MinX, roi.MinY, roi.MaxX, roi.MaxY)
		return nil
	}

	img_roi := mat.Region(image.Rect(roi.MinX, roi.MinY, roi.MaxX, roi.MaxY))
	defer img_roi.Close()

	if img_roi.Empty() {
		fmt.Println("Error: ROI extraction resulted in empty mat")
		return nil
	}

	if printProgress {
		fmt.Printf("ROI size: %dx%d\n", img_roi.Cols(), img_roi.Rows())
	}

	window1Display := img_roi.Clone()
	defer window1Display.Close()
	gocv.PutText(&window1Display, "Original ROI",
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)

	// Convert to grayscale for thresholding
	img_gray := gocv.NewMat()
	defer img_gray.Close()
	gocv.CvtColor(img_roi, &img_gray, gocv.ColorBGRToGray)

	// Adaptive threshold
	img_binary := gocv.NewMat()
	defer img_binary.Close()

	blockSize := morphParams.BlockSize
	if blockSize < 3 {
		blockSize = 3
	}
	if blockSize%2 == 0 {
		blockSize++
	}

	var adaptiveMethod gocv.AdaptiveThresholdType
	if morphParams.AdaptiveMethod == 0 {
		adaptiveMethod = gocv.AdaptiveThresholdMean
	} else {
		adaptiveMethod = gocv.AdaptiveThresholdGaussian
	}

	gocv.AdaptiveThreshold(img_gray, &img_binary, 255, adaptiveMethod, gocv.ThresholdBinary, blockSize, float32(morphParams.CValue))

	img_binary_bgr := gocv.NewMat()
	defer img_binary_bgr.Close()
	gocv.CvtColor(img_binary, &img_binary_bgr, gocv.ColorGrayToBGR)
	gocv.PutText(&img_binary_bgr, "Binary",
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)
	appendImage(&window1Display, img_binary_bgr, false)

	// Apply morphological operations: dilate then erode
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

	img_dilated := gocv.NewMat()
	defer img_dilated.Close()
	gocv.Dilate(img_binary, &img_dilated, kernel)

	img_dilated_bgr := gocv.NewMat()
	defer img_dilated_bgr.Close()
	gocv.CvtColor(img_dilated, &img_dilated_bgr, gocv.ColorGrayToBGR)
	gocv.PutText(&img_dilated_bgr, "Dilated",
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)
	appendImage(&window1Display, img_dilated_bgr, false)

	img_morphed := gocv.NewMat()
	defer img_morphed.Close()
	gocv.Erode(img_dilated, &img_morphed, kernel)

	img_morphed_bgr := gocv.NewMat()
	defer img_morphed_bgr.Close()
	gocv.CvtColor(img_morphed, &img_morphed_bgr, gocv.ColorGrayToBGR)
	gocv.PutText(&img_morphed_bgr, "Eroded",
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)
	appendImage(&window1Display, img_morphed_bgr, false)

	window1.IMShow(window1Display)

	// ============================================================================
	// Window 2: Avatar Detection (using Contours on Morphed Image)
	// ============================================================================

	// Find contours on morphed image
	contours := gocv.FindContours(img_morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	if printProgress {
		fmt.Printf("Found %d contours\n", contours.Size())
	}

	// Create display for window 2
	img_morphed_display := gocv.NewMat()
	defer img_morphed_display.Close()
	gocv.CvtColor(img_morphed, &img_morphed_display, gocv.ColorGrayToBGR)
	gocv.PutText(&img_morphed_display, "Morphed (Input)",
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)
	window2Display := img_morphed_display.Clone()
	defer window2Display.Close()

	// Show contour detection result
	contourResult := gocv.NewMat()
	defer contourResult.Close()
	gocv.CvtColor(img_morphed, &contourResult, gocv.ColorGrayToBGR)

	var avatarRect image.Rectangle
	var circleCenter image.Point
	var circleRadius int
	circleFound := false
	validCount := 0
	invalidCount := 0

	// Process each contour to find avatar
	for i := 0; i < contours.Size(); i++ {
		contour := contours.At(i)

		// Get bounding rectangle
		rect := gocv.BoundingRect(contour)
		width := rect.Dx()
		height := rect.Dy()
		maxSide := width
		if height > maxSide {
			maxSide = height
		}
		minSide := width
		if height < minSide {
			minSide = height
		}

		if printProgress && i < 10 { // Only print first 10
			fmt.Printf("  Contour %d: pos=(%d,%d) size=%dx%d maxSide=%d minSide=%d\n",
				i, rect.Min.X, rect.Min.Y, width, height, maxSide, minSide)
		}

		// Check if both sides are within range
		if maxSide >= circleParams.MinSize && maxSide <= circleParams.MaxSize &&
			minSide >= circleParams.MinSize && minSide <= circleParams.MaxSize {
			// Valid avatar contour
			validCount++

			if !circleFound {
				// Select the first valid contour
				avatarRect = rect
				circleCenter = image.Pt(rect.Min.X+width/2, rect.Min.Y+height/2)
				circleRadius = (width + height) / 4 // average radius for bar area calculation
				circleFound = true
				if printProgress {
					fmt.Printf("Selected avatar: center=(%d,%d) size=%dx%d rect=(%d,%d)-(%d,%d)\n",
						circleCenter.X, circleCenter.Y, width, height,
						rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
				}
				// Draw selected contour in green
				gocv.Rectangle(&contourResult, rect, color.RGBA{0, 255, 0, 255}, 2)
				// Draw center point
				gocv.Circle(&contourResult, circleCenter, 3, color.RGBA{0, 0, 255, 255}, -1)
			} else {
				// Draw other valid contours in yellow
				gocv.Rectangle(&contourResult, rect, color.RGBA{0, 255, 255, 255}, 1)
			}
		} else {
			// Invalid contour (size out of range)
			invalidCount++
			if invalidCount <= 10 { // Only draw first 10 invalid contours
				gocv.Rectangle(&contourResult, rect, color.RGBA{255, 0, 0, 255}, 1)
			}
		}
	}

	gocv.PutText(&contourResult, fmt.Sprintf("Valid: %d Invalid: %d", validCount, invalidCount),
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)
	appendImage(&window2Display, contourResult, false)

	window2.IMShow(window2Display)

	// ============================================================================
	// Window 3: Bar Detection Area
	// ============================================================================
	window3Display := img_roi.Clone()
	defer window3Display.Close()
	gocv.PutText(&window3Display, "Original",
		image.Pt(10, 20),
		gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)

	var barArea image.Rectangle
	var barAreaValid bool

	if circleFound {
		// Define bar area based on circle position
		barLeft := circleCenter.X + circleRadius + barAreaParams.LeftOffset
		barRight := circleCenter.X + circleRadius + barAreaParams.RightOffset
		barTop := circleCenter.Y - circleRadius + barAreaParams.TopOffset
		barBottom := circleCenter.Y + circleRadius + barAreaParams.BottomOffset

		// Clamp to ROI bounds
		if barLeft < 0 {
			barLeft = 0
		}
		if barRight > img_roi.Cols() {
			barRight = img_roi.Cols()
		}
		if barTop < 0 {
			barTop = 0
		}
		if barBottom > img_roi.Rows() {
			barBottom = img_roi.Rows()
		}

		barArea = image.Rect(barLeft, barTop, barRight, barBottom)
		barAreaValid = barArea.Dx() > 0 && barArea.Dy() > 0

		if printProgress {
			fmt.Printf("Bar area: (%d,%d) to (%d,%d), size: %dx%d\n",
				barArea.Min.X, barArea.Min.Y, barArea.Max.X, barArea.Max.Y,
				barArea.Dx(), barArea.Dy())
		}

		// Draw bar area
		areaResult := img_roi.Clone()
		defer areaResult.Close()

		// Draw avatar rectangle
		gocv.Rectangle(&areaResult, avatarRect, color.RGBA{0, 255, 0, 255}, 2)
		// Draw center point
		gocv.Circle(&areaResult, circleCenter, 3, color.RGBA{0, 0, 255, 255}, -1)

		// Draw bar area
		if barAreaValid {
			gocv.Rectangle(&areaResult, barArea, color.RGBA{255, 255, 0, 255}, 2)
			gocv.PutText(&areaResult, "Bar Area",
				image.Pt(barArea.Min.X+5, barArea.Min.Y+15),
				gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 0, 255}, 1)
		}

		gocv.PutText(&areaResult, "Detection Area",
			image.Pt(10, 20),
			gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)
		appendImage(&window3Display, areaResult, false)
	} else {
		if printProgress {
			fmt.Println("Avatar not found, skipping bar detection")
		}
		noAvatarResult := img_roi.Clone()
		defer noAvatarResult.Close()
		gocv.PutText(&noAvatarResult, "No Avatar Found",
			image.Pt(10, 20),
			gocv.FontHersheyPlain, 1.2, color.RGBA{0, 0, 255, 255}, 2)
		appendImage(&window3Display, noAvatarResult, false)
	}

	window3.IMShow(window3Display)

	// ============================================================================
	// Window 4: Morphological Operations on Bar Area
	// ============================================================================
	window4Display := gocv.NewMat()
	defer window4Display.Close()

	var barAreaBinary gocv.Mat
	var barAreaMorphed gocv.Mat

	if circleFound && barAreaValid {
		// Validate bar area coordinates against img_morphed dimensions
		binaryWidth := img_morphed.Cols()
		binaryHeight := img_morphed.Rows()

		barLeft := barArea.Min.X
		barTop := barArea.Min.Y
		barRight := barArea.Max.X
		barBottom := barArea.Max.Y

		if barLeft < 0 {
			barLeft = 0
		}
		if barTop < 0 {
			barTop = 0
		}
		if barRight > binaryWidth {
			barRight = binaryWidth
		}
		if barBottom > binaryHeight {
			barBottom = binaryHeight
		}

		// Check if adjusted bar area is still valid
		if barLeft >= barRight || barTop >= barBottom {
			fmt.Printf("Error: bar area out of bounds: (%d,%d) to (%d,%d), binary size: %dx%d\n",
				barLeft, barTop, barRight, barBottom, binaryWidth, binaryHeight)
			barAreaValid = false
		}

		if barAreaValid {
			// Extract bar area from morphed image (not binary)
			barArea = image.Rect(barLeft, barTop, barRight, barBottom)
			barAreaBinary = img_morphed.Region(barArea)
			defer barAreaBinary.Close()

			if barAreaBinary.Empty() {
				fmt.Println("Error: bar area extraction resulted in empty mat")
				barAreaValid = false
			}
		}
	}

	if circleFound && barAreaValid {

		barAreaBinary_bgr := gocv.NewMat()
		defer barAreaBinary_bgr.Close()
		gocv.CvtColor(barAreaBinary, &barAreaBinary_bgr, gocv.ColorGrayToBGR)
		gocv.PutText(&barAreaBinary_bgr, "Bar Area Morphed",
			image.Pt(5, 15),
			gocv.FontHersheyPlain, 1.0, color.RGBA{0, 255, 0, 255}, 1)
		window4Display = barAreaBinary_bgr.Clone()

		// Morphological operations: dilate then erode
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

		dilated := gocv.NewMat()
		defer dilated.Close()
		gocv.Dilate(barAreaBinary, &dilated, kernel)

		dilated_bgr := gocv.NewMat()
		defer dilated_bgr.Close()
		gocv.CvtColor(dilated, &dilated_bgr, gocv.ColorGrayToBGR)
		gocv.PutText(&dilated_bgr, "Dilated",
			image.Pt(5, 15),
			gocv.FontHersheyPlain, 1.0, color.RGBA{0, 255, 0, 255}, 1)
		appendImage(&window4Display, dilated_bgr, false)

		barAreaMorphed = gocv.NewMat()
		defer barAreaMorphed.Close()
		gocv.Erode(dilated, &barAreaMorphed, kernel)

		morphed_bgr := gocv.NewMat()
		defer morphed_bgr.Close()
		gocv.CvtColor(barAreaMorphed, &morphed_bgr, gocv.ColorGrayToBGR)
		gocv.PutText(&morphed_bgr, "Eroded",
			image.Pt(5, 15),
			gocv.FontHersheyPlain, 1.0, color.RGBA{0, 255, 0, 255}, 1)
		appendImage(&window4Display, morphed_bgr, false)

		if printProgress {
			fmt.Println("Morphological operations completed")
		}
	} else {
		noDataImg := gocv.NewMatWithSize(100, 300, gocv.MatTypeCV8UC3)
		defer noDataImg.Close()
		gocv.PutText(&noDataImg, "No bar area",
			image.Pt(10, 50),
			gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 255, 255}, 1)
		window4Display = noDataImg.Clone()
	}

	window4.IMShow(window4Display)

	// ============================================================================
	// Window 5: Bar Detection and Results
	// ============================================================================
	var targetBars []TargetBarInfo

	if circleFound && barAreaValid && !barAreaMorphed.Empty() {
		// Find contours in morphed bar area
		barContours := gocv.FindContours(barAreaMorphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
		defer barContours.Close()

		if printProgress {
			fmt.Printf("Found %d contours in bar area\n", barContours.Size())
		}

		// Extract bars based on size constraints
		var bars []image.Rectangle
		for i := 0; i < barContours.Size(); i++ {
			contour := barContours.At(i)
			rect := gocv.BoundingRect(contour)

			if printProgress {
				fmt.Printf("  Contour %d: w=%d h=%d\n", i, rect.Dx(), rect.Dy())
			}

			// Check if matches bar constraints
			if rect.Dx() >= barParams.MinWidth && rect.Dx() <= barParams.MaxWidth &&
				rect.Dy() >= barParams.MinHeight && rect.Dy() <= barParams.MaxHeight {
				// Convert to absolute coordinates
				absoluteRect := image.Rect(
					rect.Min.X+barArea.Min.X,
					rect.Min.Y+barArea.Min.Y,
					rect.Max.X+barArea.Min.X,
					rect.Max.Y+barArea.Min.Y,
				)
				bars = append(bars, absoluteRect)
				if printProgress {
					fmt.Printf("    -> Bar matched: (%d,%d) size %dx%d\n",
						absoluteRect.Min.X, absoluteRect.Min.Y, absoluteRect.Dx(), absoluteRect.Dy())
				}
			}
		}

		// Sort bars from top to bottom
		for i := 0; i < len(bars); i++ {
			for j := i + 1; j < len(bars); j++ {
				if bars[i].Min.Y > bars[j].Min.Y {
					bars[i], bars[j] = bars[j], bars[i]
				}
			}
		}

		// Process HP and MP bars
		if len(bars) >= 2 {
			// Convert to HSV for color detection
			img_hsv := gocv.NewMat()
			defer img_hsv.Close()
			gocv.CvtColor(img_roi, &img_hsv, gocv.ColorBGRToHSV)

			barTypes := []string{"HP", "MP"}
			for i := 0; i < 2 && i < len(bars); i++ {
				barRect := bars[i]
				barType := barTypes[i]
				hsvRange := hsvRanges[i]

				// Validate bar rect coordinates against img_hsv dimensions
				hsvWidth := img_hsv.Cols()
				hsvHeight := img_hsv.Rows()

				barLeft := barRect.Min.X
				barTop := barRect.Min.Y
				barRight := barRect.Max.X
				barBottom := barRect.Max.Y

				if barLeft < 0 {
					barLeft = 0
				}
				if barTop < 0 {
					barTop = 0
				}
				if barRight > hsvWidth {
					barRight = hsvWidth
				}
				if barBottom > hsvHeight {
					barBottom = hsvHeight
				}

				// Check if adjusted bar rect is still valid
				if barLeft >= barRight || barTop >= barBottom {
					fmt.Printf("Warning: bar %d out of bounds, skipping\n", i)
					continue
				}

				// Extract bar region from HSV
				barRect = image.Rect(barLeft, barTop, barRight, barBottom)
				barROI := img_hsv.Region(barRect)
				defer barROI.Close()

				if barROI.Empty() {
					fmt.Printf("Warning: bar ROI %d is empty, skipping\n", i)
					continue
				}

				// Create mask for the specific color range
				lower := gocv.NewScalar(float64(hsvRange.HMin), float64(hsvRange.SMin), float64(hsvRange.VMin), 0)
				upper := gocv.NewScalar(float64(hsvRange.HMax), float64(hsvRange.SMax), float64(hsvRange.VMax), 0)
				mask := gocv.NewMat()
				defer mask.Close()
				gocv.InRangeWithScalar(barROI, lower, upper, &mask)

				// Find the rightmost white pixel
				fillWidth := 0
				colSums := gocv.NewMat()
				defer colSums.Close()
				gocv.Reduce(mask, &colSums, 0, gocv.ReduceSum, gocv.MatTypeCV32F)

				for x := colSums.Cols() - 1; x >= 0; x-- {
					if colSums.GetFloatAt(0, x) > 0 {
						fillWidth = x + 1
						break
					}
				}

				percentage := float64(fillWidth) / float64(barRect.Dx()) * 100

				targetBars = append(targetBars, TargetBarInfo{
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

		// Display window 5
		window5Display := img_roi.Clone()
		defer window5Display.Close()

		// Draw avatar rectangle
		gocv.Rectangle(&window5Display, avatarRect, color.RGBA{0, 255, 0, 255}, 2)
		// Draw center point
		gocv.Circle(&window5Display, circleCenter, 3, color.RGBA{0, 0, 255, 255}, -1)

		// Draw bar area
		gocv.Rectangle(&window5Display, barArea, color.RGBA{255, 255, 0, 255}, 1)

		// Draw detected bars
		colors := []color.RGBA{
			{255, 0, 0, 255}, // HP - Red
			{0, 0, 255, 255}, // MP - Blue
		}

		for i, bar := range targetBars {
			if i < len(colors) {
				gocv.Rectangle(&window5Display, bar.Rect, colors[i], 2)
				text := fmt.Sprintf("%s: %.1f%%", bar.Type, bar.Percentage)
				gocv.PutText(&window5Display, text,
					image.Pt(bar.Rect.Min.X, bar.Rect.Min.Y-5),
					gocv.FontHersheyPlain, 0.8, colors[i], 1)

				// Draw fill indicator
				if bar.FillWidth > 0 {
					fillRect := image.Rect(
						bar.Rect.Min.X,
						bar.Rect.Min.Y,
						bar.Rect.Min.X+bar.FillWidth,
						bar.Rect.Max.Y,
					)
					gocv.Rectangle(&window5Display, fillRect, colors[i], 1)
				}
			}
		}

		gocv.PutText(&window5Display, fmt.Sprintf("Bars: %d", len(targetBars)),
			image.Pt(10, 20),
			gocv.FontHersheyPlain, 1.2, color.RGBA{0, 255, 0, 255}, 2)

		window5.IMShow(window5Display)
	} else {
		noResultImg := gocv.NewMatWithSize(100, 300, gocv.MatTypeCV8UC3)
		defer noResultImg.Close()
		gocv.PutText(&noResultImg, "No bars detected",
			image.Pt(10, 50),
			gocv.FontHersheyPlain, 1.0, color.RGBA{255, 255, 255, 255}, 1)
		window5.IMShow(noResultImg)
	}

	return targetBars
}

// runDetection4 runs the target detection algorithm with all debug windows
func runDetection4(useStaticImage bool, staticMat gocv.Mat, browser *DebugBrowser, statusImagePath string) {
	// Create 5 windows
	window1 := gocv.NewWindow("1-ROI & Binary")
	defer window1.Close()

	window2 := gocv.NewWindow("2-Circle Detection")
	defer window2.Close()

	window3 := gocv.NewWindow("3-Bar Area")
	defer window3.Close()

	window4 := gocv.NewWindow("4-Morphology")
	defer window4.Close()

	window5 := gocv.NewWindow("5-Bar Results")
	defer window5.Close()

	// Create morph parameters with initial values
	morphParams := &MorphParams{
		UseAdaptive:    1,   // Use adaptive threshold
		VThreshold:     128, // Not used when adaptive
		AdaptiveMethod: 1,   // Gaussian
		BlockSize:      40,  // Block size for adaptive threshold
		CValue:         10,  // C value for adaptive threshold
		MorphWidth:     3,   // Width for morphological kernel
		MorphHeight:    3,   // Height for morphological kernel
	}

	// Create trackbars for morphology parameters on window 1
	window1.CreateTrackbarWithValue("Use Adaptive (0/1)", &morphParams.UseAdaptive, 1)
	window1.CreateTrackbarWithValue("Adaptive Method (0/1)", &morphParams.AdaptiveMethod, 1)
	window1.CreateTrackbarWithValue("Block Size", &morphParams.BlockSize, 199)
	window1.CreateTrackbarWithValue("C Value", &morphParams.CValue, 50)
	window1.CreateTrackbarWithValue("Morph Width", &morphParams.MorphWidth, 50)
	window1.CreateTrackbarWithValue("Morph Height", &morphParams.MorphHeight, 50)

	// Create ROI parameters
	roi := &ROIParams4{
		MinX: 35,
		MinY: 660,
		MaxX: 450,
		MaxY: 950,
	}

	// Create trackbars for ROI on window 1
	window1.CreateTrackbarWithValue("ROI MinX", &roi.MinX, 1920)
	window1.CreateTrackbarWithValue("ROI MinY", &roi.MinY, 1080)
	window1.CreateTrackbarWithValue("ROI MaxX", &roi.MaxX, 1920)
	window1.CreateTrackbarWithValue("ROI MaxY", &roi.MaxY, 1080)

	// Create avatar detection parameters
	circleParams := &CircleParams{
		MinSize: 40,
		MaxSize: 70,
	}

	// Create trackbars for avatar detection on window 2
	window2.CreateTrackbarWithValue("Min Size", &circleParams.MinSize, 200)
	window2.CreateTrackbarWithValue("Max Size", &circleParams.MaxSize, 200)

	// Create bar area parameters
	barAreaParams := &BarAreaParams{
		LeftOffset:   5,   // 5 pixels from circle right edge
		RightOffset:  200, // Extend 200 pixels from circle right edge
		TopOffset:    0,   // Align with circle top
		BottomOffset: 0,   // Align with circle bottom
	}

	// Create trackbars for bar area on window 3
	window3.CreateTrackbarWithValue("Left Offset", &barAreaParams.LeftOffset, 100)
	window3.CreateTrackbarWithValue("Right Offset", &barAreaParams.RightOffset, 300)
	window3.CreateTrackbarWithValue("Top Offset", &barAreaParams.TopOffset, 100)
	window3.CreateTrackbarWithValue("Bottom Offset", &barAreaParams.BottomOffset, 100)

	// Create bar detection parameters
	barParams := &BarParams{
		MinWidth:  50,
		MaxWidth:  200,
		MinHeight: 5,
		MaxHeight: 20,
	}

	// Create trackbars for bar constraints on window 5
	window5.CreateTrackbarWithValue("Min Width", &barParams.MinWidth, 300)
	window5.CreateTrackbarWithValue("Max Width", &barParams.MaxWidth, 300)
	window5.CreateTrackbarWithValue("Min Height", &barParams.MinHeight, 50)
	window5.CreateTrackbarWithValue("Max Height", &barParams.MaxHeight, 50)

	// Create HSV range parameters for HP and MP
	hpRange := &HSVRange{HMin: 160, HMax: 180, SMin: 100, SMax: 255, VMin: 100, VMax: 255}
	mpRange := &HSVRange{HMin: 90, HMax: 130, SMin: 100, SMax: 255, VMin: 100, VMax: 255}
	hsvRanges := [2]*HSVRange{hpRange, mpRange}

	// Create trackbars for HP on window 5
	window5.CreateTrackbarWithValue("HP H Min", &hpRange.HMin, 180)
	window5.CreateTrackbarWithValue("HP H Max", &hpRange.HMax, 180)
	window5.CreateTrackbarWithValue("HP S Min", &hpRange.SMin, 255)
	window5.CreateTrackbarWithValue("HP S Max", &hpRange.SMax, 255)
	window5.CreateTrackbarWithValue("HP V Min", &hpRange.VMin, 255)
	window5.CreateTrackbarWithValue("HP V Max", &hpRange.VMax, 255)

	// Create trackbars for MP on window 5
	window5.CreateTrackbarWithValue("MP H Min", &mpRange.HMin, 180)
	window5.CreateTrackbarWithValue("MP H Max", &mpRange.HMax, 180)
	window5.CreateTrackbarWithValue("MP S Min", &mpRange.SMin, 255)
	window5.CreateTrackbarWithValue("MP S Max", &mpRange.SMax, 255)
	window5.CreateTrackbarWithValue("MP V Min", &mpRange.VMin, 255)
	window5.CreateTrackbarWithValue("MP V Max", &mpRange.VMax, 255)

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

	fmt.Println("=== Contour-Based Target Detection ===")
	fmt.Println("Windows:")
	fmt.Println("  1: ROI → Binary → Dilate → Erode (for avatar detection)")
	fmt.Println("  2: Avatar Detection (Contour on morphed image)")
	fmt.Println("  3: Bar Detection Area (based on avatar)")
	fmt.Println("  4: Bar Area Morphology (for bar detection)")
	fmt.Println("  5: Bar Detection Results (HP/MP)")
	fmt.Println("\nControls:")
	fmt.Println("  's': Save current frame to status.jpeg (browser mode only)")
	fmt.Println("  'q' or ESC: Quit")

	for {
		// Add panic recovery for the main loop
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Recovered from panic in main loop: %v\n", r)
				}
			}()

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
						return
					}
					matInitialized = true
					originalMat = mat.Clone()
				}

				if !matInitialized {
					key := window1.WaitKey(100)
					if key == 'q' || key == 27 {
						return
					}
					return
				}
			}

			// Detect and display
			detectStatusBars4(mat, window1, window2, window3, window4, window5, roi, morphParams, circleParams, barAreaParams, barParams, hsvRanges, false)
		}()

		key := window1.WaitKey(100)
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
