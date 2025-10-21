// Package main - tray.go
//
// This file implements the system tray UI that provides user configuration interface.
// Uses getlantern/systray library for cross-platform tray menu support.
//
// Menu Structure:
//   Flyff Bot
//   ├─ Status: Mode | Kills | KPM | Uptime (read-only, updates every iteration)
//   ├─ Mode Selection
//   │  ├─ Stop (idle, recognition continues)
//   │  ├─ Farming (autonomous mob hunting)
//   │  ├─ Support (party healing/buffing)
//   │  └─ Shouting (auto-chat, not implemented)
//   ├─ Slots (3-level: Slots → Slot Type → 0-9)
//   │  ├─ Attack Slots (checkboxes for slots 0-9)
//   │  ├─ Heal Slots
//   │  ├─ Buff Slots
//   │  ├─ MP Restore Slots
//   │  ├─ FP Restore Slots
//   │  └─ Pickup Slots
//   ├─ Thresholds (3-level: Thresholds → Type → 0%-100%)
//   │  ├─ HP Threshold (radio buttons 0-100 in 10% increments)
//   │  ├─ MP Threshold
//   │  └─ FP Threshold
//   ├─ Capture Frequency
//   │  ├─ Continuous (0ms)
//   │  ├─ 1 Second (default)
//   │  ├─ 2 Seconds
//   │  ├─ 3 Seconds
//   │  └─ 4 Seconds
//   ├─ Statistics (read-only, placeholder for future)
//   └─ Quit (graceful shutdown)
//
// Concurrency Model:
// The tray spawns 60+ goroutines for event handling:
//   - 60 slot click handlers (6 slot types × 10 slots each)
//   - 33 threshold handlers (3 types × 11 percentages each)
//   - 5 capture frequency handlers
//   - 1 mode selection handler
//   - 1 quit handler
//
// Auto-Save:
// All configuration changes trigger immediate SaveState() to persist settings.
//
// Lifecycle:
//   1. NewTrayApp: Create instance with bot reference
//   2. Run: Start systray (blocking call)
//   3. onReady: Initialize menu structure
//   4. handleEvents: Listen for user interactions (infinite loop)
//   5. onExit: Clean up and save final state
package main

import (
	"fmt"
	"os"

	"github.com/getlantern/systray"
)

// TrayApp manages the system tray application and user interface.
//
// The TrayApp creates and manages a system tray icon with nested menus for
// configuration. It handles user interactions and updates the bot configuration
// accordingly, with immediate persistence to data.json.
//
// Menu Item Arrays:
// Uses fixed-size arrays to store menu item references for each configurable
// option (slots 0-9, thresholds 0-100%, etc.), enabling efficient event handling
// and checkmark updates.
type TrayApp struct {
	bot       *Bot
	onExit    func()

	// Menu items
	statusItem    *systray.MenuItem

	// Mode items
	stopItem      *systray.MenuItem
	farmingItem   *systray.MenuItem
	supportItem   *systray.MenuItem
	shoutingItem  *systray.MenuItem

	// Slot config items - parent menus
	attackSlotsItem    *systray.MenuItem
	healSlotsItem      *systray.MenuItem
	buffSlotsItem      *systray.MenuItem
	mpRestoreSlotsItem *systray.MenuItem
	fpRestoreSlotsItem *systray.MenuItem
	pickupSlotsItem    *systray.MenuItem
	pickupPetItem      *systray.MenuItem // Parent menu for pickup and pet

	// Slot submenu items (for each slot type, we have 10 slot options: 0-9)
	attackSlotItems    [10]*systray.MenuItem
	healSlotItems      [10]*systray.MenuItem
	buffSlotItems      [10]*systray.MenuItem
	mpRestoreSlotItems [10]*systray.MenuItem
	fpRestoreSlotItems [10]*systray.MenuItem
	pickupSlotItems    [10]*systray.MenuItem
	pickupPetSlotItems [10]*systray.MenuItem // Pickup pet slot selection (radio)
	pickupMotionSlotItems [10]*systray.MenuItem // Pickup motion slot selection (radio)

	// Threshold items - parent menus
	hpThresholdItem *systray.MenuItem
	mpThresholdItem *systray.MenuItem
	fpThresholdItem *systray.MenuItem

	// Threshold submenu items (0-100% in 10% increments: 11 options)
	hpThresholdItems [11]*systray.MenuItem
	mpThresholdItems [11]*systray.MenuItem
	fpThresholdItems [11]*systray.MenuItem

	// Capture frequency items
	captureFreqItem      *systray.MenuItem
	captureFreqItems     [5]*systray.MenuItem // Continuous, 1s, 2s, 3s, 4s

	// Slot cooldown configuration
	slotCooldownItem     *systray.MenuItem
	slotCooldownSlots    [10]*systray.MenuItem // Slot 0-9 selection
	slotCooldownTimes    [21]*systray.MenuItem // Current cooldown time options for selected slot
}

// NewTrayApp creates a new tray application
func NewTrayApp(bot *Bot) *TrayApp {
	return &TrayApp{
		bot: bot,
	}
}

// Run starts the tray application
func (t *TrayApp) Run() {
	LogInfo("Starting system tray application")
	systray.Run(t.onReady, func() {
		LogInfo("System tray onExit callback triggered")
		if t.bot != nil {
			LogInfo("Stopping bot from tray exit")
			t.bot.StopBehavior()
			LogInfo("Closing browser from tray exit")
			t.bot.browser.Close()
		}
		LogInfo("System tray exit complete")
	})
	LogInfo("System tray Run() returned")
}

// onReady is called when the tray is ready
func (t *TrayApp) onReady() {
	systray.SetTitle("Flyff Bot")
	systray.SetTooltip("Flyff Universe Bot")

	// TODO: Set icon
	// systray.SetIcon(iconData)

	// Status (read-only)
	t.statusItem = systray.AddMenuItem("Status: Starting...", "Current bot status")
	t.statusItem.Disable()

	systray.AddSeparator()

	// Mode selection
	modeMenu := systray.AddMenuItem("Mode", "Select bot mode")
	t.stopItem = modeMenu.AddSubMenuItem("Stop", "Stop all actions")
	t.farmingItem = modeMenu.AddSubMenuItem("Farming", "Farm mobs automatically")
	t.supportItem = modeMenu.AddSubMenuItem("Support", "Support party members")
	t.shoutingItem = modeMenu.AddSubMenuItem("Shouting", "Auto shout in chat")
	t.farmingItem.Check() // Default mode

	systray.AddSeparator()

	// Slot configuration - with 3-level menu (Slots -> Slot Type -> 0-9)
	slotsMenu := systray.AddMenuItem("Slots", "Configure skill slots")
	t.attackSlotsItem = slotsMenu.AddSubMenuItem("Attack Slots", "Configure attack skill slots")
	t.healSlotsItem = slotsMenu.AddSubMenuItem("Heal Slots", "Configure heal skill slots")
	t.buffSlotsItem = slotsMenu.AddSubMenuItem("Buff Slots", "Configure buff skill slots")
	t.mpRestoreSlotsItem = slotsMenu.AddSubMenuItem("MP Restore Slots", "Configure MP restore slots")
	t.fpRestoreSlotsItem = slotsMenu.AddSubMenuItem("FP Restore Slots", "Configure FP restore slots")
	t.pickupSlotsItem = slotsMenu.AddSubMenuItem("Pickup Slots", "Configure pickup slots")

	// Create slot number submenus (0-9 for each slot type)
	for i := 0; i < 10; i++ {
		t.attackSlotItems[i] = t.attackSlotsItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
		t.healSlotItems[i] = t.healSlotsItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
		t.buffSlotItems[i] = t.buffSlotsItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
		t.mpRestoreSlotItems[i] = t.mpRestoreSlotsItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
		t.fpRestoreSlotItems[i] = t.fpRestoreSlotsItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
		t.pickupSlotItems[i] = t.pickupSlotsItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
	}

	// Initialize slot checkmarks based on config
	t.updateSlotCheckmarks()

	systray.AddSeparator()

	// Pickup and Pet configuration - with 3-level menu (Pickup & Pet -> Type -> Slot 0-9)
	t.pickupPetItem = systray.AddMenuItem("Pickup & Pet", "Configure pickup and pet slots")
	pickupPetSlotItem := t.pickupPetItem.AddSubMenuItem("Pickup Pet Slot", "Select slot for pickup pet summon")
	pickupMotionSlotItem := t.pickupPetItem.AddSubMenuItem("Pickup Motion Slot", "Select slot for motion-based pickup")

	// Create slot submenus for pickup pet and motion (radio buttons, -1 = disabled)
	for i := 0; i < 10; i++ {
		t.pickupPetSlotItems[i] = pickupPetSlotItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
		t.pickupMotionSlotItems[i] = pickupMotionSlotItem.AddSubMenuItemCheckbox(fmt.Sprintf("Slot %d", i), "", false)
	}

	// Initialize pickup pet/motion checkmarks based on config
	t.updatePickupPetCheckmarks()

	systray.AddSeparator()

	// Slot cooldown configuration - with 3-level menu (Slot Cooldowns -> Slot 0-9 -> Time)
	t.slotCooldownItem = systray.AddMenuItem("Slot Cooldowns", "Configure individual slot cooldowns")
	for i := 0; i < 10; i++ {
		t.slotCooldownSlots[i] = t.slotCooldownItem.AddSubMenuItem(fmt.Sprintf("Slot %d", i), fmt.Sprintf("Configure cooldown for slot %d", i))
	}

	// Note: Cooldown time submenus will be created dynamically when a slot is selected
	// to avoid creating 10*21=210 menu items upfront

	// Threshold configuration - with 3-level menu (Thresholds -> Threshold Type -> 0-100%)
	thresholdMenu := systray.AddMenuItem("Thresholds", "Configure thresholds")
	t.hpThresholdItem = thresholdMenu.AddSubMenuItem("HP Threshold", "Set HP heal threshold")
	t.mpThresholdItem = thresholdMenu.AddSubMenuItem("MP Threshold", "Set MP restore threshold")
	t.fpThresholdItem = thresholdMenu.AddSubMenuItem("FP Threshold", "Set FP restore threshold")

	// Create threshold percentage submenus (0-100% in 10% increments)
	for i := 0; i <= 10; i++ {
		percent := i * 10
		t.hpThresholdItems[i] = t.hpThresholdItem.AddSubMenuItemCheckbox(fmt.Sprintf("%d%%", percent), "", false)
		t.mpThresholdItems[i] = t.mpThresholdItem.AddSubMenuItemCheckbox(fmt.Sprintf("%d%%", percent), "", false)
		t.fpThresholdItems[i] = t.fpThresholdItem.AddSubMenuItemCheckbox(fmt.Sprintf("%d%%", percent), "", false)
	}

	// Initialize threshold checkmarks based on config
	t.updateThresholdCheckmarks()

	systray.AddSeparator()

	// Capture frequency configuration
	t.captureFreqItem = systray.AddMenuItem("Capture Frequency", "Configure capture frequency")
	t.captureFreqItems[0] = t.captureFreqItem.AddSubMenuItemCheckbox("Continuous (0ms)", "", false)
	t.captureFreqItems[1] = t.captureFreqItem.AddSubMenuItemCheckbox("1 Second", "", true)  // Default
	t.captureFreqItems[2] = t.captureFreqItem.AddSubMenuItemCheckbox("2 Seconds", "", false)
	t.captureFreqItems[3] = t.captureFreqItem.AddSubMenuItemCheckbox("3 Seconds", "", false)
	t.captureFreqItems[4] = t.captureFreqItem.AddSubMenuItemCheckbox("4 Seconds", "", false)

	systray.AddSeparator()

	// Statistics
	statsItem := systray.AddMenuItem("Statistics", "View bot statistics")
	statsItem.Disable()

	systray.AddSeparator()

	// Quit
	quitItem := systray.AddMenuItem("Quit", "Quit the application")

	// Start event loop
	go t.handleEvents(quitItem)

	LogInfo("System tray initialized")

	// Start browser and main loop in background after tray is ready
	go func() {
		LogInfo("Starting browser from tray...")
		t.bot.StartMainLoop()
		LogInfo("Browser and main loop started")
	}()
}

// handleEvents handles tray menu events
func (t *TrayApp) handleEvents(quitItem *systray.MenuItem) {
	// Start goroutines for handling slot clicks
	for i := 0; i < 10; i++ {
		go t.handleSlotClick("attack", i, t.attackSlotItems[i])
		go t.handleSlotClick("heal", i, t.healSlotItems[i])
		go t.handleSlotClick("buff", i, t.buffSlotItems[i])
		go t.handleSlotClick("mp", i, t.mpRestoreSlotItems[i])
		go t.handleSlotClick("fp", i, t.fpRestoreSlotItems[i])
		go t.handleSlotClick("pickup", i, t.pickupSlotItems[i])
		go t.handlePickupPetSlotClick(i, t.pickupPetSlotItems[i])
		go t.handlePickupMotionSlotClick(i, t.pickupMotionSlotItems[i])
	}

	// Start goroutines for handling slot cooldown clicks
	for i := 0; i < 10; i++ {
		go t.handleSlotCooldownClick(i, t.slotCooldownSlots[i])
	}

	// Start goroutines for handling threshold clicks
	for i := 0; i <= 10; i++ {
		go t.handleThresholdClick("hp", i*10, t.hpThresholdItems[i])
		go t.handleThresholdClick("mp", i*10, t.mpThresholdItems[i])
		go t.handleThresholdClick("fp", i*10, t.fpThresholdItems[i])
	}

	// Start goroutines for handling capture frequency clicks
	intervals := []int{0, 1000, 2000, 3000, 4000}
	for i := 0; i < 5; i++ {
		go t.handleCaptureFreqClick(intervals[i], t.captureFreqItems[i])
	}

	for {
		select {
		case <-t.stopItem.ClickedCh:
			t.onModeClicked("Stop")
		case <-t.farmingItem.ClickedCh:
			t.onModeClicked("Farming")
		case <-t.supportItem.ClickedCh:
			t.onModeClicked("Support")
		case <-t.shoutingItem.ClickedCh:
			t.onModeClicked("Shouting")
		case <-quitItem.ClickedCh:
			LogInfo("Quit requested by user")
			LogInfo("Stopping bot...")
			t.bot.StopBehavior()
			LogInfo("Saving state...")
			t.bot.SaveState()
			LogInfo("Closing browser...")
			t.bot.browser.Close()
			LogInfo("Closing logger...")
			CloseLogger()
			LogInfo("Quitting system tray...")
			systray.Quit()
			LogInfo("Forcing exit...")
			os.Exit(0)
		}
	}
}

// onModeClicked handles mode selection
func (t *TrayApp) onModeClicked(mode string) {
	LogInfo("Mode changed to: %s", mode)

	// Update checkmarks
	t.stopItem.Uncheck()
	t.farmingItem.Uncheck()
	t.supportItem.Uncheck()
	t.shoutingItem.Uncheck()

	switch mode {
	case "Stop":
		t.stopItem.Check()
	case "Farming":
		t.farmingItem.Check()
	case "Support":
		t.supportItem.Check()
	case "Shouting":
		t.shoutingItem.Check()
	}

	// Change bot mode (this will automatically switch behavior in main loop)
	t.bot.ChangeMode(mode)
}

// updateStatus updates the status display
func (t *TrayApp) updateStatus(status string) {
	t.statusItem.SetTitle(fmt.Sprintf("Status: %s", status))
}

// UpdateStatus updates the status from external source
func (t *TrayApp) UpdateStatus(mode string) {
	if mode == "Stop" {
		t.updateStatus(fmt.Sprintf("Mode: %s (Idle)", mode))
	} else {
		kills, kpm, _, uptime := t.bot.stats.GetStats()
		status := fmt.Sprintf("Mode: %s | %d kills | %.1f/min | %s", mode, kills, kpm, uptime)
		t.updateStatus(status)
	}
}

// handleSlotClick handles slot selection clicks
func (t *TrayApp) handleSlotClick(slotType string, slotNum int, menuItem *systray.MenuItem) {
	for {
		<-menuItem.ClickedCh

		// Toggle the slot
		config := t.bot.config
		config.mu.Lock()

		var slots *[]int
		switch slotType {
		case "attack":
			slots = &config.AttackSlots
		case "heal":
			slots = &config.HealSlots
		case "buff":
			slots = &config.BuffSlots
		case "mp":
			slots = &config.MPRestoreSlots
		case "fp":
			slots = &config.FPRestoreSlots
		case "pickup":
			slots = &config.PickupSlots
		}

		// Check if slot is already in the list
		found := false
		for i, s := range *slots {
			if s == slotNum {
				// Remove it
				*slots = append((*slots)[:i], (*slots)[i+1:]...)
				found = true
				break
			}
		}

		if !found {
			// Add it
			*slots = append(*slots, slotNum)
		}

		config.mu.Unlock()

		// Update checkmarks
		t.updateSlotCheckmarks()

		// Save configuration
		t.bot.SaveState()

		LogInfo("Updated %s slots: %v", slotType, *slots)
	}
}

// handleThresholdClick handles threshold selection clicks
func (t *TrayApp) handleThresholdClick(thresholdType string, percent int, menuItem *systray.MenuItem) {
	for {
		<-menuItem.ClickedCh

		config := t.bot.config
		config.mu.Lock()

		switch thresholdType {
		case "hp":
			config.HealThreshold = percent
		case "mp":
			config.MPThreshold = percent
		case "fp":
			config.FPThreshold = percent
		}

		config.mu.Unlock()

		// Update checkmarks
		t.updateThresholdCheckmarks()

		// Save configuration
		t.bot.SaveState()

		LogInfo("Updated %s threshold to: %d%%", thresholdType, percent)
	}
}

// updateSlotCheckmarks updates all slot checkmarks based on current config
func (t *TrayApp) updateSlotCheckmarks() {
	config := t.bot.config
	config.mu.RLock()
	defer config.mu.RUnlock()

	// Helper function to update checkmarks for a slot type
	updateSlots := func(items [10]*systray.MenuItem, configSlots []int) {
		for i := 0; i < 10; i++ {
			checked := false
			for _, slot := range configSlots {
				if slot == i {
					checked = true
					break
				}
			}
			if checked {
				items[i].Check()
			} else {
				items[i].Uncheck()
			}
		}
	}

	updateSlots(t.attackSlotItems, config.AttackSlots)
	updateSlots(t.healSlotItems, config.HealSlots)
	updateSlots(t.buffSlotItems, config.BuffSlots)
	updateSlots(t.mpRestoreSlotItems, config.MPRestoreSlots)
	updateSlots(t.fpRestoreSlotItems, config.FPRestoreSlots)
	updateSlots(t.pickupSlotItems, config.PickupSlots)
}

// updateThresholdCheckmarks updates all threshold checkmarks based on current config
func (t *TrayApp) updateThresholdCheckmarks() {
	config := t.bot.config
	config.mu.RLock()
	defer config.mu.RUnlock()

	// Helper function to update checkmarks for a threshold type
	updateThresholds := func(items [11]*systray.MenuItem, configThreshold int) {
		for i := 0; i <= 10; i++ {
			if i*10 == configThreshold {
				items[i].Check()
			} else {
				items[i].Uncheck()
			}
		}
	}

	updateThresholds(t.hpThresholdItems, config.HealThreshold)
	updateThresholds(t.mpThresholdItems, config.MPThreshold)
	updateThresholds(t.fpThresholdItems, config.FPThreshold)
}

// updatePickupPetCheckmarks updates pickup pet/motion slot checkmarks based on current config
func (t *TrayApp) updatePickupPetCheckmarks() {
	config := t.bot.config
	config.mu.RLock()
	defer config.mu.RUnlock()

	// Update pickup pet slot (radio button behavior)
	for i := 0; i < 10; i++ {
		if config.PickupPetSlot == i {
			t.pickupPetSlotItems[i].Check()
		} else {
			t.pickupPetSlotItems[i].Uncheck()
		}
	}

	// Update pickup motion slot (radio button behavior)
	for i := 0; i < 10; i++ {
		if config.PickupMotionSlot == i {
			t.pickupMotionSlotItems[i].Check()
		} else {
			t.pickupMotionSlotItems[i].Uncheck()
		}
	}
}

// handleCaptureFreqClick handles capture frequency selection clicks
func (t *TrayApp) handleCaptureFreqClick(intervalMs int, menuItem *systray.MenuItem) {
	for {
		<-menuItem.ClickedCh

		config := t.bot.config
		config.mu.Lock()
		config.CaptureInterval = intervalMs
		config.mu.Unlock()

		// Update checkmarks
		t.updateCaptureFreqCheckmarks()

		// Save configuration
		t.bot.SaveState()

		freqStr := "Continuous"
		if intervalMs > 0 {
			freqStr = fmt.Sprintf("%d seconds", intervalMs/1000)
		}
		LogInfo("Updated capture frequency to: %s", freqStr)
	}
}

// updateCaptureFreqCheckmarks updates capture frequency checkmarks based on current config
func (t *TrayApp) updateCaptureFreqCheckmarks() {
	config := t.bot.config
	config.mu.RLock()
	interval := config.CaptureInterval
	config.mu.RUnlock()

	intervals := []int{0, 1000, 2000, 3000, 4000}
	for i, iv := range intervals {
		if iv == interval {
			t.captureFreqItems[i].Check()
		} else {
			t.captureFreqItems[i].Uncheck()
		}
	}
}

// handlePickupPetSlotClick handles pickup pet slot selection clicks (radio button)
func (t *TrayApp) handlePickupPetSlotClick(slotNum int, menuItem *systray.MenuItem) {
	for {
		<-menuItem.ClickedCh

		config := t.bot.config
		config.mu.Lock()
		// Toggle: if already selected, disable it (-1)
		if config.PickupPetSlot == slotNum {
			config.PickupPetSlot = -1
		} else {
			config.PickupPetSlot = slotNum
		}
		config.mu.Unlock()

		// Update checkmarks
		t.updatePickupPetCheckmarks()

		// Save configuration
		t.bot.SaveState()

		if config.PickupPetSlot == -1 {
			LogInfo("Disabled pickup pet")
		} else {
			LogInfo("Updated pickup pet slot to: %d", slotNum)
		}
	}
}

// handlePickupMotionSlotClick handles pickup motion slot selection clicks (radio button)
func (t *TrayApp) handlePickupMotionSlotClick(slotNum int, menuItem *systray.MenuItem) {
	for {
		<-menuItem.ClickedCh

		config := t.bot.config
		config.mu.Lock()
		// Toggle: if already selected, disable it (-1)
		if config.PickupMotionSlot == slotNum {
			config.PickupMotionSlot = -1
		} else {
			config.PickupMotionSlot = slotNum
		}
		config.mu.Unlock()

		// Update checkmarks
		t.updatePickupPetCheckmarks()

		// Save configuration
		t.bot.SaveState()

		if config.PickupMotionSlot == -1 {
			LogInfo("Disabled pickup motion")
		} else {
			LogInfo("Updated pickup motion slot to: %d", slotNum)
		}
	}
}

// handleSlotCooldownClick handles slot cooldown configuration menu clicks
// This shows a submenu with cooldown time options
func (t *TrayApp) handleSlotCooldownClick(slotNum int, menuItem *systray.MenuItem) {
	// Define cooldown options (in milliseconds)
	cooldowns := []struct {
		label string
		ms    int
	}{
		{"50ms", 50},
		{"100ms", 100},
		{"200ms", 200},
		{"300ms", 300},
		{"500ms", 500},
		{"1s", 1000},
		{"2s", 2000},
		{"3s", 3000},
		{"5s", 5000},
		{"10s", 10000},
		{"15s", 15000},
		{"30s", 30000},
		{"1min", 60000},
		{"2min", 120000},
		{"3min", 180000},
		{"5min", 300000},
		{"10min", 600000},
		{"15min", 900000},
		{"30min", 1800000},
		{"1hour", 3600000},
		{"Disabled", 0},
	}

	// Create cooldown time menu items dynamically on first click
	var cooldownItems [21]*systray.MenuItem
	for i, cd := range cooldowns {
		cooldownItems[i] = menuItem.AddSubMenuItemCheckbox(cd.label, "", false)
	}

	// Start handlers for each cooldown option
	for i, cd := range cooldowns {
		go func(idx int, cooldownMs int, item *systray.MenuItem) {
			for {
				<-item.ClickedCh

				config := t.bot.config
				config.mu.Lock()
				if cooldownMs == 0 {
					delete(config.SlotCooldowns, slotNum)
				} else {
					config.SlotCooldowns[slotNum] = cooldownMs
				}
				config.mu.Unlock()

				// Update checkmarks for this slot's cooldown options
				currentCd, hasCd := config.SlotCooldowns[slotNum]
				if !hasCd {
					currentCd = 0
				}
				for j, cdo := range cooldowns {
					if cdo.ms == currentCd {
						cooldownItems[j].Check()
					} else {
						cooldownItems[j].Uncheck()
					}
				}

				// Save configuration
				t.bot.SaveState()

				if cooldownMs == 0 {
					LogInfo("Disabled cooldown for slot %d", slotNum)
				} else {
					LogInfo("Updated slot %d cooldown to: %s", slotNum, cooldowns[idx].label)
				}
			}
		}(i, cd.ms, cooldownItems[i])
	}

	// Initialize checkmarks based on current config
	config := t.bot.config
	config.mu.RLock()
	currentCd, hasCd := config.SlotCooldowns[slotNum]
	config.mu.RUnlock()
	if !hasCd {
		currentCd = 0
	}
	for i, cd := range cooldowns {
		if cd.ms == currentCd {
			cooldownItems[i].Check()
		}
	}
}
