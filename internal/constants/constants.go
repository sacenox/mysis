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
)
