package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/mysis/internal/styles"
)

// StatusBar manages the bottom status bar with animated icons and status text.
type StatusBar struct {
	width int

	// Icon animation state
	autoplayFrames int // Remaining animation frames for autoplay icon
	infoFrames     int // Remaining animation frames for info icon
	warningFrames  int // Remaining animation frames for warning icon
	errorFrames    int // Remaining animation frames for error icon
	llmFrames      int // Remaining animation frames for LLM connection icon
	mcpFrames      int // Remaining animation frames for MCP connection icon

	currentFrame int // Current animation frame (0-11 for 12 frames @ 8 FPS)

	// Status text
	errorText    string
	warningText  string
	autoplayText string
}

const (
	animationFPSFast     = 8                                                  // 8 frames per second (fast animation at start)
	animationFPSSlow     = 2                                                  // 2 frames per second (slow animation during deceleration)
	framesPerCycle       = 12                                                 // 12 frames in one complete cycle
	fastCycles           = 2                                                  // Number of fast cycles before deceleration
	decelerationFrames   = 12                                                 // Number of frames for deceleration phase
	totalAnimationFrames = (fastCycles * framesPerCycle) + decelerationFrames // 24 + 12 = 36
)

// StatusBarTickMsg is sent every animation frame.
type StatusBarTickMsg struct{}

// NewStatusBar creates a new status bar.
func NewStatusBar(width int) StatusBar {
	return StatusBar{
		width: width,
	}
}

// Init initializes the status bar.
func (s StatusBar) Init() tea.Cmd {
	return s.tick()
}

// maxFrames returns the maximum frame count across all icons.
func (s StatusBar) maxFrames() int {
	return max(s.autoplayFrames, s.infoFrames, s.warningFrames,
		s.errorFrames, s.llmFrames, s.mcpFrames)
}

// tick returns a command that sends a tick message after the animation interval.
// The tick rate varies: fast during initial cycles, slow during deceleration, stops when idle.
func (s StatusBar) tick() tea.Cmd {
	// Find the maximum frame count across all icons to determine animation phase
	maxFrames := max(s.autoplayFrames, s.infoFrames, s.warningFrames,
		s.errorFrames, s.llmFrames, s.mcpFrames)

	// No animation needed if all icons are idle
	if maxFrames == 0 {
		return nil // Stop ticking when idle (icons at baseline/thinnest frame)
	}

	// Calculate tick rate based on animation phase
	var tickRate time.Duration

	if maxFrames > decelerationFrames {
		// Fast phase: quick animation for first 24 frames (2 cycles)
		tickRate = time.Second / animationFPSFast // 125ms (8 FPS)
	} else {
		// Deceleration phase: slow down for last 12 frames
		tickRate = time.Second / animationFPSSlow // 500ms (2 FPS)
	}

	return tea.Tick(tickRate, func(time.Time) tea.Msg {
		return StatusBarTickMsg{}
	})
}

// Update handles status bar updates.
func (s StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	if _, ok := msg.(StatusBarTickMsg); ok {
		// Advance animation frame
		s.currentFrame = (s.currentFrame + 1) % framesPerCycle

		// Decrement frame counters
		if s.autoplayFrames > 0 {
			s.autoplayFrames--
		}
		if s.infoFrames > 0 {
			s.infoFrames--
		}
		if s.warningFrames > 0 {
			s.warningFrames--
		}
		if s.errorFrames > 0 {
			s.errorFrames--
		}
		if s.llmFrames > 0 {
			s.llmFrames--
		}
		if s.mcpFrames > 0 {
			s.mcpFrames--
		}

		return s, s.tick()
	}

	return s, nil
}

// SetWidth updates the status bar width.
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// AnimateAutoplay triggers the autoplay icon animation.
// Resets to full animation cycle on each event.
// Returns a command to start/restart the animation tick if needed.
func (s *StatusBar) AnimateAutoplay() tea.Cmd {
	wasIdle := s.maxFrames() == 0
	s.autoplayFrames = totalAnimationFrames
	if wasIdle {
		return s.tick() // Restart ticking when transitioning from idle
	}
	return nil
}

// AnimateInfo triggers the info icon animation.
// Resets to full animation cycle on each event.
// Returns a command to start/restart the animation tick if needed.
func (s *StatusBar) AnimateInfo() tea.Cmd {
	wasIdle := s.maxFrames() == 0
	s.infoFrames = totalAnimationFrames
	if wasIdle {
		return s.tick() // Restart ticking when transitioning from idle
	}
	return nil
}

// AnimateWarning triggers the warning icon animation.
// Resets to full animation cycle on each event.
// Returns a command to start/restart the animation tick if needed.
func (s *StatusBar) AnimateWarning() tea.Cmd {
	wasIdle := s.maxFrames() == 0
	s.warningFrames = totalAnimationFrames
	if wasIdle {
		return s.tick() // Restart ticking when transitioning from idle
	}
	return nil
}

// SetWarning sets the warning text.
func (s *StatusBar) SetWarning(text string) tea.Cmd {
	s.warningText = text
	return s.AnimateWarning()
}

// ClearWarning clears the warning text.
func (s *StatusBar) ClearWarning() {
	s.warningText = ""
}

// AnimateError triggers the error icon animation.
// Resets to full animation cycle on each event.
// Returns a command to start/restart the animation tick if needed.
func (s *StatusBar) AnimateError() tea.Cmd {
	wasIdle := s.maxFrames() == 0
	s.errorFrames = totalAnimationFrames
	if wasIdle {
		return s.tick() // Restart ticking when transitioning from idle
	}
	return nil
}

// AnimateLLM triggers the LLM connection icon animation.
// Resets to full animation cycle on each event.
// Returns a command to start/restart the animation tick if needed.
func (s *StatusBar) AnimateLLM() tea.Cmd {
	wasIdle := s.maxFrames() == 0
	s.llmFrames = totalAnimationFrames
	if wasIdle {
		return s.tick() // Restart ticking when transitioning from idle
	}
	return nil
}

// AnimateMCP triggers the MCP connection icon animation.
// Resets to full animation cycle on each event.
// Returns a command to start/restart the animation tick if needed.
func (s *StatusBar) AnimateMCP() tea.Cmd {
	wasIdle := s.maxFrames() == 0
	s.mcpFrames = totalAnimationFrames
	if wasIdle {
		return s.tick() // Restart ticking when transitioning from idle
	}
	return nil
}

// SetError sets the error text.
func (s *StatusBar) SetError(text string) tea.Cmd {
	s.errorText = text
	return s.AnimateError()
}

// ClearError clears the error text.
func (s *StatusBar) ClearError() {
	s.errorText = ""
}

// SetAutoplayText sets the autoplay status text and animates the icon.
func (s *StatusBar) SetAutoplayText(text string) tea.Cmd {
	s.autoplayText = text
	return s.AnimateAutoplay()
}

// ClearAutoplayText clears the autoplay text.
func (s *StatusBar) ClearAutoplayText() {
	s.autoplayText = ""
}

// View renders the status bar.
func (s StatusBar) View() string {
	// Left side: Status icon column (4 icons × 3 chars each = 12 chars)
	autoplayIcon := s.renderIcon(s.autoplayFrames, autoplayIcons)
	infoIcon := s.renderIcon(s.infoFrames, infoIcons)
	warningIcon := s.renderIcon(s.warningFrames, warningIcons)
	errorIcon := s.renderIcon(s.errorFrames, errorIcons)

	leftIconColumn := IconAutoplayStyle.Render(autoplayIcon) +
		IconInfoStyle.Render(infoIcon) +
		IconWarningStyle.Render(warningIcon) +
		IconErrorStyle.Render(errorIcon)

	// Right side: Connection icon column (2 icons × 3 chars each = 6 chars)
	llmIcon := s.renderIcon(s.llmFrames, llmIcons)
	mcpIcon := s.renderIcon(s.mcpFrames, mcpIcons)

	rightIconColumn := IconLLMStyle.Render(llmIcon) +
		IconMCPStyle.Render(mcpIcon)

	// Build icon parts with spacing - spaces need background too
	spaceStyle := lipgloss.NewStyle().Background(styles.ColorBg)
	leftIconsPart := spaceStyle.Render(" ") + leftIconColumn + spaceStyle.Render(" ") // 14 chars (1 + 12 + 1)
	rightIconsPart := spaceStyle.Render(" ") + rightIconColumn                        // 7 chars (1 + 6)

	// Middle: Status text (fills remaining width)
	statusTextPlain, statusTextStyle := s.renderStatusText()

	// Calculate available width for status text
	// Total width - left icons (14) - right icons (7) = available
	availableWidth := s.width - 14 - 7
	if availableWidth < 0 {
		availableWidth = 0
	}

	// Truncate text if too long (before styling)
	// Need at least 3 chars for "..." truncation
	if availableWidth < 3 {
		statusTextPlain = ""
	} else if len(statusTextPlain) > availableWidth {
		statusTextPlain = statusTextPlain[:availableWidth-3] + "..."
	}

	// Apply style with full width so background fills
	textStyle := statusTextStyle.
		Background(styles.ColorBg).
		Width(availableWidth)
	textPart := textStyle.Render(statusTextPlain)

	bar := leftIconsPart + textPart + rightIconsPart

	// Apply status bar style (border) without width constraint
	// We've already built the content to exact width
	return StatusBarStyle.Render(bar)
}

// renderIcon renders an icon based on animation state.
// When idle (frames=0), shows the baseline/thinnest frame (last in sequence).
// When animating (frames>0), cycles through frames based on currentFrame.
func (s StatusBar) renderIcon(frames int, icons []string) string {
	if frames <= 0 {
		// Idle: return the baseline/thinnest frame (last frame in sequence)
		return icons[len(icons)-1]
	}

	// Animating: cycle through frames
	iconIndex := s.currentFrame % len(icons)
	return icons[iconIndex]
}

// renderStatusText returns the appropriate status text and style.
// Priority: Error > Warning > Autoplay > Default
// Returns plain text and the style to apply
func (s StatusBar) renderStatusText() (string, lipgloss.Style) {
	if s.errorText != "" {
		return s.errorText, StatusTextErrorStyle
	}
	if s.warningText != "" {
		return s.warningText, StatusTextStyle
	}
	if s.autoplayText != "" {
		return "⟳ " + s.autoplayText, StatusTextStyle
	}
	return "All systems operational", StatusTextOKStyle
}

// Icon animation sequences
// Each sequence progresses from full/thick to thin/empty, ending at the baseline frame
var (
	// Autoplay: rotating circle, ends at ◔ (quarter circle)
	autoplayIcons = []string{"●", "◕", "◑", "◔", "○", "◌", "○", "◔", "◑", "◕", "●", "◔"}

	// Info: pulsing dot, ends at ◌ (empty circle)
	infoIcons = []string{"●", "●", "◉", "◉", "◎", "◎", "○", "○", "◌", "◌", "○", "◌"}

	// Warning: pulsing diamond, ends at ◇ (hollow diamond)
	warningIcons = []string{"◆", "◆", "◈", "◈", "◇", "◇", "◈", "◈", "◆", "◆", "◈", "◇"}

	// Error: flashing X, ends at space (empty)
	errorIcons = []string{"✖", "✖", "✖", "✕", "✕", "✕", "✖", "✕", "✕", " ", "✕", " "}

	// LLM: pulsing brain/network, ends at ◌ (empty circle) - cyan/teal color for thinking
	llmIcons = []string{"●", "◉", "◉", "◎", "◎", "○", "○", "◌", "◌", "○", "◌", "◌"}

	// MCP: rotating connection, ends at ○ (circle/dot) - purple color for server communication
	mcpIcons = []string{"◐", "◓", "◑", "◒", "◐", "◓", "◑", "◒", "○", "◌", "○", "○"}
)
