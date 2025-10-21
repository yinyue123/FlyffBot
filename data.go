// Package main - data.go
//
// This file defines core data structures used throughout the bot application.
// It provides geometric primitives, game state containers, configuration, and statistics.
//
// Major Data Categories:
//
// 1. Geometric Types:
//    - Point: 2D coordinates with distance calculations
//    - Bounds: Rectangles with center/size/containment operations
//    - PointCloud: Collection of points with clustering algorithms
//
// 2. Game State:
//    - Color: RGB color with tolerance matching
//    - Target: Detected mob with type and bounding box
//    - MobType: Enumeration (Passive, Aggressive, Violet)
//
// 3. Configuration:
//    - Config: All bot settings (mode, slots, thresholds, colors, behavior params)
//    - PersistentData: Container for config + cookies (saved to data.json)
//    - CookieData: Browser cookie representation
//
// 4. Statistics:
//    - Statistics: Kill tracking, KPM calculation, uptime
//
// 5. Screen Information:
//    - ScreenInfo: Resolution and coordinate scaling
//
// Note: StatusBar, AliveState, DetectedBar, and ClientStats have been moved to stats.go.
//
// Thread Safety:
// Config and PointCloud use RWMutex for concurrent access.
// All other types are value types and should be copied when shared.
//
// Clustering Algorithm:
// PointCloud.ClusterByDistance implements a two-pass clustering approach:
//   1. Sort by X coordinate and cluster within X distance threshold
//   2. Within each X cluster, sort by Y and cluster within Y threshold
// This produces bounding boxes around spatially close points (mob name pixels).
package main

import (
	"image"
	"math"
	"sort"
	"sync"
	"time"
)

// Point represents a 2D coordinate in screen space.
//
// Used for:
//   - Pixel coordinates during image scanning
//   - Click targets for mob selection
//   - Bounding box calculations
//   - Distance measurements
type Point struct {
	X int
	Y int
}

// NewPoint creates a new Point
func NewPoint(x, y int) Point {
	return Point{X: x, Y: y}
}

// Distance calculates Euclidean distance to another point
func (p Point) Distance(other Point) float64 {
	dx := float64(p.X - other.X)
	dy := float64(p.Y - other.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

// Bounds represents a rectangular area
type Bounds struct {
	X int // Top-left X coordinate
	Y int // Top-left Y coordinate
	W int // Width
	H int // Height
}

// NewBounds creates a new Bounds
func NewBounds(x, y, w, h int) Bounds {
	return Bounds{X: x, Y: y, W: w, H: h}
}

// Center returns the center point of the bounds
func (b Bounds) Center() Point {
	return Point{
		X: b.X + b.W/2,
		Y: b.Y + b.H/2,
	}
}

// BottomCenter returns the bottom center point (used for attack coordinates)
func (b Bounds) BottomCenter() Point {
	return Point{
		X: b.X + b.W/2,
		Y: b.Y + b.H,
	}
}

// Size returns the area of the bounds
func (b Bounds) Size() int {
	return b.W * b.H
}

// Contains checks if a point is within the bounds
func (b Bounds) Contains(p Point) bool {
	return p.X >= b.X && p.X <= b.X+b.W &&
		p.Y >= b.Y && p.Y <= b.Y+b.H
}

// Grow expands the bounds by the given amount in all directions
func (b Bounds) Grow(amount int) Bounds {
	return Bounds{
		X: b.X - amount/2,
		Y: b.Y - amount/2,
		W: b.W + amount,
		H: b.H + amount,
	}
}

// PointCloud represents a collection of points that can be clustered
type PointCloud struct {
	Points []Point
	mu     sync.RWMutex
}

// NewPointCloud creates a new point cloud
func NewPointCloud() *PointCloud {
	return &PointCloud{
		Points: make([]Point, 0),
	}
}

// Add adds a point to the cloud
func (pc *PointCloud) Add(p Point) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.Points = append(pc.Points, p)
}

// Len returns the number of points
func (pc *PointCloud) Len() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.Points)
}

// Clear removes all points
func (pc *PointCloud) Clear() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.Points = pc.Points[:0]
}

// ToBounds calculates the bounding box of all points
func (pc *PointCloud) ToBounds() Bounds {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if len(pc.Points) == 0 {
		return Bounds{}
	}

	minX, minY := pc.Points[0].X, pc.Points[0].Y
	maxX, maxY := pc.Points[0].X, pc.Points[0].Y

	for _, p := range pc.Points {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	return Bounds{
		X: minX,
		Y: minY,
		W: maxX - minX,
		H: maxY - minY,
	}
}

// ClusterByDistance groups points by distance threshold
// First clusters by X axis, then by Y axis within each X cluster
func (pc *PointCloud) ClusterByDistance(distanceX, distanceY int) []Bounds {
	pc.mu.RLock()
	points := make([]Point, len(pc.Points))
	copy(points, pc.Points)
	pc.mu.RUnlock()

	if len(points) == 0 {
		return nil
	}

	// Sort by X coordinate
	sort.Slice(points, func(i, j int) bool {
		return points[i].X < points[j].X
	})

	// Cluster by X axis
	var xClusters [][]Point
	currentCluster := []Point{points[0]}

	for i := 1; i < len(points); i++ {
		if points[i].X-points[i-1].X <= distanceX {
			currentCluster = append(currentCluster, points[i])
		} else {
			xClusters = append(xClusters, currentCluster)
			currentCluster = []Point{points[i]}
		}
	}
	xClusters = append(xClusters, currentCluster)

	// Cluster each X cluster by Y axis
	var result []Bounds
	for _, xCluster := range xClusters {
		// Sort by Y coordinate
		sort.Slice(xCluster, func(i, j int) bool {
			return xCluster[i].Y < xCluster[j].Y
		})

		currentYCluster := []Point{xCluster[0]}
		for i := 1; i < len(xCluster); i++ {
			if xCluster[i].Y-xCluster[i-1].Y <= distanceY {
				currentYCluster = append(currentYCluster, xCluster[i])
			} else {
				result = append(result, pointsToBounds(currentYCluster))
				currentYCluster = []Point{xCluster[i]}
			}
		}
		result = append(result, pointsToBounds(currentYCluster))
	}

	return result
}

// pointsToBounds converts a slice of points to bounds
func pointsToBounds(points []Point) Bounds {
	if len(points) == 0 {
		return Bounds{}
	}

	minX, minY := points[0].X, points[0].Y
	maxX, maxY := points[0].X, points[0].Y

	for _, p := range points {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	return Bounds{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}
}

// MobType represents the type of mob
type MobType int

const (
	MobPassive MobType = iota // Yellow name mobs
	MobAggressive              // Red name mobs
	MobViolet                  // Purple/Violet Magician Troupe
)

// Target represents a detected target (mob or player)
type Target struct {
	Type   MobType
	Bounds Bounds
}

// AttackCoords returns the coordinates to click for attacking
func (t *Target) AttackCoords() Point {
	return t.Bounds.BottomCenter()
}

// Color represents an RGB color
type Color struct {
	R uint8
	G uint8
	B uint8
}

// NewColor creates a new Color
func NewColor(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b}
}

// Matches checks if another color matches within tolerance
func (c Color) Matches(other Color, tolerance uint8) bool {
	return absDiff(c.R, other.R) <= tolerance &&
		absDiff(c.G, other.G) <= tolerance &&
		absDiff(c.B, other.B) <= tolerance
}

// absDiff returns absolute difference between two uint8 values
func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// Note: StatusBar, AliveState, DetectedBar, and ClientStats have been moved to stats.go
// for better organization and to implement the correct pixel-based detection algorithm.

// Config holds bot configuration
type Config struct {
	Mode              string // "Farming" or "Support"

	// Slot assignments (slot numbers 0-9)
	AttackSlots       []int
	AOEAttackSlots    []int
	HealSlots         []int
	AOEHealSlots      []int
	BuffSlots         []int
	MPRestoreSlots    []int
	FPRestoreSlots    []int
	PickupSlots       []int
	PickupPetSlot     int  // Slot for pickup pet summon
	PickupMotionSlot  int  // Slot for motion-based pickup
	RezSlots          []int // Resurrection skill slots
	PartySkillSlots   []int // Party buff skill slots (auto-cast periodically)

	// Thresholds (0-100 in 10% increments)
	HealThreshold     int
	MPThreshold       int
	FPThreshold       int

	// Mob colors
	PassiveColor      Color
	AggressiveColor   Color
	VioletColor       Color
	PassiveTolerance  uint8
	AggressiveTolerance uint8
	VioletTolerance   uint8

	// Behavior settings
	PrioritizeAggro            bool
	MinMobNameWidth            int
	MaxMobNameWidth            int
	CircleMoveDuration         int
	MinHPAttack                int  // Minimum HP% to attack passive mobs
	StopFighting               bool // Stop fighting flag
	ObstacleAvoidanceMaxTry    int  // Max tries to avoid obstacle
	ObstacleAvoidanceCooldown  int  // Cooldown in ms before obstacle avoidance
	MaxAOEFarming              int  // Max concurrent mobs for AOE
	MobsTimeout                int  // Timeout in ms when no mobs found

	// Support mode settings
	FollowDistance    int
	InParty           bool

	// Shout mode settings
	ShoutMessages  []string // Messages to shout
	ShoutInterval  int      // Interval between shouts in ms

	// Capture frequency settings (in milliseconds)
	CaptureInterval   int // 0=continuous, 1000=1s, 2000=2s, 3000=3s, 4000=4s

	// Slot cooldown tracking (in milliseconds)
	SlotCooldowns     map[int]int // slot number -> cooldown duration in ms

	mu                sync.RWMutex
}

// NewConfig creates default configuration
func NewConfig() *Config {
	return &Config{
		Mode:                      "Farming",
		AttackSlots:               []int{0},
		AOEAttackSlots:            []int{},
		HealSlots:                 []int{1},
		AOEHealSlots:              []int{},
		BuffSlots:                 []int{},
		MPRestoreSlots:            []int{2},
		FPRestoreSlots:            []int{3},
		PickupSlots:               []int{4},
		HealThreshold:             50,
		MPThreshold:               30,
		FPThreshold:               30,
		PassiveColor:              NewColor(234, 234, 149),
		AggressiveColor:           NewColor(179, 23, 23),
		VioletColor:               NewColor(182, 144, 146),
		PassiveTolerance:          5,
		AggressiveTolerance:       5,
		VioletTolerance:           5,
		PrioritizeAggro:           true,
		MinMobNameWidth:           15,
		MaxMobNameWidth:           150,
		CircleMoveDuration:        100,
		MinHPAttack:               70,
		StopFighting:              false,
		ObstacleAvoidanceMaxTry:   3,
		ObstacleAvoidanceCooldown: 5000,
		MaxAOEFarming:             1,
		MobsTimeout:               0, // 0 = disabled
		FollowDistance:            200,
		InParty:                   false,
		ShoutMessages:             []string{},
		ShoutInterval:             30000, // 30 seconds
		CaptureInterval:           1000,  // Default to 1 second
		PickupPetSlot:             -1,    // -1 = disabled
		PickupMotionSlot:          -1,    // -1 = disabled
		RezSlots:                  []int{},
		PartySkillSlots:           []int{},
		SlotCooldowns:             make(map[int]int),
	}
}

// GetMode safely returns current mode
func (c *Config) GetMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Mode
}

// SetMode safely sets the mode
func (c *Config) SetMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Mode = mode
}

// PersistentData holds all data that should be saved
type PersistentData struct {
	Config  *Config         `json:"config"`
	Cookies []CookieData    `json:"cookies"`
}

// CookieData represents a browser cookie
type CookieData struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// NewPersistentData creates a new persistent data structure
func NewPersistentData() *PersistentData {
	return &PersistentData{
		Config:  NewConfig(),
		Cookies: make([]CookieData, 0),
	}
}

// Statistics holds runtime statistics
type Statistics struct {
	StartTime        time.Time
	KillCount        int
	LastKillTime     time.Time
	TotalKillTime    time.Duration
	TotalSearchTime  time.Duration
	mu               sync.RWMutex
}

// NewStatistics creates new statistics
func NewStatistics() *Statistics {
	return &Statistics{
		StartTime: time.Now(),
	}
}

// AddKill records a new kill
func (s *Statistics) AddKill(killTime, searchTime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.KillCount++
	s.LastKillTime = time.Now()
	s.TotalKillTime += killTime
	s.TotalSearchTime += searchTime
}

// KillsPerMinute calculates kills per minute
func (s *Statistics) KillsPerMinute() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	elapsed := time.Since(s.StartTime).Minutes()
	if elapsed <= 0 {
		return 0
	}
	return float64(s.KillCount) / elapsed
}

// KillsPerHour calculates kills per hour
func (s *Statistics) KillsPerHour() float64 {
	return s.KillsPerMinute() * 60
}

// GetStats returns formatted statistics
func (s *Statistics) GetStats() (kills int, kpm, kph float64, uptime string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kills = s.KillCount
	kpm = s.KillsPerMinute()
	kph = kpm * 60
	uptime = FormatDuration(time.Since(s.StartTime))
	return
}

// ScreenInfo holds screen resolution information
type ScreenInfo struct {
	Width  int
	Height int
	Bounds image.Rectangle
}

// NewScreenInfo creates screen info from rectangle
func NewScreenInfo(bounds image.Rectangle) *ScreenInfo {
	return &ScreenInfo{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Bounds: bounds,
	}
}

// Scale calculates scaled coordinates based on resolution
// Base resolution is 800x600
func (si *ScreenInfo) Scale(baseX, baseY int) (int, int) {
	scaleX := float64(si.Width) / 800.0
	scaleY := float64(si.Height) / 600.0

	return int(float64(baseX) * scaleX), int(float64(baseY) * scaleY)
}

// ScaleBounds scales bounds based on resolution
func (si *ScreenInfo) ScaleBounds(baseBounds Bounds) Bounds {
	x, y := si.Scale(baseBounds.X, baseBounds.Y)
	w := int(float64(baseBounds.W) * float64(si.Width) / 800.0)
	h := int(float64(baseBounds.H) * float64(si.Height) / 600.0)

	return Bounds{X: x, Y: y, W: w, H: h}
}

// Center returns the center point of the screen
func (si *ScreenInfo) Center() Point {
	return Point{X: si.Width / 2, Y: si.Height / 2}
}
