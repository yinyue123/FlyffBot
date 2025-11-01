# Flyff Bot - Go Edition

A simplified, streamlined automation tool for Flyff Universe, rewritten in Go with a clean menu-driven interface.

## Features

- **Simple Menu-Based UI**: All configuration through dropdown menus, no complex slot-by-slot setup
- **Two Automation Modes**: Farming and Support behaviors
- **Resolution Adaptive**: Automatically scales to your screen resolution (not limited to 800x600)
- **Real-time Statistics**: Monitor kills, KPM (kills per minute), and uptime in the status bar
- **Cross-Platform**: Works on Windows, macOS, and Linux
- **Detailed Logging**: All actions logged to `Debug.log` in the current directory

## Architecture

The project is organized into focused, single-purpose files:

```
flyff/
├── main.go          # Entry point and bot controller
├── interface.go     # Fyne-based GUI with game window display
├── toolbar.go       # Menu system for configuration
├── platform.go      # Cross-platform screen capture and input simulation
├── data.go          # Core data structures and algorithms
├── movement.go      # Character movement and action coordination
├── analyzer.go      # Image recognition engine (mobs, HP/MP/FP detection)
├── behavior.go      # Farming and Support behavior implementations
├── utils.go         # Logging, timing, and utility functions
├── go.mod           # Go module dependencies
├── Debug.log        # Runtime log file (auto-created)
└── README.md        # This file
```

## Installation

### Prerequisites

- Go 1.21 or higher
- C compiler (for CGO dependencies)
  - **Windows**: MinGW-w64 or Visual Studio
  - **macOS**: Xcode Command Line Tools
  - **Linux**: GCC

### Build

```bash
cd flyff
go mod download
go build -o flyff-bot
```

## Usage

### Running the Bot

```bash
./flyff-bot
```

The application will:
1. Open a window showing the game screen
2. Display a menu bar at the top
3. Show real-time status at the bottom
4. Log all actions to `Debug.log`

### Configuration

All configuration is done through the menu bar:

#### **Mode Menu**
- **Farming**: Auto-detect and attack mobs
- **Support**: Follow and heal party leader

#### **Slots Menu**
Configure which hotbar slots (0-9) to use for each function:

- **Attack Slots**: Skills for attacking mobs (Farming mode)
- **Heal Slots**: Healing skills/items for HP restoration
- **Buff Slots**: Buff skills to cast periodically
- **MP Restore Slots**: MP potions/skills
- **FP Restore Slots**: FP (stamina) restoration
- **Pickup Slots**: Pet summon or pickup motion

**Example**: To use slots 0, 1, and 2 for attacks, enter: `0,1,2`

#### **Thresholds Menu**
Set when to trigger restoration actions (in 10% increments):

- **HP Threshold**: Use heal when HP drops below this % (e.g., 50%)
- **MP Threshold**: Use MP restore when MP drops below this %
- **FP Threshold**: Use FP restore when FP drops below this %

#### **Settings Menu**
- **Mob Colors**: View default mob detection colors
  - Passive mobs: Yellow (RGB 234, 234, 149)
  - Aggressive mobs: Red (RGB 179, 23, 23)
- **Behavior**:
  - **Prioritize Aggressive Mobs**: Attack red mobs first
  - **In Party**: Enable party support mode features

#### **Help Menu**
- **View Log**: Open the Debug.log file
- **About**: Version and credits

### Controls

- **Start Bot**: Click the green "Start Bot" button or use menu
- **Stop Bot**: Click the red "Stop Bot" button or use menu
- **Quit**: Close the window or press Ctrl+C in terminal

## How It Works

### Farming Mode

1. **Mob Detection**: Scans screen for yellow (passive) and red (aggressive) mob names
2. **Target Selection**: Finds closest mob to screen center
3. **Combat**: Clicks mob, uses attack skills, monitors target HP
4. **Obstacle Avoidance**: If stuck, jumps in random directions
5. **Pickup**: After kill, uses pickup slot to collect loot
6. **Restoration**: Automatically uses heals/MP/FP when below thresholds
7. **Circle Movement**: If no mobs found, moves in circles to search

### Support Mode

1. **Leader Selection**: Selects party leader as target
2. **Following**: Uses 'Z' key to follow target
3. **Distance Check**: Ensures staying within range
4. **Healing**: Monitors target HP and heals when needed
5. **Buffing**: Casts buffs on target periodically
6. **Self-Care**: Heals self if HP drops
7. **AFK Prevention**: Random camera movements

### Image Recognition

The analyzer uses parallel pixel scanning to detect:

- **Mob Names**: Color-based detection of name text
  - Yellow text = Passive mob
  - Red text = Aggressive mob
- **Status Bars**: HP/MP/FP bar width measurement
  - Red pixels = HP
  - Blue pixels = MP
  - Green pixels = FP
- **Target Marker**: Red arrow above selected target
- **Point Cloud Clustering**: Groups pixels into bounding boxes

### Resolution Scaling

All coordinates and regions automatically scale based on your resolution:

```go
// Base resolution: 800x600
// Your resolution: e.g., 1920x1080
scaledX = baseX * (1920 / 800)
scaledY = baseY * (1080 / 600)
```

This ensures detection works on any common resolution.

## File Descriptions

### main.go (Bot Controller)
- Application entry point
- Creates and manages bot instance
- Main loop coordination (10 Hz update rate)
- Signal handling for graceful shutdown
- Behavior switching (Farming/Support)

### interface.go (GUI)
- Fyne-based window with game view
- Start/Stop button
- Status display integration
- Image rendering
- Event callbacks

### toolbar.go (Menu System)
- Menu bar creation
- Configuration dialogs
- Slot number parsing
- Threshold selection (10% increments)
- Settings persistence

### platform.go (OS Abstraction)
- Screen capture (screenshot library)
- Keyboard input simulation
- Mouse click simulation
- Cross-platform key mapping
- Parallel pixel scanning (multi-core)
- Ignore area calculation (UI elements)

### data.go (Data Structures)
- `Point`: 2D coordinates
- `Bounds`: Rectangular regions
- `PointCloud`: Pixel clustering
- `Target`: Detected mob with bounds
- `Color`: RGB with tolerance matching
- `StatusBar`: HP/MP/FP tracking
- `ClientStats`: Player state
- `Config`: Bot configuration
- `Statistics`: Runtime metrics
- `ScreenInfo`: Resolution scaling

### analyzer.go (Image Recognition)
- Screen capture integration
- Mob identification (color-based)
- Status bar detection (HP/MP/FP)
- Target marker detection
- Distance estimation
- Point cloud clustering algorithm
- Avoidance list management

### movement.go (Movement Control)
- Key press/hold/release
- Movement patterns (forward, backward, rotate)
- Circle movement (farming pattern)
- Obstacle avoidance (jump maneuvers)
- Target locking (Z key)
- Slot usage (hotbar 0-9)
- Chat messages
- Party leader selection

### behavior.go (Bot Logic)
- `FarmingBehavior`: Mob hunting logic
  - State machine: Search → Click → Verify → Attack → Loot
  - Rotation search pattern
  - Obstacle detection and avoidance
  - Kill statistics tracking
- `SupportBehavior`: Party support logic
  - Leader following
  - Distance monitoring
  - Target healing
  - Periodic buffing
  - AFK prevention

### utils.go (Utilities)
- **Logger**: Thread-safe file logging
  - Levels: DEBUG, INFO, WARN, ERROR
  - Timestamp with microseconds
  - Writes to Debug.log
- **Timer**: Performance measurement
  - Named timers
  - Automatic logging on completion
- **RateLimiter**: Execution throttling
- **Helpers**: Clamping, formatting, safe goroutines

## Configuration Examples

### Basic Farmer Setup

1. **Mode**: Farming
2. **Attack Slots**: `0,1` (two attack skills)
3. **Heal Slots**: `2` (healing skill or HP potion)
4. **MP Restore**: `3` (MP potion)
5. **Pickup**: `4` (pickup pet or motion)
6. **HP Threshold**: 50%
7. **MP Threshold**: 30%
8. **Prioritize Aggro**: ✓ (enabled)

### Support Setup for Assist

1. **Mode**: Support
2. **Heal Slots**: `0,1` (single-target and AoE heals)
3. **Buff Slots**: `2,3` (buffs to cast on leader)
4. **MP Restore**: `4`
5. **HP Threshold**: 70% (heal target earlier)
6. **In Party**: ✓ (enabled)

## Performance

- **CPU Usage**: ~5-15% (mostly screen capture)
- **Memory**: ~50-100 MB
- **Update Rate**: 10 Hz (10 iterations per second)
- **Image Processing**: Parallel (uses all CPU cores)

Typical operation times:
- Screen capture: 5-10ms
- Pixel scanning: 3-5ms
- Point clustering: 1-2ms
- Total loop: 20-40ms (allowing 60ms idle time)

## Logging

All bot actions are logged to `Debug.log`:

```
2025/01/20 14:30:15.123456 [INFO] Logger initialized
2025/01/20 14:30:15.234567 [INFO] Starting bot in Farming mode
2025/01/20 14:30:16.345678 [DEBUG] Found 5 passive points, 2 aggressive points
2025/01/20 14:30:16.456789 [INFO] Target acquired
2025/01/20 14:30:20.567890 [INFO] Target defeated
2025/01/20 14:30:20.678901 [DEBUG] Stats - HP: 85%, MP: 60%, FP: 90%
```

Log levels:
- **DEBUG**: Detailed execution info (pixel counts, coordinates)
- **INFO**: Major events (target acquired, killed)
- **WARN**: Issues that don't stop execution
- **ERROR**: Critical failures

## Safety and Limitations

### What the Bot Does
- ✓ Detects mobs by name color
- ✓ Clicks on mobs to attack
- ✓ Uses configured hotbar slots
- ✓ Monitors HP/MP/FP and restores
- ✓ Picks up items after kills
- ✓ Follows party leader (Support mode)

### What the Bot Doesn't Do
- ✗ Handle complex terrain/obstacles perfectly
- ✗ Respond to PvP attacks
- ✗ Navigate to farming spots automatically
- ✗ Manage inventory (will stop when full)
- ✗ Handle server disconnects
- ✗ Avoid GMs or detection

### Risks
- **Against ToS**: Using bots violates Flyff's Terms of Service
- **Ban Risk**: Account may be permanently banned
- **Use at own risk**: No guarantees or support for banned accounts

### Best Practices
- Test on a secondary account first
- Don't run 24/7 (use reasonable schedules)
- Monitor the bot periodically
- Have the game window visible (minimized may break detection)
- Use safe, popular farming spots
- Don't interact with other players while botting

## Troubleshooting

### Bot doesn't detect mobs
- Ensure game resolution is standard (1920x1080, 1600x900, 1280x720, 800x600)
- Check mob name colors haven't changed (game updates)
- Verify game window is fully visible (not covered)
- Check Debug.log for "Found X points" messages

### Bot clicks wrong locations
- Resolution scaling may need adjustment
- Check if game UI layout changed
- Try restarting the bot

### HP/MP not detected
- Status tray must be open in-game (press 'T')
- Check Debug.log for "Stats - HP: X%" messages
- Verify status bar colors haven't changed

### Frequent "obstacle avoidance"
- Terrain too complex
- Adjust circle movement duration in code
- Move to flatter farming area

### Logs not created
- Check file permissions in current directory
- Ensure you have write access
- Debug.log should appear immediately on start

## Development

### Adding New Features

The modular design makes extensions easy:

1. **New Behavior**: Implement `BotBehavior` interface in `behavior.go`
2. **New Detection**: Add methods to `ImageAnalyzer` in `analyzer.go`
3. **New Movements**: Add methods to `MovementCoordinator` in `movement.go`
4. **New Config**: Add fields to `Config` struct in `data.go`
5. **New Menu**: Add items in `Toolbar.createMenu()` in `toolbar.go`

### Code Style

- All comments in English (no Chinese characters)
- Descriptive function names
- Struct-based organization
- Interface-driven behavior
- Goroutine-safe with mutexes

## Credits

Based on the original Neuz project (Rust + TypeScript), redesigned in Go with simplified architecture.

## License

Use at your own risk. This project is for educational purposes only.

## Support

Check `Debug.log` for diagnostic information. All bot actions and errors are logged there.
