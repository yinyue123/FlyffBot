// Package main - farming.go
//
// This file implements the Farming behavior with a state machine.
// It handles autonomous mob hunting, attacking, and resource management.
package main

import (
	"fmt"
	"math/rand"
	"time"
)

// Stage represents the current state of the farming behavior
type Stage int

const (
	StageInitializing Stage = iota
	StageNoEnemyFound
	StageSearchingForEnemy
	StageNavigating
	StageEnemyFound
	StageAttacking
	StageEscaping
	StageAfterEnemyKill
	StageDead
	StageOffline
)

// String returns the string representation of the stage
func (s Stage) String() string {
	switch s {
	case StageInitializing:
		return "Initializing"
	case StageNoEnemyFound:
		return "NoEnemyFound"
	case StageSearchingForEnemy:
		return "SearchingForEnemy"
	case StageNavigating:
		return "Navigating"
	case StageEnemyFound:
		return "EnemyFound"
	case StageAttacking:
		return "Attacking"
	case StageEscaping:
		return "Escaping"
	case StageAfterEnemyKill:
		return "AfterEnemyKill"
	case StageDead:
		return "Dead"
	case StageOffline:
		return "Offline"
	default:
		return "Unknown"
	}
}

// RetryState tracks retry attempts for various checks
type RetryState struct {
	State           int // Number of times state bar not detected
	Target          int // Number of times target not detected consecutively
	Map             int // Number of times map not detected
	OfflineKeyEvent int // Offline key event counter (1-30: Enter, 31-40: Escape)
}

// SearchingEnemyState tracks searching behavior
type SearchingEnemyState struct {
	UpAndDown   int       // 1-3: looking down, 4-6: looking up
	Reverse     bool      // true: turn left, false: turn right
	Count       int       // Remaining rotation count
	Wander      int       // Wander counter
	ForwardTime time.Time // Time when started moving forward
	Careful     bool      // Careful mode when too many mobs
}

// TargetState tracks current target information
type TargetState struct {
	LastHP       int       // Last recorded HP
	LastHPUpdate time.Time // Last time HP was updated
}

// ObstacleState tracks obstacle avoidance
type ObstacleState struct {
	Count int // Number of obstacle avoidance attempts
}

// Farming implements the farming behavior
type Farming struct {
	Stage          Stage
	Retry          RetryState
	SearchingEnemy SearchingEnemyState
	Target         TargetState
	Obstacle       ObstacleState
	Config         *Config
	Browser        *Browser
	Detector       *ClientDetect // To be implemented
}

// NewFarming creates a new farming behavior
func NewFarming(cfg *Config, browser *Browser, detector *ClientDetect) *Farming {
	return &Farming{
		Stage:    StageInitializing,
		Config:   cfg,
		Browser:  browser,
		Detector: detector,
	}
}

// Restore handles HP/MP/FP restoration and buff management
func (f *Farming) Restore() {
	cfg := f.Config

	// Check if state bar is open
	if !f.Detector.MyStats.Open {
		cfg.Log("State bar not open, switching to Initializing")
		f.Stage = StageInitializing
		return
	}

	// Check if player is dead
	if !f.Detector.MyStats.Alive {
		cfg.Log("Player is dead")
		f.Stage = StageDead
		return
	}

	// Check if disconnected (no kills for a long time)
	if time.Since(cfg.Status.Player.LastKilledTime).Seconds() > float64(cfg.Stat.Settings.WatchDogTime) {
		cfg.Log("WatchDog timeout: No kills for %d seconds", cfg.Stat.Settings.WatchDogTime)
		f.Stage = StageOffline
		return
	}

	// Don't restore if dead, initializing, or offline
	if f.Stage == StageDead || f.Stage == StageInitializing || f.Stage == StageOffline {
		return
	}

	// Update status with detector values
	cfg.Status.Player.HP = f.Detector.MyStats.HP.Value
	cfg.Status.Player.MP = f.Detector.MyStats.MP.Value
	cfg.Status.Player.FP = f.Detector.MyStats.FP.Value

	// HP restoration
	if f.Detector.MyStats.HP.Value < 100 {
		// Try to use food
		page, slot := cfg.GetAvailableSlot(SlotTypeFood, cfg.Status.Player.HP)
		if page != -1 || slot != -1 {
			f.UseSlot(page, slot)
			cfg.AddAction(fmt.Sprintf("use_food(%d:%d)", page, slot))
		} else {
			// Try to use pill
			page, slot = cfg.GetAvailableSlot(SlotTypePill, cfg.Status.Player.HP)
			if page != -1 || slot != -1 {
				f.UseSlot(page, slot)
				cfg.AddAction(fmt.Sprintf("use_pill(%d:%d)", page, slot))
			} else if cfg.Status.Player.HP < cfg.Stat.Attack.EscapeHP {
				// HP too low and restoration on cooldown, escape!
				cfg.Log("HP too low (%d%%), escaping!", cfg.Status.Player.HP)
				f.Stage = StageEscaping
			}
		}
	}

	// MP restoration
	if f.Detector.MyStats.MP.Value < 100 {
		page, slot := cfg.GetAvailableSlot(SlotTypeMPRestore, f.Detector.MyStats.MP.Value)
		if page != -1 || slot != -1 {
			f.UseSlot(page, slot)
			cfg.AddAction(fmt.Sprintf("use_mp(%d:%d)", page, slot))
		}
	}

	// FP restoration
	if f.Detector.MyStats.FP.Value < 100 {
		page, slot := cfg.GetAvailableSlot(SlotTypeFPRestore, f.Detector.MyStats.FP.Value)
		if page != -1 || slot != -1 {
			f.UseSlot(page, slot)
			cfg.AddAction(fmt.Sprintf("use_fp(%d:%d)", page, slot))
		}
	}

	// Buff
	page, slot := cfg.GetAvailableSlot(SlotTypeBuff, 0)
	if page != -1 || slot != -1 {
		f.UseSlot(page, slot)
		cfg.AddAction(fmt.Sprintf("use_buff(%d:%d)", page, slot))
	}
}

// AfterEnemyKill handles post-kill actions (pickup, pet)
func (f *Farming) AfterEnemyKill() {
	cfg := f.Config

	stage := cfg.SwitchWaitCtx("AfterEnemyKill")
	switch stage {
	case 1:
		// Increment kill count
		cfg.AddKilled()
		cfg.Log("Killed mob! Total: %d", cfg.Status.Player.Killed)

		// Setup wait for defeat interval
		cfg.SetupWaitCtx("AfterEnemyKill", cfg.Stat.Attack.DefeatInterval)

	case 2:
		// Try to use pet for pickup
		page, slot := cfg.GetAvailableSlot(SlotTypePet, 0)
		if page != -1 || slot != -1 {
			f.UseSlot(page, slot)
			cfg.AddAction(fmt.Sprintf("summon_pet(%d:%d)", page, slot))
		} else {
			// Use pickup action
			page, slot = cfg.GetAvailableSlot(SlotTypePick, 0)
			if page != -1 || slot != -1 {
				for i := 0; i < 10; i++ {
					f.UseSlot(page, slot)
				}
				cfg.AddAction("pickup_items")
			}
		}

		// Clear wait context and switch to searching state
		cfg.SetupWaitCtx("AfterEnemyKill", -1)
		f.Stage = StageSearchingForEnemy

	case -1:
		// Still waiting
		return
	}
}

// Attacking handles the attack logic
func (f *Farming) Attacking() {
	cfg := f.Config

	// Check if target exists and is open
	hasTarget := f.Detector.Target.Open && f.Detector.Target.Alive

	// Check if target is lost
	if !hasTarget {
		f.Retry.Target++
		if f.Retry.Target >= 5 {
			cfg.Log("Target lost (5 times), searching for new target")
			f.Retry.Target = 0
			f.Stage = StageSearchingForEnemy
		}
		return
	}

	f.Retry.Target = 0

	// Check if target is NPC (not a mob)
	if f.Detector.Target.NPC {
		cfg.Log("Target is NPC, canceling")
		f.Browser.SendKey("Escape", "press")
		cfg.AddAction("cancel_npc_target")
		f.Stage = StageSearchingForEnemy
		return
	}

	// Update status with detector values
	if cfg.Status.Target == nil {
		cfg.Status.Target = &TargetStatus{}
	}
	cfg.Status.Target.HP = f.Detector.Target.HP.Value
	cfg.Status.Target.MP = f.Detector.Target.MP.Value

	// Check if HP decreased
	if f.Detector.Target.HP.Value < f.Target.LastHP {
		f.Target.LastHP = f.Detector.Target.HP.Value
		f.Target.LastHPUpdate = time.Now()
		cfg.Status.Attack.LastUpdateHP = f.Detector.Target.HP.Value
		cfg.Status.Attack.LastUpdateTime = time.Now()
	}

	// Check for obstacle (HP not changing for a long time)
	timeSinceLastUpdate := time.Since(f.Target.LastHPUpdate).Milliseconds()
	if timeSinceLastUpdate > int64(cfg.Stat.Attack.ObstacleThresholdTime) {
		cfg.Log("Obstacle detected: HP not changing for %dms", timeSinceLastUpdate)

		if f.Detector.Target.HP.Value == 100 {
			// Never hit the target
			cfg.Log("Never hit target, canceling")
			f.Browser.SendKey("Escape", "press")
			cfg.AddAction("cancel_obstacle_target")
			f.Stage = StageSearchingForEnemy
			f.Obstacle.Count = 0
			return
		} else if f.Obstacle.Count < cfg.Stat.Attack.ObstacleAvoidCount {
			// Try to avoid obstacle using state machine
			obstacleStage := cfg.SwitchWaitCtx("ObstacleAvoid")
			switch obstacleStage {
			case 1:
				cfg.Log("Avoiding obstacle (attempt %d/%d)", f.Obstacle.Count, cfg.Stat.Attack.ObstacleAvoidCount)
				f.Browser.SendKey("w", "press")
				cfg.SetupWaitCtx("ObstacleAvoid", 100)

			case 2:
				// Random left/right movement
				if rand.Intn(2) == 0 {
					f.Browser.SendKey("ArrowLeft", "hold")
				} else {
					f.Browser.SendKey("ArrowRight", "hold")
				}
				f.Browser.SendKey(" ", "press") // Jump
				cfg.SetupWaitCtx("ObstacleAvoid", 10)

			case 3:
				if rand.Intn(2) == 0 {
					f.Browser.SendKey("ArrowLeft", "release")
				} else {
					f.Browser.SendKey("ArrowRight", "release")
				}
				f.Obstacle.Count++
				f.Target.LastHPUpdate = time.Now() // Reset update time
				cfg.SetupWaitCtx("ObstacleAvoid", cfg.Stat.Attack.ObstacleCoolDown)

			case 4:
				// Obstacle avoidance complete
				cfg.SetupWaitCtx("ObstacleAvoid", -1)

			case -1:
				// Still waiting
				return
			}
			return
		} else {
			// Give up after max attempts
			cfg.Log("Obstacle avoidance failed, giving up")
			f.Browser.SendKey("Escape", "press")
			cfg.AddAction("give_up_obstacle")
			f.Stage = StageSearchingForEnemy
			f.Obstacle.Count = 0
			return
		}
	}

	// Check attack timeout
	attackDuration := time.Since(cfg.Status.Attack.AttackTime).Seconds()
	if attackDuration > float64(cfg.Stat.Attack.MaxTime) {
		cfg.Log("Attack timeout (%ds), giving up", cfg.Stat.Attack.MaxTime)
		f.Browser.SendKey("Escape", "press")
		cfg.AddAction("timeout_give_up")
		f.Stage = StageSearchingForEnemy
		return
	}

	// Check if target is dead
	if f.Detector.Target.HP.Value == 0 || !f.Detector.Target.Alive {
		cfg.Log("Target killed!")
		f.Stage = StageAfterEnemyKill
		f.Obstacle.Count = 0
		return
	}

	// Use attack skill
	page, slot := cfg.GetAvailableSlot(SlotTypeAttack, cfg.Status.Player.HP)
	if page != -1 || slot != -1 {
		f.UseSlot(page, slot)
		cfg.AddAction(fmt.Sprintf("attack(%d:%d)", page, slot))
	}
}

// SearchingForEnemy handles the enemy search logic
func (f *Farming) SearchingForEnemy() {
	cfg := f.Config

	// Get target and mobs info from Detector
	hasTarget := f.Detector.Target.Open && f.Detector.Target.Alive

	// Count total mobs (aggressive + passive + violet)
	mobsCount := len(f.Detector.Mobs.AggressiveMobs) +
		len(f.Detector.Mobs.PassiveMobs) +
		len(f.Detector.Mobs.VioletMobs)

	// Update status mobs list
	cfg.Status.Mobs = make([]string, 0)
	for _, mob := range f.Detector.Mobs.AggressiveMobs {
		mobStr := fmt.Sprintf("(%d,%d,%d,%d,aggressive)", mob.MinX, mob.MinY, mob.MaxX-mob.MinX, mob.MaxY-mob.MinY)
		cfg.Status.Mobs = append(cfg.Status.Mobs, mobStr)
	}
	for _, mob := range f.Detector.Mobs.PassiveMobs {
		mobStr := fmt.Sprintf("(%d,%d,%d,%d,passive)", mob.MinX, mob.MinY, mob.MaxX-mob.MinX, mob.MaxY-mob.MinY)
		cfg.Status.Mobs = append(cfg.Status.Mobs, mobStr)
	}
	for _, mob := range f.Detector.Mobs.VioletMobs {
		mobStr := fmt.Sprintf("(%d,%d,%d,%d,violet)", mob.MinX, mob.MinY, mob.MaxX-mob.MinX, mob.MaxY-mob.MinY)
		cfg.Status.Mobs = append(cfg.Status.Mobs, mobStr)
	}

	// If target exists
	if hasTarget {
		// Check if it's NPC
		if f.Detector.Target.NPC {
			f.Browser.SendKey("Escape", "press")
			cfg.AddAction("cancel_non_mob")
			return
		}

		// Initialize attack parameters
		cfg.Status.Attack.AttackTime = time.Now()
		f.Target.LastHP = 100
		f.Target.LastHPUpdate = time.Now()
		f.Obstacle.Count = 0
		f.Stage = StageAttacking
		cfg.Log("Target acquired, starting attack")
		return
	}

	// If mobs detected
	if mobsCount > 0 {
		// Too many mobs, enter careful mode
		if mobsCount > 7 && !f.SearchingEnemy.Careful {
			cfg.Log("Too many mobs (%d), adjusting view", mobsCount)
			f.Browser.SendKey("ArrowUp", "press")
			cfg.AddAction("careful_mode")
			f.SearchingEnemy.Careful = true
			return
		}

		// Stop moving forward if currently moving
		if !f.SearchingEnemy.ForwardTime.IsZero() {
			f.Browser.SendKey("w", "release")
			cfg.AddAction("stop_forward")
			f.SearchingEnemy.ForwardTime = time.Time{}
		}

		// Click on mob (prioritize aggressive, then passive, then violet)
		var targetMob *MobsPosition
		if len(f.Detector.Mobs.AggressiveMobs) > 0 {
			targetMob = &f.Detector.Mobs.AggressiveMobs[0]
			cfg.Log("Clicking on aggressive mob")
		} else if len(f.Detector.Mobs.PassiveMobs) > 0 {
			targetMob = &f.Detector.Mobs.PassiveMobs[0]
			cfg.Log("Clicking on passive mob")
		} else if len(f.Detector.Mobs.VioletMobs) > 0 {
			targetMob = &f.Detector.Mobs.VioletMobs[0]
			cfg.Log("Clicking on violet mob")
		}

		if targetMob != nil {
			// Click on center of mob
			x := (targetMob.MinX + targetMob.MaxX) / 2
			y := (targetMob.MinY + targetMob.MaxY) / 2
			f.Browser.SimpleClick(x, y)
			cfg.AddAction(fmt.Sprintf("click_mob(%d,%d)", x, y))
		}
		return
	}

	// No mobs, start rotation search
	if f.SearchingEnemy.Count > 0 {
		// Still have rotation attempts left
		if f.SearchingEnemy.Reverse {
			f.Browser.SendKey("ArrowLeft", "press")
			cfg.AddAction("rotate_left")
		} else {
			f.Browser.SendKey("ArrowRight", "press")
			cfg.AddAction("rotate_right")
		}
		f.SearchingEnemy.Count--
	} else {
		// Rotation attempts exhausted, change strategy
		if f.SearchingEnemy.UpAndDown >= 1 && f.SearchingEnemy.UpAndDown <= 3 {
			// Look down
			f.Browser.SendKey("ArrowDown", "press")
			cfg.AddAction("look_down")
			f.SearchingEnemy.Count = rand.Intn(6) + 7 // 7-12
			f.SearchingEnemy.UpAndDown++
		} else if f.SearchingEnemy.UpAndDown >= 4 && f.SearchingEnemy.UpAndDown <= 6 {
			// Look up
			f.Browser.SendKey("ArrowUp", "press")
			cfg.AddAction("look_up")
			f.SearchingEnemy.Count = rand.Intn(6) + 7 // 7-12
			f.SearchingEnemy.UpAndDown++
		} else if cfg.Stat.Navigate {
			// Navigation enabled
			cfg.Log("Entering navigation mode")
			f.Stage = StageNavigating
		} else {
			// Move forward
			cfg.Log("Moving forward to find mobs")
			f.Browser.SendKey("w", "hold")
			cfg.AddAction("move_forward")

			// Record start time, duration 20-40 seconds
			duration := rand.Intn(21) + 20 // 20-40
			f.SearchingEnemy.ForwardTime = time.Now().Add(time.Duration(duration) * time.Second)

			// Random jump
			if rand.Intn(3) == 1 {
				// jumpDuration := rand.Float64()*1.5 + 0.5 // 0.5-2.0
				// time.Sleep(time.Duration(jumpDuration*1000) * time.Millisecond) // TODO: Handle jump timing differently
				f.Browser.SendKey(" ", "press")
				cfg.AddAction("jump")
			}

			// Random left/right strafe
			direct := rand.Intn(6)
			if direct == 1 {
				// moveDuration := rand.Float64()*1.5 + 0.5 // 0.5-2.0
				f.Browser.SendKey("ArrowLeft", "hold")
				// time.Sleep(time.Duration(moveDuration*1000) * time.Millisecond) // TODO: Handle strafe timing differently
				f.Browser.SendKey("ArrowLeft", "release")
				cfg.AddAction("strafe_left")
			} else if direct == 2 {
				// moveDuration := rand.Float64()*1.5 + 0.5 // 0.5-2.0
				f.Browser.SendKey("ArrowRight", "hold")
				// time.Sleep(time.Duration(moveDuration*1000) * time.Millisecond) // TODO: Handle strafe timing differently
				f.Browser.SendKey("ArrowRight", "release")
				cfg.AddAction("strafe_right")
			}

			// Reset
			f.SearchingEnemy.UpAndDown = 1
			f.SearchingEnemy.Reverse = !f.SearchingEnemy.Reverse
		}
	}

	// Check if need to stop moving forward
	if !f.SearchingEnemy.ForwardTime.IsZero() && time.Now().After(f.SearchingEnemy.ForwardTime) {
		f.Browser.SendKey("w", "release")
		cfg.AddAction("stop_forward_timeout")
		f.SearchingEnemy.ForwardTime = time.Time{}
		f.SearchingEnemy.UpAndDown = 1
	}
}

// Escaping handles escape from danger
func (f *Farming) Escaping() {
	cfg := f.Config

	stage := cfg.SwitchWaitCtx("Escaping")
	switch stage {
	case 1:
		cfg.Log("Escaping from danger...")
		// Press and hold forward
		f.Browser.SendKey("w", "hold")
		cfg.AddAction("escape_forward")
		// Hold for 10 seconds
		cfg.SetupWaitCtx("Escaping", 10000)

	case 2:
		// Release forward
		f.Browser.SendKey("w", "release")
		cfg.AddAction("escape_stop")

		// Use board skill
		page, slot := cfg.GetAvailableSlot(SlotTypeBoard, 0)
		if page != -1 || slot != -1 {
			f.UseSlot(page, slot)
			cfg.AddAction(fmt.Sprintf("escape_board(%d:%d)", page, slot))
		}

		// Wait 20 seconds
		cfg.SetupWaitCtx("Escaping", 20000)

	case 3:
		// Press board skill again to dismount
		page, slot := cfg.GetAvailableSlot(SlotTypeBoard, 0)
		if page != -1 || slot != -1 {
			f.UseSlot(page, slot)
			cfg.AddAction(fmt.Sprintf("escape_dismount(%d:%d)", page, slot))
		}

		// Clear wait context and switch to searching
		cfg.SetupWaitCtx("Escaping", -1)
		cfg.Log("Escape completed, searching for enemy")
		f.Stage = StageSearchingForEnemy

	case -1:
		// Still waiting
		return
	}
}

// Dead handles death and respawn
func (f *Farming) Dead() {
	cfg := f.Config

	stage := cfg.SwitchWaitCtx("Dead")
	switch stage {
	case 1:
		cfg.Log("Dead, waiting for respawn...")
		f.Browser.SendKey("Enter", "press")
		cfg.AddAction("death_confirm")
		// Setup wait for death confirm interval
		cfg.SetupWaitCtx("Dead", cfg.Stat.Settings.DeathConfirm)

	case 2:
		// Death handling complete, check if alive
		if f.Detector.MyStats.Alive {
			cfg.Log("Respawned successfully")
			cfg.SetupWaitCtx("Dead", -1) // Clear wait context
			f.Stage = StageInitializing
		} else {
			// Still dead, press Enter again
			cfg.Log("Still dead, retrying...")
			f.Browser.SendKey("Enter", "press")
			cfg.AddAction("death_confirm_retry")
			cfg.SetupWaitCtx("Dead", cfg.Stat.Settings.DeathConfirm)
		}

	case -1:
		// Still waiting
		return
	}
}

// Offline handles disconnection recovery
func (f *Farming) Offline() {
	cfg := f.Config

	stage := cfg.SwitchWaitCtx("Offline")
	switch stage {
	case 1:
		cfg.Log("Handling offline state")
		// Refresh browser
		err := f.Browser.Refresh(cfg)
		if err != nil {
			cfg.Log("Failed to refresh browser: %v", err)
		}
		// Wait for page to load (5 seconds)
		cfg.SetupWaitCtx("Offline", 5000)
		f.Retry.OfflineKeyEvent = 1

	case 2:
		// Press Enter every second until state bar appears (1-30: Enter)
		// Check if state bar is already open
		if f.Detector.MyStats.Open {
			cfg.Log("State bar detected, skipping Enter presses")
			f.Retry.OfflineKeyEvent = 31 // Skip to Escape stage
			cfg.SetupWaitCtx("Offline", 0) // No wait, immediately go to stage 3
			return
		}

		if f.Retry.OfflineKeyEvent <= 30 {
			f.Browser.SendKey("Enter", "press")
			cfg.AddAction(fmt.Sprintf("reconnect_enter(%d)", f.Retry.OfflineKeyEvent))
			f.Retry.OfflineKeyEvent++
			cfg.SetupWaitCtx("Offline", 1000) // Wait 1 second
		} else {
			// Done with Enter presses, move to Escape
			cfg.SetupWaitCtx("Offline", 0) // No wait, immediately go to stage 3
		}

	case 3:
		// Press ESC every second (31-40: Escape)
		if f.Retry.OfflineKeyEvent <= 40 {
			f.Browser.SendKey("Escape", "press")
			cfg.AddAction(fmt.Sprintf("cleanup_esc(%d)", f.Retry.OfflineKeyEvent-30))
			f.Retry.OfflineKeyEvent++
			cfg.SetupWaitCtx("Offline", 1000) // Wait 1 second
		} else {
			// Done with Escape presses
			cfg.SetupWaitCtx("Offline", 0) // No wait, immediately go to stage 4
		}

	case 4:
		// Reconnection attempt completed
		cfg.Log("Reconnection attempt completed")
		cfg.SetupWaitCtx("Offline", -1) // Clear wait context
		f.Retry.OfflineKeyEvent = 0
		f.Stage = StageInitializing

	case -1:
		// Still waiting
		return
	}
}

// Initializing checks if the game is ready
func (f *Farming) Initializing() bool {
	cfg := f.Config

	stage := cfg.SwitchWaitCtx("Initializing")
	switch stage {
	case 1:
		cfg.Log("Initializing...")
		// Check if state bar is open
		if f.Detector.MyStats.Open {
			// State bar is open, check map
			// TODO: Check if map is open (need to implement map detection in Detector)
			mapOpen := true // Temporary, assume map is always open for now

			if mapOpen {
				cfg.Log("Initialization completed")
				cfg.SetupWaitCtx("Initializing", -1) // Clear wait context
				f.Retry.State = 0
				f.Stage = StageSearchingForEnemy
				return true
			}
		} else {
			// State bar not open, increment retry counter
			f.Retry.State++
			cfg.Log("State bar not detected (retry %d)", f.Retry.State)

			if f.Retry.State > 5 {
				// Try to open state bar by pressing 't'
				f.Browser.SendKey("t", "press")
				cfg.AddAction(fmt.Sprintf("open_state_bar(retry_%d)", f.Retry.State))
				f.Retry.State = 0 // Reset counter after pressing 't'
			}

			// Wait 5 seconds before checking again
			cfg.SetupWaitCtx("Initializing", 5000)
		}

	case 2:
		// After waiting 5 seconds, go back to stage 1 to check again
		cfg.SetupWaitCtx("Initializing", -1) // Clear and restart
		return f.Initializing()

	case -1:
		// Still waiting
		return false
	}

	return false
}

// UseSlot uses a skill/item slot
func (f *Farming) UseSlot(page, slot int) error {
	// Switch page if needed
	if page != -1 {
		f.Browser.SendKey(fmt.Sprintf("F%d", page), "press")
		f.Config.UpdateCurrentPage(page)
	}

	// Press slot key
	return f.Browser.SendKey(fmt.Sprintf("%d", slot), "press")
}

// Start is the main farming loop
func (f *Farming) Start() {
	cfg := f.Config
	cfg.Log("Starting farming behavior")

	for cfg.IsEnabled() {
		// Record frame start time
		frameStartTime := time.Now()

		// Capture screenshot
		img, err := f.Browser.Capture()
		if err != nil {
			cfg.Log("Failed to capture: %v", err)
			cfg.WaitInterval(frameStartTime)
			continue
		}

		// Update image in detector (converts to Mat internally)
		err = f.Detector.UpdateImage(img)
		if err != nil {
			cfg.Log("Failed to update image: %v", err)
			cfg.WaitInterval(frameStartTime)
			continue
		}

		// Update player and target state (always needed)
		f.Detector.UpdateMyStats()
		f.Detector.UpdateTargetStats()

		// Update mobs detection only when searching or navigating
		if f.Stage == StageSearchingForEnemy || f.Stage == StageNavigating {
			f.Detector.UpdateMobs()
		}

		// Restore HP/MP/FP
		f.Restore()

		// Update stage to config
		cfg.UpdateStage(f.Stage.String())

		// Execute logic based on stage
		switch f.Stage {
		case StageInitializing:
			f.Initializing()

		case StageSearchingForEnemy:
			f.SearchingForEnemy()

		case StageAttacking:
			f.Attacking()

		case StageAfterEnemyKill:
			f.AfterEnemyKill()

		case StageEscaping:
			f.Escaping()

		case StageDead:
			f.Dead()

		case StageOffline:
			f.Offline()

		case StageNavigating:
			// TODO: Implement navigation logic
			cfg.Log("Navigating...")
			f.Stage = StageSearchingForEnemy
		}

		// Save status
		err = cfg.SaveStatus()
		if err != nil {
			cfg.Log("Failed to save status: %v", err)
		}

		// Wait for frame interval
		cfg.WaitInterval(frameStartTime)
	}

	cfg.Log("Farming behavior stopped")
}
