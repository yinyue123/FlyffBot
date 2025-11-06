// Package main - farming.go
//
// This file implements the Farming behavior with a state machine.
// It handles autonomous mob hunting, attacking, and resource management.
package main

import (
	"fmt"
	"math/rand"
	"time"

	"gocv.io/x/gocv"
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
	State  int // Number of times state bar not detected
	Target int // Number of times target not detected consecutively
	Map    int // Number of times map not detected
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
		time.Sleep(time.Duration(cfg.Stat.Settings.BuffInterval) * time.Millisecond)
	}
}

// AfterEnemyKill handles post-kill actions (pickup, pet)
func (f *Farming) AfterEnemyKill() {
	cfg := f.Config

	// Increment kill count
	cfg.AddKilled()
	cfg.Log("Killed mob! Total: %d", cfg.Status.Player.Killed)

	// Wait for defeat interval
	time.Sleep(time.Duration(cfg.Stat.Attack.DefeatInterval) * time.Millisecond)

	// Try to use pet for pickup
	page, slot := cfg.GetAvailableSlot(SlotTypePet, 0)
	if page != -1 || slot != -1 {
		f.UseSlot(page, slot)
		cfg.AddAction(fmt.Sprintf("summon_pet(%d:%d)", page, slot))
	} else {
		// Use pickup action
		page, slot = cfg.GetAvailableSlot(SlotTypePick, 0)
		if page != -1 || slot != -1 {
			// Press 10 times, 300ms interval
			for i := 0; i < 10; i++ {
				f.UseSlot(page, slot)
				time.Sleep(300 * time.Millisecond)
			}
			cfg.AddAction("pickup_items")
		}
	}

	// Switch to searching state
	f.Stage = StageSearchingForEnemy
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
			// Try to avoid obstacle
			cfg.Log("Avoiding obstacle (attempt %d/%d)", f.Obstacle.Count, cfg.Stat.Attack.ObstacleAvoidCount)
			f.Browser.SendKey("w", "press")
			time.Sleep(100 * time.Millisecond)

			// Random left/right movement
			if rand.Intn(2) == 0 {
				f.Browser.SendKey("ArrowLeft", "hold")
			} else {
				f.Browser.SendKey("ArrowRight", "hold")
			}
			f.Browser.SendKey(" ", "press") // Jump
			time.Sleep(10 * time.Millisecond)

			if rand.Intn(2) == 0 {
				f.Browser.SendKey("ArrowLeft", "release")
			} else {
				f.Browser.SendKey("ArrowRight", "release")
			}

			f.Obstacle.Count++
			f.Target.LastHPUpdate = time.Now() // Reset update time
			time.Sleep(time.Duration(cfg.Stat.Attack.ObstacleCoolDown) * time.Millisecond)
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
				jumpDuration := rand.Float64()*1.5 + 0.5 // 0.5-2.0
				time.Sleep(time.Duration(jumpDuration*1000) * time.Millisecond)
				f.Browser.SendKey(" ", "press")
				cfg.AddAction("jump")
			}

			// Random left/right strafe
			direct := rand.Intn(6)
			if direct == 1 {
				moveDuration := rand.Float64()*1.5 + 0.5 // 0.5-2.0
				f.Browser.SendKey("ArrowLeft", "hold")
				time.Sleep(time.Duration(moveDuration*1000) * time.Millisecond)
				f.Browser.SendKey("ArrowLeft", "release")
				cfg.AddAction("strafe_left")
			} else if direct == 2 {
				moveDuration := rand.Float64()*1.5 + 0.5 // 0.5-2.0
				f.Browser.SendKey("ArrowRight", "hold")
				time.Sleep(time.Duration(moveDuration*1000) * time.Millisecond)
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

// Offline handles disconnection recovery
func (f *Farming) Offline() {
	cfg := f.Config
	cfg.Log("Handling offline state")

	// Refresh browser
	err := f.Browser.Refresh()
	if err != nil {
		cfg.Log("Failed to refresh browser: %v", err)
	}

	// Wait for page to load
	time.Sleep(5 * time.Second)

	// Press Enter every second until state bar appears
	for i := 0; i < 30; i++ {
		// TODO: Check if state bar appears
		// if stateBarAppears {
		//     break
		// }
		f.Browser.SendKey("Enter", "press")
		cfg.AddAction("reconnect_enter")
		time.Sleep(1 * time.Second)
	}

	// Press ESC 10 times
	for i := 0; i < 10; i++ {
		f.Browser.SendKey("Escape", "press")
		cfg.AddAction("cleanup_esc")
		time.Sleep(1 * time.Second)
	}

	// Increment disconnect count
	// TODO: Need to add disconnect counter to struct
	cfg.Log("Reconnection attempt completed")

	// Switch to initializing state
	f.Stage = StageInitializing
}

// Initializing checks if the game is ready
func (f *Farming) Initializing() bool {
	cfg := f.Config
	cfg.Log("Initializing...")

	// Check if state bar is open
	stateBarOpen := f.Detector.MyStats.Open
	if !stateBarOpen {
		// Try to open state bar
		for i := 0; i < 5; i++ {
			f.Browser.SendKey("t", "press")
			cfg.AddAction("open_state_bar")
			time.Sleep(2 * time.Second)

			// Check again after waiting
			if f.Detector.MyStats.Open {
				stateBarOpen = true
				break
			}
		}
	}

	// TODO: Check if map is open (need to implement map detection in Detector)
	mapOpen := true // Temporary, assume map is always open for now

	if stateBarOpen && mapOpen {
		cfg.Log("Initialization completed")
		f.Stage = StageSearchingForEnemy
		return true
	}

	cfg.Log("Initialization failed: stateBar=%v, map=%v", stateBarOpen, mapOpen)
	return false
}

// UseSlot uses a skill/item slot
func (f *Farming) UseSlot(page, slot int) error {
	// Switch page if needed
	if page != -1 {
		f.Browser.SendKey(fmt.Sprintf("F%d", page), "press")
		f.Config.UpdateCurrentPage(page)
		time.Sleep(100 * time.Millisecond)
	}

	// Press slot key
	return f.Browser.SendKey(fmt.Sprintf("%d", slot), "press")
}

// Start is the main farming loop
func (f *Farming) Start() {
	cfg := f.Config
	cfg.Log("Starting farming behavior")

	for cfg.IsEnabled() {
		// Capture screenshot
		img, err := f.Browser.Capture()
		if err != nil {
			cfg.Log("Failed to capture: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Convert image.RGBA to gocv.Mat
		mat, err := gocv.ImageToMatRGB(img)
		if err != nil {
			cfg.Log("Failed to convert image to Mat: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		defer mat.Close()

		// Update player and target state (always needed)
		UpdateState(mat, &f.Detector.MyStats, f.Detector.Debug, "My")
		UpdateState(mat, &f.Detector.Target, f.Detector.Debug, "Target")

		// Update mobs detection only when searching or navigating
		if f.Stage == StageSearchingForEnemy || f.Stage == StageNavigating {
			UpdateMobs(mat, &f.Detector.Mobs, f.Detector.Debug)
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
			// TODO: Implement escape logic
			cfg.Log("Escaping...")
			f.Stage = StageSearchingForEnemy

		case StageDead:
			// TODO: Implement death handling
			cfg.Log("Dead, waiting for respawn...")
			f.Browser.SendKey("Enter", "press")
			time.Sleep(time.Duration(cfg.Stat.Settings.DeathConfirm) * time.Millisecond)

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

		// Sleep based on capture interval
		if cfg.Stat.Type > 0 {
			time.Sleep(time.Duration(1000) * time.Millisecond) // Default 1 second
		}
	}

	cfg.Log("Farming behavior stopped")
}
