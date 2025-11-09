// Package main - debug.go
//
// Debug window manager for image visualization.
// Handles UI operations on the main thread to comply with macOS requirements.
package main

import (
	"sync"

	"gocv.io/x/gocv"
)

// DebugWindowSet holds the three images and one window for a debug session
type DebugWindowSet struct {
	Original gocv.Mat
	Mask     gocv.Mat
	Result   gocv.Mat
	Window   *gocv.Window
}

// DebugUpdate notification for window updates
type DebugUpdate struct {
	Name string
}

// Debug manages debug windows and image updates
type Debug struct {
	Enable  bool
	Stat    *Stat
	Chan    chan DebugUpdate
	Windows map[string]*DebugWindowSet
	mu      sync.Mutex // Protects Windows map and Mat data
}

// NewDebug creates a new Debug instance
func NewDebug(stat *Stat) *Debug {
	d := &Debug{
		Enable:  stat.Debug,
		Stat:    stat,
		Chan:    make(chan DebugUpdate, 10),
		Windows: make(map[string]*DebugWindowSet),
	}
	return d
}

// CreateDebug creates debug windows
// MUST be called from the main thread
func (d *Debug) CreateDebug() {
	if !d.Enable {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Create windows for each detection type
	// User can configure which ones to create based on needs
	windowNames := []string{
		"MyHP",
		"MyMP",
		"MyFP",
		// "TargetHP",
		// "TargetMP",
		// "TargetFP",
		// "Aggressive",
		// "Passive",
		// "Violet",
	}

	for _, name := range windowNames {
		d.Windows[name] = &DebugWindowSet{
			Original: gocv.NewMat(),
			Mask:     gocv.NewMat(),
			Result:   gocv.NewMat(),
			Window:   gocv.NewWindow(name),
		}
	}
}

// ProcessUpdates processes pending image updates
// MUST be called from the main thread
func (d *Debug) ProcessUpdates() {
	if !d.Enable {
		return
	}

	// Process all pending updates non-blockingly
	for {
		select {
		case update := <-d.Chan:
			d.displayUpdate(update.Name)
		default:
			return
		}
	}
}

// SendUpdate sends image update to the main thread
// Can be called from any goroutine
func (d *Debug) SendUpdate(name string, original, mask, result gocv.Mat) {
	if !d.Enable {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if window exists in map
	windowSet, exists := d.Windows[name]
	if !exists {
		return
	}

	// Update the stored images
	original.CopyTo(&windowSet.Original)
	mask.CopyTo(&windowSet.Mask)
	result.CopyTo(&windowSet.Result)

	// Notify main thread to update display
	select {
	case d.Chan <- DebugUpdate{Name: name}:
	default:
		// Channel full, skip notification
	}
}

// displayUpdate displays the images for a window by vertically concatenating them
// MUST be called from the main thread
func (d *Debug) displayUpdate(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	windowSet, exists := d.Windows[name]
	if !exists {
		return
	}

	// Vertically concatenate the three images
	combined := gocv.NewMat()
	defer combined.Close()

	images := []gocv.Mat{}
	if !windowSet.Original.Empty() {
		images = append(images, windowSet.Original)
	}
	if !windowSet.Mask.Empty() {
		images = append(images, windowSet.Mask)
	}
	if !windowSet.Result.Empty() {
		images = append(images, windowSet.Result)
	}

	if len(images) > 0 {
		gocv.Vconcat(images[0], images[1], &combined)
		if len(images) > 2 {
			temp := gocv.NewMat()
			defer temp.Close()
			gocv.Vconcat(combined, images[2], &temp)
			temp.CopyTo(&combined)
		}
		windowSet.Window.IMShow(combined)
	}

	// Wait for window events to process
	gocv.WaitKey(1)
}

// Close closes all debug windows
// MUST be called from the main thread
func (d *Debug) Close() {
	if !d.Enable {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, windowSet := range d.Windows {
		if windowSet.Window != nil {
			windowSet.Window.Close()
		}
		windowSet.Original.Close()
		windowSet.Mask.Close()
		windowSet.Result.Close()
	}

	close(d.Chan)
}
