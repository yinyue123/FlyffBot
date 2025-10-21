// Package main - farming.go
//
// This file implements the Farming Behavior with a state machine.
// It handles autonomous mob hunting, attacking, and item collection.
//
// State Machine States:
//   - NoEnemyFound: No mobs detected, initiating search
//   - SearchingForEnemy: Actively searching for targets
//   - EnemyFound: Mob detected, initiating attack
//   - VerifyTarget: Verifying target selection was successful
//   - Attacking: Currently in combat with target
//   - AfterEnemyKill: Processing kill, picking up items
//
// State Transitions:
//   NoEnemyFound -> SearchingForEnemy (after rotation/movement)
//   SearchingForEnemy -> NoEnemyFound (no mobs found)
//   SearchingForEnemy -> EnemyFound (mob detected)
//   EnemyFound -> VerifyTarget (clicked on mob)
//   VerifyTarget -> Attacking (target confirmed)
//   VerifyTarget -> SearchingForEnemy (target not confirmed)
//   Attacking -> AfterEnemyKill (mob defeated)
//   Attacking -> SearchingForEnemy (target lost or invalid)
//   AfterEnemyKill -> SearchingForEnemy (ready for next target)
//
// Key Features:
//   - Obstacle avoidance with retry limits
//   - Area avoidance system (blacklist failed locations)
//   - AOE farming support for multiple mobs
//   - Aggressive mob prioritization
//   - Kill statistics tracking
//   - Automatic pickup after kills
package main

import (
	"fmt"
	"time"
)

// FarmingState represents the current state of the farming behavior
type FarmingState int

const (
	FarmingStateNoEnemyFound FarmingState = iota
	FarmingStateSearchingForEnemy
	FarmingStateEnemyFound
	FarmingStateVerifyTarget
	FarmingStateAttacking
	FarmingStateAfterEnemyKill
)

// String returns the string representation of the state
func (s FarmingState) String() string {
	switch s {
	case FarmingStateNoEnemyFound:
		return "NoEnemyFound"
	case FarmingStateSearchingForEnemy:
		return "SearchingForEnemy"
	case FarmingStateEnemyFound:
		return "EnemyFound"
	case FarmingStateVerifyTarget:
		return "VerifyTarget"
	case FarmingStateAttacking:
		return "Attacking"
	case FarmingStateAfterEnemyKill:
		return "AfterEnemyKill"
	default:
		return "Unknown"
	}
}

// FarmingBehavior implements autonomous mob hunting with state machine
type FarmingBehavior struct {
	// State machine
	state FarmingState

	// Timing
	lastKillTime          time.Time
	lastSearchTime        time.Time
	lastInitialAttackTime time.Time
	lastNoEnemyTime       *time.Time

	// Attack management
	currentTarget     *Target
	isAttacking       bool
	alreadyAttackCount int
	lastClickPos      *Point

	// Obstacle and avoidance
	rotationAttempts      int
	obstacleAvoidanceCount int
	avoidanceList         *AvoidanceList
	avoidedBounds         []AvoidedArea

	// Statistics
	killCount             int
	stealedTargetCount    int
	lastKilledType        MobType
	concurrentMobsAttack  int

	// Wait management
	waitDuration *time.Duration
	waitStart    time.Time

	// Pickup pet management
	lastSummonPetTime time.Time
	slotUsageTimes    map[int]time.Time // slot number -> last usage time
}

// NewFarmingBehavior creates a new farming behavior
func NewFarmingBehavior() *FarmingBehavior {
	return &FarmingBehavior{
		state:                FarmingStateSearchingForEnemy,
		avoidanceList:        NewAvoidanceList(),
		lastKillTime:         time.Now(),
		avoidedBounds:        make([]AvoidedArea, 0),
		lastKilledType:       MobPassive,
		slotUsageTimes:       make(map[int]time.Time),
		lastSummonPetTime:    time.Now(),
	}
}

// GetState returns the current state name
func (fb *FarmingBehavior) GetState() string {
	return fb.state.String()
}

// Run executes one iteration of farming behavior
func (fb *FarmingBehavior) Run(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, stats *Statistics) error {
	// Update player stats
	analyzer.UpdateStats()
	clientStats := analyzer.GetStats()

	// Check if player is alive
	if clientStats.IsAlive != AliveStateAlive {
		LogWarn("Player is dead, stopping farming")
		return nil
	}

	// Update timestamps
	fb.updateTimestamps()

	// Check restorations (HP/MP/FP)
	fb.checkRestorations(movement, config, clientStats)

	// Check if we should wait
	if fb.waitCooldown() {
		// Use buffs during wait if available
		if len(config.BuffSlots) > 0 {
			movement.UseSkill(config.BuffSlots)
			fb.wait(1500 * time.Millisecond)
		}

		// Only return early if we're not in critical states
		shouldReturn := fb.state == FarmingStateAfterEnemyKill
		if shouldReturn {
			return nil
		}
	}

	// State machine execution
	fb.state = fb.runStateMachine(analyzer, movement, config, stats, clientStats)

	return nil
}

// runStateMachine executes the state machine and returns next state
func (fb *FarmingBehavior) runStateMachine(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, stats *Statistics, clientStats *ClientStats) FarmingState {
	switch fb.state {
	case FarmingStateNoEnemyFound:
		return fb.onNoEnemyFound(movement, config)
	case FarmingStateSearchingForEnemy:
		return fb.onSearchingForEnemy(analyzer, config)
	case FarmingStateEnemyFound:
		return fb.onEnemyFound(movement)
	case FarmingStateVerifyTarget:
		return fb.onVerifyTarget(clientStats)
	case FarmingStateAttacking:
		return fb.onAttacking(analyzer, movement, config, clientStats)
	case FarmingStateAfterEnemyKill:
		return fb.afterEnemyKill(movement, config, stats)
	default:
		return FarmingStateSearchingForEnemy
	}
}

// onNoEnemyFound handles the state when no enemy is found
func (fb *FarmingBehavior) onNoEnemyFound(movement *MovementCoordinator, config *Config) FarmingState {
	// Check for timeout if configured
	if fb.lastNoEnemyTime == nil {
		now := time.Now()
		fb.lastNoEnemyTime = &now
	} else if config.MobsTimeout > 0 {
		if time.Since(*fb.lastNoEnemyTime).Milliseconds() > int64(config.MobsTimeout) {
			LogError("No enemies found for too long, exiting")
			// Exit application
			return FarmingStateNoEnemyFound
		}
	}

	// Try rotating first
	if fb.rotationAttempts < 30 {
		movement.RotateRight(50 * time.Millisecond)
		movement.Wait(50 * time.Millisecond)
		fb.rotationAttempts++
		return FarmingStateSearchingForEnemy
	}

	// Use circle movement if configured
	if config.CircleMoveDuration > 0 {
		fb.moveCirclePattern(movement, time.Duration(config.CircleMoveDuration)*time.Millisecond)
	} else {
		fb.rotationAttempts = 0
		return fb.state
	}

	return FarmingStateSearchingForEnemy
}

// moveCirclePattern performs circular movement pattern
func (fb *FarmingBehavior) moveCirclePattern(movement *MovementCoordinator, rotationDuration time.Duration) {
	movement.HoldKeys([]string{"W", "Space", "D"})
	movement.Wait(rotationDuration)
	movement.ReleaseKey("D")
	movement.Wait(20 * time.Millisecond)
	movement.ReleaseKeys([]string{"Space", "W"})
	movement.HoldKeyFor("S", 50*time.Millisecond)
}

// onSearchingForEnemy handles searching for enemies
func (fb *FarmingBehavior) onSearchingForEnemy(analyzer *ImageAnalyzer, config *Config) FarmingState {
	// Check if should stop fighting
	if config.StopFighting {
		return FarmingStateVerifyTarget
	}

	// Identify mobs
	mobs := analyzer.IdentifyMobs(config)
	if len(mobs) == 0 {
		return FarmingStateNoEnemyFound
	}

	if config.CircleMoveDuration == 0 {
		maxDistance = 325
	} else {
		maxDistance = 1000
	}

	// Prioritize mobs
	mobList := fb.prioritizeMobs(analyzer, config, mobs)
	if len(mobList) == 0 {
		return FarmingStateNoEnemyFound
	}

	fb.rotationAttempts = 0

	// Find closest mob avoiding blacklisted areas
	var closest *Target
	if len(fb.avoidedBounds) == 0 {
		closest = analyzer.FindClosestMob(mobList)
	} else {
		// Filter mobs that are in avoided areas
		for i := range mobList {
			mob := &mobList[i]
			attackCoords := mob.AttackCoords()
			shouldAvoid := false

			for _, avoided := range fb.avoidedBounds {
				if avoided.Bounds.Contains(attackCoords) {
					shouldAvoid = true
					break
				}
			}

			if !shouldAvoid {
				if closest == nil {
					closest = mob
				} else {
					// Check distance
					screenCenter := analyzer.screenInfo.Center()
					currentDist := attackCoords.Distance(screenCenter)
					closestDist := closest.AttackCoords().Distance(screenCenter)
					if currentDist < closestDist {
						closest = mob
					}
				}
			}
		}
	}

	if closest == nil {
		return FarmingStateSearchingForEnemy
	}

	fb.currentTarget = closest
	return FarmingStateEnemyFound
}

// prioritizeMobs prioritizes aggressive mobs if configured
func (fb *FarmingBehavior) prioritizeMobs(analyzer *ImageAnalyzer, config *Config, mobs []Target) []Target {
	if !config.PrioritizeAggro {
		// Return all non-violet mobs
		result := make([]Target, 0)
		for _, mob := range mobs {
			if mob.Type != MobViolet {
				result = append(result, mob)
			}
		}
		return result
	}

	// Get aggressive mobs
	aggressive := make([]Target, 0)
	passive := make([]Target, 0)

	for _, mob := range mobs {
		if mob.Type == MobAggressive {
			aggressive = append(aggressive, mob)
		} else if mob.Type == MobPassive {
			passive = append(passive, mob)
		}
	}

	// If no aggressive or just killed aggressive, use passive if HP is good
	clientHP := analyzer.GetStats().HP.Value
	if (len(aggressive) == 0 ||
		(fb.lastKilledType == MobAggressive && len(aggressive) == 1 &&
		 time.Since(fb.lastKillTime).Milliseconds() < 5000)) &&
		clientHP >= config.MinHPAttack {
		return passive
	}

	return aggressive
}

// onEnemyFound handles when an enemy is found
func (fb *FarmingBehavior) onEnemyFound(movement *MovementCoordinator) FarmingState {
	if fb.currentTarget == nil {
		return FarmingStateSearchingForEnemy
	}

	// Get attack coordinates
	attackCoords := fb.currentTarget.AttackCoords()
	fb.lastClickPos = &attackCoords

	// Click on mob
	movement.ClickTarget(attackCoords)

	// Wait before verifying
	time.Sleep(150 * time.Millisecond)

	fb.isAttacking = false
	return FarmingStateVerifyTarget
}

// onVerifyTarget verifies the target was selected
func (fb *FarmingBehavior) onVerifyTarget(clientStats *ClientStats) FarmingState {
	// Check if target marker exists and is a mover (not NPC)
	if clientStats.TargetOnScreen && clientStats.TargetIsAlive {
		LogDebug("Target verified and is alive")
		return FarmingStateAttacking
	}

	// Failed to select target
	fb.avoidLastClick()
	return FarmingStateSearchingForEnemy
}

// onAttacking handles the attacking state
func (fb *FarmingBehavior) onAttacking(analyzer *ImageAnalyzer, movement *MovementCoordinator, config *Config, clientStats *ClientStats) FarmingState {
	if !fb.isAttacking {
		fb.rotationAttempts = 0
		fb.obstacleAvoidanceCount = 0
		fb.lastInitialAttackTime = time.Now()
		fb.isAttacking = true
		fb.alreadyAttackCount = 0
	}

	// Check if target still exists and is alive
	if !clientStats.TargetOnScreen || !clientStats.TargetIsAlive {
		// Check if we're still alive - if so, mob is dead
		if clientStats.IsAlive == AliveStateAlive {
			LogInfo("Target defeated")

			// Record mob type
			if fb.currentTarget != nil {
				fb.lastKilledType = fb.currentTarget.Type
			}

			fb.concurrentMobsAttack = 0
			fb.isAttacking = false
			return FarmingStateAfterEnemyKill
		}

		fb.isAttacking = false
		return FarmingStateSearchingForEnemy
	}

	// Get target HP
	targetHP := analyzer.DetectTargetHP()

	// Check for obstacle avoidance
	lastUpdate := time.Since(clientStats.TargetHP.LastUpdateTime)
	obstacleTimeout := time.Duration(config.ObstacleAvoidanceCooldown) * time.Millisecond

	if !clientStats.TargetOnScreen || lastUpdate > obstacleTimeout {
		if targetHP == 100 {
			if fb.avoidObstacle(movement, analyzer, 2) {
				return FarmingStateSearchingForEnemy
			}
		} else if fb.avoidObstacle(movement, analyzer, config.ObstacleAvoidanceMaxTry) {
			return FarmingStateSearchingForEnemy
		}
	}

	// Use attack skills
	if len(config.AttackSlots) > 0 {
		movement.UseSkill(config.AttackSlots)
	}

	// Check for AOE farming
	if config.MaxAOEFarming > 1 {
		if fb.concurrentMobsAttack < config.MaxAOEFarming {
			if targetHP < 90 {
				fb.concurrentMobsAttack++
				return fb.abortAttack(movement, analyzer)
			}
			return fb.state
		}
	}

	// Use AOE skills if target is close enough
	targetDistance := clientStats.TargetDistance
	if targetDistance < 75 && len(config.AOEAttackSlots) > 0 {
		movement.UseSkill(config.AOEAttackSlots)
	}

	return fb.state
}

// avoidObstacle attempts to avoid obstacle
func (fb *FarmingBehavior) avoidObstacle(movement *MovementCoordinator, analyzer *ImageAnalyzer, maxTries int) bool {
	if fb.obstacleAvoidanceCount < maxTries {
		if fb.obstacleAvoidanceCount == 0 {
			// First try: press Z and move forward
			movement.PressKey("Z")
			movement.HoldKeys([]string{"W", "Space"})
			movement.Wait(800 * time.Millisecond)
			movement.ReleaseKeys([]string{"Space", "W"})
		} else {
			// Random direction movement
			directions := []string{"A", "D"}
			rotationKey := directions[fb.obstacleAvoidanceCount%2]

			movement.HoldKeys([]string{"W", "Space"})
			movement.HoldKeyFor(rotationKey, 200*time.Millisecond)
			movement.Wait(800 * time.Millisecond)
			movement.ReleaseKeys([]string{"Space", "W"})
			movement.PressKey("Z")
		}

		// Reset target HP update time
		analyzer.GetStats().TargetHP.ResetLastUpdateTime()
		fb.obstacleAvoidanceCount++
		return false
	}

	// Too many attempts, abort
	fb.abortAttack(movement, analyzer)
	return true
}

// abortAttack aborts the current attack
func (fb *FarmingBehavior) abortAttack(movement *MovementCoordinator, analyzer *ImageAnalyzer) FarmingState {
	fb.isAttacking = false

	if fb.alreadyAttackCount > 0 {
		// Add marker area to avoidance
		if analyzer.GetStats().TargetMarker != nil {
			markerX := analyzer.GetStats().TargetMarker.X
			markerY := analyzer.GetStats().TargetMarker.Y
			bounds := Bounds{X: markerX - 20, Y: markerY - 20, W: 40, H: 40}
			growAmount := fb.alreadyAttackCount * 10
			grownBounds := bounds.Grow(growAmount)

			fb.avoidedBounds = append(fb.avoidedBounds, AvoidedArea{
				Bounds:    grownBounds,
				CreatedAt: time.Now(),
				Duration:  2 * time.Second,
			})
		}
		fb.alreadyAttackCount++
	} else {
		fb.obstacleAvoidanceCount = 0
		fb.isAttacking = false
		fb.avoidLastClick()
	}

	movement.PressKey("Escape")
	return FarmingStateSearchingForEnemy
}

// avoidLastClick adds last click position to avoidance list
func (fb *FarmingBehavior) avoidLastClick() {
	if fb.lastClickPos != nil {
		marker := Bounds{
			X: fb.lastClickPos.X - 1,
			Y: fb.lastClickPos.Y - 1,
			W: 2,
			H: 2,
		}
		fb.avoidedBounds = append(fb.avoidedBounds, AvoidedArea{
			Bounds:    marker,
			CreatedAt: time.Now(),
			Duration:  5 * time.Second,
		})
	}
}

// afterEnemyKill handles post-kill actions
func (fb *FarmingBehavior) afterEnemyKill(movement *MovementCoordinator, config *Config, stats *Statistics) FarmingState {
	// Record kill statistics
	killTime := time.Since(fb.lastInitialAttackTime)
	searchTime := fb.lastInitialAttackTime.Sub(fb.lastKillTime)
	stats.AddKill(killTime, searchTime)

	fb.killCount++
	fb.stealedTargetCount = 0
	fb.lastKillTime = time.Now()

	LogInfo(fmt.Sprintf("Kill #%d - Search: %v, Kill: %v", fb.killCount, searchTime, killTime))

	// Pickup items
	fb.performPickup(movement, config)

	// Reset for next target
	fb.currentTarget = nil

	return FarmingStateSearchingForEnemy
}

// updatePickupPet checks if pickup pet should be unsummoned based on cooldown
func (fb *FarmingBehavior) updatePickupPet(movement *MovementCoordinator, config *Config) {
	// Check if pet slot is configured
	if config.PickupPetSlot < 0 {
		return
	}

	// Get cooldown for pet slot (default to 3 seconds if not configured)
	cooldown := 3000 // ms
	if cd, ok := config.SlotCooldowns[config.PickupPetSlot]; ok && cd > 0 {
		cooldown = cd
	}

	// Check if enough time has passed since last summon
	timeSinceLastSummon := time.Since(fb.lastSummonPetTime).Milliseconds()
	if timeSinceLastSummon > int64(cooldown) {
		LogDebug("Unsummoning pickup pet (cooldown expired)")
		fb.sendSlot(movement, config, config.PickupPetSlot)
		fb.lastSummonPetTime = time.Now()
	}
}

// performPickup performs item pickup using pet or motion
func (fb *FarmingBehavior) performPickup(movement *MovementCoordinator, config *Config) {
	// Try pet-based pickup first
	if config.PickupPetSlot >= 0 {
		LogDebug("Picking up items with pet")
		fb.sendSlot(movement, config, config.PickupPetSlot)
		fb.lastSummonPetTime = time.Now()
		time.Sleep(1500 * time.Millisecond)
		fb.updatePickupPet(movement, config)
		return
	}

	// Fallback to motion-based pickup
	if config.PickupMotionSlot >= 0 {
		LogDebug("Picking up items with motion")
		fb.sendSlot(movement, config, config.PickupMotionSlot)
		time.Sleep(1 * time.Second)
		return
	}

	// Legacy pickup slots support
	if len(config.PickupSlots) > 0 {
		LogDebug("Picking up items (legacy)")
		movement.UseSkill(config.PickupSlots)
		time.Sleep(1 * time.Second)
	}
}

// sendSlot sends a slot keystroke with cooldown tracking
func (fb *FarmingBehavior) sendSlot(movement *MovementCoordinator, config *Config, slot int) {
	// Check if slot is on cooldown
	if lastUsage, ok := fb.slotUsageTimes[slot]; ok {
		cooldown := 0
		if cd, hasCd := config.SlotCooldowns[slot]; hasCd {
			cooldown = cd
		}

		timeSinceLastUse := time.Since(lastUsage).Milliseconds()
		if timeSinceLastUse < int64(cooldown) {
			LogDebug("Slot %d on cooldown, skipping", slot)
			return
		}
	}

	// Use the slot
	movement.UseSlot(slot)
	fb.slotUsageTimes[slot] = time.Now()
}

// checkRestorations checks and uses restoration items/skills
func (fb *FarmingBehavior) checkRestorations(movement *MovementCoordinator, config *Config, stats *ClientStats) {
	// Use party skills
	fb.usePartySkills(movement, config)

	hpValue := stats.HP.Value

	if hpValue > 0 {
		// Check HP - Pills first, then heal skills, then food
		if hpValue < config.HealThreshold {
			if len(config.HealSlots) > 0 {
				LogDebug("HP low (%d%%), using heal skill", hpValue)
				movement.UseSkill(config.HealSlots)
			} else if len(config.AOEHealSlots) > 0 {
				LogDebug("HP low (%d%%), using AOE heal", hpValue)
				movement.UseSkill(config.AOEHealSlots)
				time.Sleep(100 * time.Millisecond)
				movement.UseSkill(config.AOEHealSlots)
				time.Sleep(100 * time.Millisecond)
				movement.UseSkill(config.AOEHealSlots)
			}
		}

		// Check MP
		mpValue := stats.MP.Value
		if mpValue < config.MPThreshold && len(config.MPRestoreSlots) > 0 {
			LogDebug("MP low (%d%%), restoring", mpValue)
			movement.UseSkill(config.MPRestoreSlots)
		}

		// Check FP
		fpValue := stats.FP.Value
		if fpValue < config.FPThreshold && len(config.FPRestoreSlots) > 0 {
			LogDebug("FP low (%d%%), restoring", fpValue)
			movement.UseSkill(config.FPRestoreSlots)
		}
	}
}

// usePartySkills uses party buff skills
func (fb *FarmingBehavior) usePartySkills(movement *MovementCoordinator, config *Config) {
	if len(config.PartySkillSlots) == 0 {
		return
	}

	// Use all party skills that are not on cooldown
	for _, slot := range config.PartySkillSlots {
		// Check if slot is on cooldown using sendSlot
		fb.sendSlot(movement, config, slot)
		// Small delay between skills
		fb.wait(100 * time.Millisecond)
	}
}

// updateTimestamps updates internal timestamps
func (fb *FarmingBehavior) updateTimestamps() {
	// Update avoided bounds - remove expired ones
	now := time.Now()
	active := make([]AvoidedArea, 0)

	for _, avoided := range fb.avoidedBounds {
		if now.Sub(avoided.CreatedAt) < avoided.Duration {
			active = append(active, avoided)
		}
	}

	fb.avoidedBounds = active
}

// wait sets a wait duration
func (fb *FarmingBehavior) wait(duration time.Duration) {
	if fb.waitDuration != nil {
		newDuration := *fb.waitDuration + duration
		fb.waitDuration = &newDuration
	} else {
		fb.waitStart = time.Now()
		fb.waitDuration = &duration
	}
}

// waitCooldown checks if we should wait
func (fb *FarmingBehavior) waitCooldown() bool {
	if fb.waitDuration != nil {
		if time.Since(fb.waitStart) < *fb.waitDuration {
			return true
		}
		fb.waitDuration = nil
	}
	return false
}

// Stop stops the farming behavior
func (fb *FarmingBehavior) Stop() {
	fb.isAttacking = false
	fb.currentTarget = nil
	fb.state = FarmingStateSearchingForEnemy
}
