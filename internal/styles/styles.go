// Package styles provides UI styling based on Zoea Nova brand colors.
package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors - Retro-futuristic aesthetic based on Zoea Nova brand
// Brand colors from logo: #9D00FF (electric purple), #00FFCC (bright teal)
var (
	// Brand colors
	ColorBrand    = lipgloss.Color("#9D00FF") // Electric purple (from logo)
	ColorTeal     = lipgloss.Color("#00FFCC") // Bright teal (from logo)
	ColorBrandDim = lipgloss.Color("#6B00B3") // Dimmed purple for subtle accents
	ColorTealDim  = lipgloss.Color("#00AA99") // Dimmed teal

	// Role colors
	ColorUser      = lipgloss.Color("#00FF66") // Bright green for user messages
	ColorAssistant = lipgloss.Color("#FF00CC") // Magenta/pink for assistant
	ColorSystem    = lipgloss.Color("#00CCFF") // Cyan for system messages
	ColorTool      = lipgloss.Color("#FFCC00") // Yellow/gold for tool calls

	// Semantic colors
	ColorError   = lipgloss.Color("#FF3366") // Error red-pink
	ColorSuccess = lipgloss.Color("#00FF66") // Success green
	ColorMuted   = lipgloss.Color("#5555AA") // Muted purple-gray

	// Backgrounds - deep space with purple undertones
	ColorBg      = lipgloss.Color("#08080F") // Deep space black
	ColorBgAlt   = lipgloss.Color("#101018") // Slightly lighter
	ColorBgPanel = lipgloss.Color("#14141F") // Panel background
	ColorBorder  = lipgloss.Color("#2A2A55") // Purple-tinted border
)

// Base styles
var (
	BaseStyle = lipgloss.NewStyle().
			Background(ColorBg)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBrand)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)
)
