// Package main - browser.go
//
// This file manages the browser controller for game interaction.
// It provides browser lifecycle management, JavaScript injection, and game actions.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Browser manages the chromedp browser instance
type Browser struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCtx    context.Context
	allocCancel context.CancelFunc
	frameChan   chan *image.RGBA
}

// EvalJS contains the JavaScript code to inject into the game page
const EvalJS = `
const client = document.querySelector('canvas')
const input = document.querySelector('input')
const DEBUG = false
function addTargetMarker(color = 'red', x = 0, y = 0,) {
    if (!DEBUG) return
    const targetMarker = document.createElement('div')
    const targetMarkerStyle = ` + "`position: fixed; width: 2px; height: 2px; background-color: ${color}; border-radius: 50%;z-index: 9999;left: ${x}px;top: ${y}px;`" + `
    targetMarker.style = targetMarkerStyle
    document.body.appendChild(targetMarker)

    setTimeout(() => {
        targetMarker.remove()
    }, 1000)
}

function isMob() {
    return document.body.style.cursor.indexOf('curattack') > 0
}
function dispatchEvent(event) {
    return client.dispatchEvent(event)
}

function after(duration = 0, callback) {
    setTimeout(callback, duration)
}

let checkMobTimeout = null;
function mouseEvent(type, x, y, { checkMob = false, delay = 50, duration } = {}) {
    if (checkMobTimeout) {
        clearTimeout(checkMobTimeout)
        checkMobTimeout = null
    }
    function waitDuration(type) {
        if (duration) {
            after(duration, () => {
                dispatchEvent(new MouseEvent(type ?? 'mouseup', { clientX: x, clientY: y }))
            })
        } else if (type) {
            dispatchEvent(new MouseEvent(type, { key }))
        }
    }
    switch (type) {
        case 'move':
            dispatchEvent(new MouseEvent('mousemove', { clientX: x, clientY: y }))
            break;
        case 'press':
            dispatchEvent(new MouseEvent('mousedown', { clientX: x, clientY: y }))
            waitDuration('mouseup')
            break;
        case 'hold':
            dispatchEvent(new MouseEvent('mousedown', { clientX: x, clientY: y }))
            waitDuration()
            break;
        case 'release':
            dispatchEvent(new MouseEvent('mouseup', { clientX: x, clientY: y }))
            break;
        case 'moveClick':
            dispatchEvent(new MouseEvent('mousemove', { clientX: x, clientY: y }))

            if (checkMob) {
                checkMobTimeout = setTimeout(() => {
                    if (isMob()) {
                        dispatchEvent(new MouseEvent('mousedown', { clientX: x, clientY: y }))
                        dispatchEvent(new MouseEvent('mouseup', { clientX: x, clientY: y }))
                        addTargetMarker('green', x, y)
                    } else {
                        addTargetMarker('red', x, y)
                    }
                }, delay)
            } else if (!checkMob) {
                addTargetMarker('blue', x, y)
                dispatchEvent(new MouseEvent('mousedown', { clientX: x, clientY: y }))
                dispatchEvent(new MouseEvent('mouseup', { clientX: x, clientY: y }))
            }
            break;
    }
}
function keyboardEvent(keyMode, key, duration = null) {
    function waitDuration(type) {
        if (duration) {
            setTimeout(() => {
                dispatchEvent(new KeyboardEvent(type ?? 'keyup', { key }))
            }, duration)
        } else if (type) {
            dispatchEvent(new KeyboardEvent(type, { key }))
        }
    }
    switch (keyMode) {
        case 'press':
            dispatchEvent(new KeyboardEvent('keydown', { key }))
            waitDuration('keyup')
            break;
        case 'hold':
            dispatchEvent(new KeyboardEvent('keydown', { key }))
            waitDuration()
            break;
        case 'release':
            dispatchEvent(new KeyboardEvent('keyup', { key }))
            break;
    }
}

function sendSlot(slotBarIndex, slotIndex) {
    keyboardEvent('press', ` + "`F${slotBarIndex + 1}`" + `)
    keyboardEvent('press', slotIndex)
}

function setInputChat(text) {
    input.value = text
    input.select()
}
`

// NewBrowser creates a new browser instance
func NewBrowser() *Browser {
	return &Browser{
		frameChan: make(chan *image.RGBA, 1), // Buffer of 1 to hold latest frame
	}
}

// Start initializes the browser and loads the game
func (b *Browser) Start(cfg *Config) error {
	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.WindowSize(800, 600),
	)

	b.allocCtx, b.allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)

	// Create context with browser log output
	contextOpts := []chromedp.ContextOption{}
	if cfg.BrowserLogFile != nil {
		// Redirect chromedp logs to browser log file
		contextOpts = append(contextOpts,
			chromedp.WithLogf(cfg.BrowserLog),
			chromedp.WithErrorf(cfg.BrowserLog),
		)
	}
	b.ctx, b.cancel = chromedp.NewContext(b.allocCtx, contextOpts...)

	// Start screencast BEFORE navigation
	cfg.Log("Setting up screencast listener...")
	b.setupScreencastListener(cfg)

	// Get cookies from config
	cookies := cfg.Cookies

	// Set cookies before navigation
	if len(cookies) > 0 {
		cfg.Log("Setting %d cookies before navigation", len(cookies))
		err := b.setCookies(cookies)
		if err != nil {
			cfg.Log("Warning: failed to set cookies: %v", err)
		}
	}

	// Navigate to game (don't wait for full page load)
	cfg.Log("Navigating to https://universe.flyff.com/play")
	err := chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, _, _, err := page.Navigate("https://universe.flyff.com/play").Do(ctx)
			return err
		}),
	)

	if err != nil {
		cfg.Log("Navigation error: %v", err)
		return err
	}

	// Give it a moment to start loading
	cfg.Log("Waiting for page to start loading...")
	time.Sleep(2 * time.Second)

	// Start screencast after page loads
	cfg.Log("Starting screencast stream...")
	err = chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return page.StartScreencast().
				WithFormat("jpeg").
				WithQuality(70).
				Do(ctx)
		}),
	)
	if err != nil {
		cfg.Log("Failed to start screencast: %v", err)
		return err
	}

	// Inject JavaScript
	err = b.InjectJS()
	if err != nil {
		cfg.Log("Warning: failed to inject JS: %v", err)
	}

	cfg.Log("Browser started successfully")
	return nil
}

// setupScreencastListener sets up the event listener for screencast frames
func (b *Browser) setupScreencastListener(cfg *Config) {
	frameCount := 0
	// Listen for screencast frames
	chromedp.ListenTarget(b.ctx, func(ev interface{}) {
		if ev, ok := ev.(*page.EventScreencastFrame); ok {
			frameCount++
			if frameCount%30 == 1 { // Log every 30th frame
				cfg.Log("Received screencast frame #%d", frameCount)
			}

			// Process frame in goroutine to avoid blocking event listener
			go func(frameData string, sessionID int64) {
				// Decode the frame
				data, err := base64.StdEncoding.DecodeString(frameData)
				if err != nil {
					cfg.Log("Failed to decode frame: %v", err)
					return
				}

				// Decode image
				img, _, err := image.Decode(bytes.NewReader(data))
				if err != nil {
					cfg.Log("Failed to decode image: %v", err)
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

				// Send frame to channel (blocking until Capture() takes it)
				b.frameChan <- rgba

				// After frame is consumed by Capture(), acknowledge to Chrome
				// This way Chrome won't send next frame until this one is consumed
				chromedp.Run(b.ctx, page.ScreencastFrameAck(sessionID))
			}(ev.Data, ev.SessionID)
		}
	})
}

// Capture returns the latest frame from the screencast stream
func (b *Browser) Capture() (*image.RGBA, error) {
	if b.ctx == nil || b.ctx.Err() != nil {
		return nil, fmt.Errorf("browser context is invalid")
	}

	select {
	case frame := <-b.frameChan:
		return frame, nil
	default:
		return nil, fmt.Errorf("no frame available")
	}
}

// SaveCookie saves browser cookies to config
func (b *Browser) SaveCookie(cfg *Config) error {
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
		cfg.Log("Failed to get cookies: %v", err)
		return err
	}

	// Convert to config cookie format
	cfg.mu.Lock()
	cfg.Cookies = make([]Cookie, len(cookies))
	for i, c := range cookies {
		cfg.Cookies[i] = Cookie{
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
	cfg.mu.Unlock()

	// Save to file
	err = cfg.SaveCookies()
	if err != nil {
		cfg.Log("Failed to save cookies to file: %v", err)
		return err
	}

	cfg.Log("Saved %d cookies", len(cookies))
	return nil
}

// Refresh reloads the current page (for reconnection)
func (b *Browser) Refresh(cfg *Config) error {
	if b.ctx == nil || b.ctx.Err() != nil {
		return fmt.Errorf("browser context is invalid")
	}

	err := chromedp.Run(b.ctx,
		chromedp.Reload(),
	)

	if err != nil {
		return err
	}

	// Wait for page to reload
	time.Sleep(2 * time.Second)

	// Restart screencast after reload
	cfg.Log("Restarting screencast stream...")
	err = chromedp.Run(b.ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return page.StartScreencast().
				WithFormat("jpeg").
				WithQuality(70).
				Do(ctx)
		}),
	)
	if err != nil {
		cfg.Log("Failed to restart screencast: %v", err)
		return err
	}

	// Re-inject JavaScript
	return b.InjectJS()
}

// Stop closes the browser
func (b *Browser) Stop() {
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

// InjectJS injects the eval.js script into the page
func (b *Browser) InjectJS() error {
	if b.ctx == nil || b.ctx.Err() != nil {
		return fmt.Errorf("browser context is invalid")
	}

	return chromedp.Run(b.ctx,
		chromedp.Evaluate(EvalJS, nil),
	)
}

// Eval executes custom JavaScript code
func (b *Browser) Eval(js string) error {
	if b.ctx == nil || b.ctx.Err() != nil {
		return fmt.Errorf("browser context is invalid")
	}

	return chromedp.Run(b.ctx,
		chromedp.Evaluate(js, nil),
	)
}

// SimpleClick performs a simple click at the given coordinates
func (b *Browser) SimpleClick(x, y int) error {
	js := fmt.Sprintf("mouseEvent('moveClick', %d, %d);", x, y)
	return b.Eval(js)
}

// SendMessage sets the chat input text
func (b *Browser) SendMessage(text string) error {
	js := fmt.Sprintf("setInputChat('%s')", text)
	return b.Eval(js)
}

// SendSlot sends a slot action (page + slot)
func (b *Browser) SendSlot(page, slot int) error {
	// page is 1-9, convert to 0-8 for slotBarIndex
	slotBarIndex := page - 1
	js := fmt.Sprintf("sendSlot(%d, %d)", slotBarIndex, slot)
	return b.Eval(js)
}

// SendKey sends a keyboard event
func (b *Browser) SendKey(key string, mode string) error {
	// mode can be: "press", "hold", "release"
	js := fmt.Sprintf("keyboardEvent('%s', '%s');", mode, key)
	return b.Eval(js)
}

// setCookies is an internal helper to set cookies in the browser
func (b *Browser) setCookies(cookies []Cookie) error {
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
