// Package main - support.go
//
// This file implements the Support Behavior for healing and buffing party members.
// It handles following a target (usually party leader), healing, buffing, and resurrection.
//
// State Machine States:
//   - NoTarget: No target selected, attempting to select party leader
//   - TargetSelected: Target selected, verifying state
//   - Following: Following the target
//   - TooFar: Target is too far, moving closer
//   - Healing: Healing the target
//   - Buffing: Buffing the target
//   - SelfBuffing: Buffing self
//   - Resurrecting: Resurrecting dead target
//
// State Transitions:
//   NoTarget -> TargetSelected (party leader selected)
//   TargetSelected -> Following (target verified)
//   Following -> TooFar (target out of range)
//   Following -> Healing (target HP low)
//   Following -> Buffing (buff timer expired)
//   Following -> Resurrecting (target dead)
//   TooFar -> Following (in range again)
//   Healing -> Following (target healed)
//   Buffing -> Following (buff complete)
//   SelfBuffing -> NoTarget (self buff complete)
//   Resurrecting -> Following (rez complete)
//
// Key Features:
//   - Party leader auto-selection
//   - Distance-based following
//   - Target HP monitoring and healing
//   - Periodic buffing (separate cooldowns for self and target)
//   - Resurrection support
//   - Self-care (own HP/MP/FP management)
//   - Random camera movement to avoid AFK detection
package main

import (
	"time"
)

// SupportState represents the current state of the support behavior
type SupportState int

const (
	SupportStateNoTarget SupportState = iota
	SupportStateTargetSelected
	SupportStateFollowing
	SupportStateTooFar
	SupportStateHealing
	SupportStateBuffing
	SupportStateSelfBuffing
	SupportStateResurrecting
)

// String returns the string representation of the state
func (s SupportState) String() string {
	switch s {
	case SupportStateNoTarget:
		return "NoTarget"
	case SupportStateTargetSelected:
		return "TargetSelected"
	case SupportStateFollowing:
		return "Following"
	case SupportStateTooFar:
		return "TooFar"
	case SupportStateHealing:
		return "Healing"
	case SupportStateBuffing:
		return "Buffing"
	case SupportStateSelfBuffing:
		return "SelfBuffing"
	case SupportStateResurrecting:
		return "Resurrecting"
	default:
		return "Unknown"
	}
}

// SupportBehavior implements support behavior with state machine
type SupportBehavior struct {
	// State machine
	state SupportState

	// Target management
	hasTarget          bool
	lastTargetDistance int

	// Timing
	lastJumpTime       time.Time
	lastFarFromTarget  *time.Time
	lastBuffTime       time.Time
	lastSelfBuffTime   time.Time

	// Wait management
	waitDuration *time.Duration
	waitStart    time.Time

	// Buffing state
	selfBuffing    bool
	targetBuffing  bool
	buffCounter    int

	// Resurrection
	isWaitingForRevive bool

	// Obstacle avoidance
	avoidObstacleDirection string
}

// NewSupportBehavior creates a new support behavior
func NewSupportBehavior() *SupportBehavior {
	return &SupportBehavior{
		state:                  SupportStateNoTarget,
		lastJumpTime:           time.Now(),
		lastBuffTime:           time.Now(),
		lastSelfBuffTime:       time.Now(),
		avoidObstacleDirection: "D",
	}
}

// GetState returns the current state name
func (sb *SupportBehavior) GetState() string {
	return sb.state.String()
}

// Run executes one iteration of support behavior
func (sb *SupportBehavior) Run(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, stats *Statistics) error {
	// Update player stats
	analyzer.UpdateStats()
	clientStats := analyzer.GetStats()

	// Check if player is alive
	if clientStats.IsAlive != AliveStateAlive {
		LogWarn("Player is dead, stopping support")
		return nil
	}

	// Use party skills
	sb.usePartySkills(movement, config)

	// Check self restorations
	sb.checkSelfRestorations(movement, config, clientStats)

	// Update target status
	sb.hasTarget = clientStats.TargetOnScreen

	// Check if we should wait
	if sb.waitCooldown() {
		return nil
	}

	// Random camera movement
	sb.randomCameraMovement(movement)

	// State machine execution
	sb.state = sb.runStateMachine(analyzer, movement, config, clientStats)

	return nil
}

// runStateMachine executes the state machine and returns next state
func (sb *SupportBehavior) runStateMachine(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, clientStats *ClientStats) SupportState {
	switch sb.state {
	case SupportStateNoTarget:
		return sb.onNoTarget(movement, config)
	case SupportStateTargetSelected:
		return sb.onTargetSelected(clientStats)
	case SupportStateFollowing:
		return sb.onFollowing(analyzer, movement, config, clientStats)
	case SupportStateTooFar:
		return sb.onTooFar(movement, analyzer, config)
	case SupportStateHealing:
		return sb.onHealing(movement, config, clientStats)
	case SupportStateBuffing:
		return sb.onBuffing(movement, config)
	case SupportStateSelfBuffing:
		return sb.onSelfBuffing(movement, config)
	case SupportStateResurrecting:
		return sb.onResurrecting(movement, config, clientStats)
	default:
		return SupportStateNoTarget
	}
}

// onNoTarget handles state when no target is selected
func (sb *SupportBehavior) onNoTarget(movement *MovementCoordinator, config *Config) SupportState {
	if config.InParty {
		sb.selectPartyLeader(movement)
		sb.wait(500 * time.Millisecond)
		return SupportStateTargetSelected
	}
	return SupportStateNoTarget
}

// onTargetSelected handles state when target is selected
func (sb *SupportBehavior) onTargetSelected(clientStats *ClientStats) SupportState {
	if !sb.hasTarget {
		return SupportStateNoTarget
	}
	return SupportStateFollowing
}

// onFollowing handles the following state
func (sb *SupportBehavior) onFollowing(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, clientStats *ClientStats) SupportState {
	if !sb.hasTarget {
		return SupportStateNoTarget
	}

	// Check if target is dead
	if !clientStats.TargetIsAlive {
		return SupportStateResurrecting
	}

	// Check target distance
	if clientStats.TargetOnScreen {
		isInRange := sb.isTargetInRange(analyzer, config)
		if !isInRange {
			return SupportStateTooFar
		}
	}

	// Check if target needs healing
	targetHP := analyzer.DetectTargetHP()
	if targetHP > 0 && targetHP < config.HealThreshold {
		return SupportStateHealing
	}

	// Check if should buff target
	if time.Since(sb.lastBuffTime) > 30*time.Second && len(config.BuffSlots) > 0 {
		return SupportStateBuffing
	}

	// Check if should self-buff (in party mode)
	if config.InParty && time.Since(sb.lastSelfBuffTime) > 60*time.Second && len(config.BuffSlots) > 0 {
		return SupportStateSelfBuffing
	}

	// Continue following
	sb.followTarget(movement)

	return SupportStateFollowing
}

// onTooFar handles state when target is too far
func (sb *SupportBehavior) onTooFar(movement *MovementCoordinator, analyzer *ImageAnalyzer, config *Config) SupportState {
	// Move towards target
	sb.followTarget(movement)

	// Check if back in range
	if sb.isTargetInRange(analyzer, config) {
		return SupportStateFollowing
	}

	return SupportStateTooFar
}

// onHealing handles the healing state
func (sb *SupportBehavior) onHealing(movement *MovementCoordinator, config *Config, clientStats *ClientStats) SupportState {
	targetHP := clientStats.TargetHP.GetValue()

	if len(config.HealSlots) > 0 {
		LogDebug("Healing target (HP: %d%%)", targetHP)
		movement.UseSkill(config.HealSlots)
		sb.wait(2000 * time.Millisecond)
	} else if len(config.AOEHealSlots) > 0 {
		LogDebug("AOE healing target (HP: %d%%)", targetHP)
		movement.UseSkill(config.AOEHealSlots)
		time.Sleep(100 * time.Millisecond)
		movement.UseSkill(config.AOEHealSlots)
		time.Sleep(100 * time.Millisecond)
		movement.UseSkill(config.AOEHealSlots)
		sb.wait(100 * time.Millisecond)
	}

	return SupportStateFollowing
}

// onBuffing handles the buffing state
func (sb *SupportBehavior) onBuffing(movement *MovementCoordinator, config *Config) SupportState {
	if !sb.targetBuffing {
		sb.targetBuffing = true
		sb.buffCounter = 0
		LogDebug("Starting target buffing")
	}

	if len(config.BuffSlots) > 0 {
		movement.UseSkill(config.BuffSlots)
		sb.buffCounter++
		sb.wait(2500 * time.Millisecond)

		// Check if more buffs to apply
		if sb.buffCounter >= len(config.BuffSlots) {
			sb.targetBuffing = false
			sb.lastBuffTime = time.Now()
			LogDebug("Target buffing complete (%d buffs)", sb.buffCounter)
			return SupportStateFollowing
		}
	} else {
		sb.targetBuffing = false
		sb.lastBuffTime = time.Now()
		return SupportStateFollowing
	}

	return SupportStateBuffing
}

// onSelfBuffing handles the self-buffing state
func (sb *SupportBehavior) onSelfBuffing(movement *MovementCoordinator, config *Config) SupportState {
	if !sb.selfBuffing {
		sb.selfBuffing = true
		sb.buffCounter = 0
		LogDebug("Starting self buffing")
	}

	// Lose target to self-cast
	if sb.hasTarget {
		sb.loseTarget(movement)
		sb.wait(250 * time.Millisecond)
	}

	if len(config.BuffSlots) > 0 {
		movement.UseSkill(config.BuffSlots)
		sb.buffCounter++
		sb.wait(2500 * time.Millisecond)

		// Check if more buffs to apply
		if sb.buffCounter >= len(config.BuffSlots) {
			sb.selfBuffing = false
			sb.lastSelfBuffTime = time.Now()
			LogDebug("Self buffing complete (%d buffs)", sb.buffCounter)

			// Reselect party leader
			return SupportStateNoTarget
		}
	} else {
		sb.selfBuffing = false
		sb.lastSelfBuffTime = time.Now()
		return SupportStateNoTarget
	}

	return SupportStateSelfBuffing
}

// onResurrecting handles the resurrection state
func (sb *SupportBehavior) onResurrecting(movement *MovementCoordinator, config *Config, clientStats *ClientStats) SupportState {
	if sb.hasTarget && !clientStats.TargetIsAlive {
		LogDebug("Target is dead, need resurrection")

		if sb.isWaitingForRevive {
			// Check if target is alive now
			if clientStats.TargetHP.GetValue() > 0 {
				LogInfo("Target has been revived")
				sb.isWaitingForRevive = false
				return SupportStateFollowing
			}
			// Still waiting for revive
			return SupportStateResurrecting
		} else {
			// Cast resurrection skill
			if len(config.RezSlots) > 0 {
				LogInfo("Casting resurrection on target")
				movement.UseSkill(config.RezSlots)
				// Wait for cast time (resurrection typically takes 3-5 seconds)
				sb.wait(3000 * time.Millisecond)
				sb.isWaitingForRevive = true
			} else {
				LogWarn("No resurrection skill configured (RezSlots empty)")
				sb.isWaitingForRevive = true
			}
			return SupportStateResurrecting
		}
	}

	// Target is alive or no target, go back to following
	sb.isWaitingForRevive = false
	return SupportStateFollowing
}

// isTargetInRange checks if target is within acceptable range
func (sb *SupportBehavior) isTargetInRange(analyzer *ImageAnalyzer, config *Config) bool {
	distance := analyzer.DetectTargetDistance()

	if distance == 9999 {
		sb.moveCirclePattern(analyzer, config)
		return false
	}

	maxDistance := config.FollowDistance
	if distance > maxDistance {
		// Check if moving away
		if distance > maxDistance*2 {
			sb.moveCirclePattern(analyzer, config)
		} else {
			// Check if consistently far
			if sb.lastFarFromTarget != nil {
				if time.Since(*sb.lastFarFromTarget).Milliseconds() > 3000 && sb.lastTargetDistance < distance {
					now := time.Now()
					sb.lastFarFromTarget = &now
					sb.moveCirclePattern(analyzer, config)
				}
			} else {
				now := time.Now()
				sb.lastFarFromTarget = &now
			}
		}

		sb.lastTargetDistance = distance
		return false
	}

	// In range
	sb.lastFarFromTarget = nil
	return true
}

// moveCirclePattern performs circular movement
func (sb *SupportBehavior) moveCirclePattern(analyzer *ImageAnalyzer, config *Config) {
	movement := &MovementCoordinator{
		screenInfo: analyzer.screenInfo,
	}

	movement.HoldKeys([]string{"W", "Space", sb.avoidObstacleDirection})
	movement.Wait(100 * time.Millisecond)
	movement.ReleaseKey(sb.avoidObstacleDirection)
	movement.Wait(500 * time.Millisecond)
	movement.ReleaseKeys([]string{"Space", "W"})
	movement.PressKey("Z")

	// Alternate direction
	if sb.avoidObstacleDirection == "D" {
		sb.avoidObstacleDirection = "A"
	} else {
		sb.avoidObstacleDirection = "D"
	}
}

// selectPartyLeader selects the party leader
func (sb *SupportBehavior) selectPartyLeader(movement *MovementCoordinator) {
	LogDebug("Selecting party leader")

	// Open party menu
	movement.PressKey("P")
	time.Sleep(150 * time.Millisecond)

	// Click on party leader position
	point := Point{X: 213, Y: 440}
	movement.ClickTarget(point)

	movement.PressKey("Z")
	movement.Wait(10 * time.Millisecond)
	movement.PressKey("P")

	time.Sleep(500 * time.Millisecond)
}

// followTarget follows the current target
func (sb *SupportBehavior) followTarget(movement *MovementCoordinator) {
	if sb.hasTarget {
		movement.PressKey("Z")
	}
}

// loseTarget cancels current target
func (sb *SupportBehavior) loseTarget(movement *MovementCoordinator) {
	if sb.hasTarget {
		movement.PressKey("Escape")
		movement.WaitRandom(200, 250)
	}
}

// randomCameraMovement adds random camera movement
func (sb *SupportBehavior) randomCameraMovement(movement *MovementCoordinator) {
	if time.Since(sb.lastJumpTime) > 10*time.Second {
		movement.RotateRight(50 * time.Millisecond)
		movement.Wait(50 * time.Millisecond)
		sb.lastJumpTime = time.Now()
	}
}

// usePartySkills uses party buff skills
func (sb *SupportBehavior) usePartySkills(movement *MovementCoordinator, config *Config) {
	if len(config.PartySkillSlots) == 0 {
		return
	}

	// Use all party skills (they typically have longer cooldowns managed externally)
	for _, slot := range config.PartySkillSlots {
		movement.UseSlot(slot)
		// Small delay between skills
		sb.wait(100 * time.Millisecond)
	}
}

// checkSelfRestorations checks and restores player's own HP/MP/FP
func (sb *SupportBehavior) checkSelfRestorations(movement *MovementCoordinator, config *Config, stats *ClientStats) {
	hpValue := stats.HP.GetValue()

	// Check HP
	if hpValue < config.HealThreshold && len(config.HealSlots) > 0 {
		LogDebug("Self HP low (%d%%), healing", hpValue)

		// If in party, need to lose target first
		if config.InParty && sb.hasTarget {
			sb.loseTarget(movement)
			movement.UseSkill(config.HealSlots)
			sb.wait(2000 * time.Millisecond)
			// Target will be reselected in next iteration
		} else {
			movement.UseSkill(config.HealSlots)
			time.Sleep(500 * time.Millisecond)
		}
	} else if hpValue < config.HealThreshold && len(config.AOEHealSlots) > 0 {
		LogDebug("Self HP low (%d%%), using AOE heal", hpValue)
		movement.UseSkill(config.AOEHealSlots)
		time.Sleep(100 * time.Millisecond)
		movement.UseSkill(config.AOEHealSlots)
		time.Sleep(100 * time.Millisecond)
		movement.UseSkill(config.AOEHealSlots)
	}

	// Check MP
	mpValue := stats.MP.GetValue()
	if mpValue < config.MPThreshold && len(config.MPRestoreSlots) > 0 {
		LogDebug("Self MP low (%d%%), restoring", mpValue)
		movement.UseSkill(config.MPRestoreSlots)
		time.Sleep(300 * time.Millisecond)
	}

	// Check FP
	fpValue := stats.FP.GetValue()
	if fpValue < config.FPThreshold && len(config.FPRestoreSlots) > 0 {
		LogDebug("Self FP low (%d%%), restoring", fpValue)
		movement.UseSkill(config.FPRestoreSlots)
		time.Sleep(300 * time.Millisecond)
	}
}

// wait sets a wait duration
func (sb *SupportBehavior) wait(duration time.Duration) {
	if sb.waitDuration != nil {
		newDuration := *sb.waitDuration + duration
		sb.waitDuration = &newDuration
	} else {
		sb.waitStart = time.Now()
		sb.waitDuration = &duration
	}
}

// waitCooldown checks if we should wait
func (sb *SupportBehavior) waitCooldown() bool {
	if sb.waitDuration != nil {
		if time.Since(sb.waitStart) < *sb.waitDuration {
			return true
		}
		sb.waitDuration = nil
	}
	return false
}

// Stop stops the support behavior
func (sb *SupportBehavior) Stop() {
	sb.hasTarget = false
	sb.selfBuffing = false
	sb.targetBuffing = false
	sb.waitDuration = nil
	sb.state = SupportStateNoTarget
}
