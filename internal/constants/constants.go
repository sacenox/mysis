// Package constants provides application-wide constants.
package constants

import "time"

const (
	// AppName is the application name.
	AppName = "mysis"

	// AppDataDir is the directory name for application data.
	AppDataDir = ".config/mysis"
)

// Timing constants
const (
	// DefaultTimeout is the default timeout for operations.
	DefaultTimeout = 30 * time.Second

	// GameTickDuration is the SpaceMolt game server tick duration.
	GameTickDuration = 10 * time.Second

	// AvgToolCallsPerTurn is the expected average tool calls per turn for autoplay timing.
	// Database analysis shows actual average is ~3, but we use 10 for safety margin.
	AvgToolCallsPerTurn = 10
)

var (
	// AutoplayInterval is the interval between autoplay turns.
	// Calculated as: AvgToolCallsPerTurn × GameTickDuration × 0.75
	// Per DESIGN.md: "game tick time * max tool calls * .75"
	AutoplayInterval = time.Duration(float64(AvgToolCallsPerTurn)*GameTickDuration.Seconds()*0.75) * time.Second
)
