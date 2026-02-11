// Package tui provides the terminal user interface for Mysis.
package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/mysis/internal/styles"
)

// TUI-specific styles building on base styles
var (
	// Log/Conversation styles
	LogStyle = lipgloss.NewStyle().
			Background(styles.ColorBg)

	// Role-based message styles
	UserStyle = lipgloss.NewStyle().
			Foreground(styles.ColorUser).
			Background(styles.ColorBg).
			Bold(true)

	AssistantStyle = lipgloss.NewStyle().
			Foreground(styles.ColorAssistant).
			Background(styles.ColorBg)

	SystemStyle = lipgloss.NewStyle().
			Foreground(styles.ColorSystem).
			Background(styles.ColorBg).
			Italic(true)

	ToolStyle = lipgloss.NewStyle().
			Foreground(styles.ColorTool).
			Background(styles.ColorBg)

	ToolSuccessStyle = lipgloss.NewStyle().
				Foreground(styles.ColorSuccess).
				Background(styles.ColorBg)

	ToolErrorStyle = lipgloss.NewStyle().
			Foreground(styles.ColorError).
			Background(styles.ColorBg)

	// Input styles
	InputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true, false, false, false). // Top border only
				BorderForeground(styles.ColorBorder).
				Background(styles.ColorBg).
				Padding(0, 1)

	InputPromptStyle = lipgloss.NewStyle().
				Foreground(styles.ColorBrand).
				Background(styles.ColorBg).
				Bold(true)

	InputTextStyle = lipgloss.NewStyle().
			Foreground(styles.ColorTeal).
			Background(styles.ColorBg)

	InputPlaceholderStyle = lipgloss.NewStyle().
				Foreground(styles.ColorMuted).
				Background(styles.ColorBg).
				Italic(true)

	// Status bar styles
	StatusBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false). // Top border only
			BorderForeground(styles.ColorBorder).
			Background(styles.ColorBg)

	// Status icon styles (3-char width each: [ <icon> ])
	IconAutoplayStyle = lipgloss.NewStyle().
				Foreground(styles.ColorTeal).
				Background(styles.ColorBg).
				Width(3).
				Align(lipgloss.Center)

	IconInfoStyle = lipgloss.NewStyle().
			Foreground(styles.ColorSuccess).
			Background(styles.ColorBg).
			Width(3).
			Align(lipgloss.Center)

	IconWarningStyle = lipgloss.NewStyle().
				Foreground(styles.ColorTool).
				Background(styles.ColorBg).
				Width(3).
				Align(lipgloss.Center)

	IconErrorStyle = lipgloss.NewStyle().
			Foreground(styles.ColorError).
			Background(styles.ColorBg).
			Width(3).
			Align(lipgloss.Center)

	// Connection status icons (network activity)
	IconLLMStyle = lipgloss.NewStyle().
			Foreground(styles.ColorTeal). // Cyan/teal for LLM thinking
			Background(styles.ColorBg).
			Width(3).
			Align(lipgloss.Center)

	IconMCPStyle = lipgloss.NewStyle().
			Foreground(styles.ColorBrand). // Purple for MCP server communication
			Background(styles.ColorBg).
			Width(3).
			Align(lipgloss.Center)

	// Status text styles
	StatusTextStyle = lipgloss.NewStyle().
			Foreground(styles.ColorMuted).
			Background(styles.ColorBg)

	StatusTextErrorStyle = lipgloss.NewStyle().
				Foreground(styles.ColorError).
				Background(styles.ColorBg)

	StatusTextOKStyle = lipgloss.NewStyle().
				Foreground(styles.ColorSuccess).
				Background(styles.ColorBg)

	// Scrollbar style
	ScrollbarStyle = lipgloss.NewStyle().
			Foreground(styles.ColorBorder).
			Background(styles.ColorBg)

	// Dimmed text
	DimmedStyle = lipgloss.NewStyle().
			Foreground(styles.ColorMuted).
			Background(styles.ColorBg)
)

// RoleStyle returns the appropriate style for a message role.
func RoleStyle(role string) lipgloss.Style {
	switch role {
	case "user":
		return UserStyle
	case "assistant":
		return AssistantStyle
	case "system":
		return SystemStyle
	case "tool":
		return ToolStyle
	default:
		return SystemStyle
	}
}

// RoleLabel returns a styled label for message roles.
func RoleLabel(role string) string {
	switch role {
	case "user":
		return UserStyle.Render("You")
	case "assistant":
		return AssistantStyle.Render("Agent")
	case "system":
		return SystemStyle.Render("System")
	case "tool":
		return ToolStyle.Render("Tool")
	default:
		return DimmedStyle.Render("Unknown")
	}
}
