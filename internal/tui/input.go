package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/mysis/internal/styles"
)

const maxHistorySize = 100

// Input handles text input with history navigation.
type Input struct {
	textInput    textinput.Model
	history      []string // Previous messages
	historyIndex int      // Current position in history (-1 = not browsing)
	draft        string   // Saved draft when browsing history
	width        int
}

// NewInput creates a new input component.
func NewInput(width int) Input {
	ti := textinput.New()
	ti.Placeholder = "Type message or command..."
	ti.Prompt = "> "
	ti.CharLimit = 2000
	ti.Width = width - 6 // Account for: border (2) + padding (2) + prompt (2)
	ti.Focus()

	// Set text input colors to match our theme
	ti.PromptStyle = InputPromptStyle
	ti.TextStyle = InputTextStyle
	ti.PlaceholderStyle = InputPlaceholderStyle

	return Input{
		textInput:    ti,
		history:      make([]string, 0, maxHistorySize),
		historyIndex: -1,
		width:        width,
	}
}

// SetWidth updates the input width.
func (i *Input) SetWidth(width int) {
	i.width = width
	// Account for: border (2) + padding (2) + prompt (2) = 6 chars
	i.textInput.Width = width - 6
}

// Focus focuses the input.
func (i *Input) Focus() tea.Cmd {
	return i.textInput.Focus()
}

// Blur blurs the input.
func (i *Input) Blur() {
	i.textInput.Blur()
}

// Value returns the current input value.
func (i Input) Value() string {
	return i.textInput.Value()
}

// SetValue sets the input value.
func (i *Input) SetValue(value string) {
	i.textInput.SetValue(value)
}

// Reset clears the input.
func (i *Input) Reset() {
	i.textInput.Reset()
	i.historyIndex = -1
	i.draft = ""
}

// AddToHistory adds a message to the history.
func (i *Input) AddToHistory(message string) {
	if message == "" {
		return
	}

	// Avoid duplicate consecutive entries
	if len(i.history) > 0 && i.history[len(i.history)-1] == message {
		return
	}

	i.history = append(i.history, message)

	// Trim if too large
	if len(i.history) > maxHistorySize {
		i.history = i.history[len(i.history)-maxHistorySize:]
	}
}

// History key bindings
var historyKeys = struct {
	Up   key.Binding
	Down key.Binding
}{
	Up:   key.NewBinding(key.WithKeys("up")),
	Down: key.NewBinding(key.WithKeys("down")),
}

// Update handles input updates.
func (i Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	// Handle history navigation
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, historyKeys.Up):
			i.navigateHistory(1) // Go back in history
			return i, nil
		case key.Matches(keyMsg, historyKeys.Down):
			i.navigateHistory(-1) // Go forward in history
			return i, nil
		}
	}

	var cmd tea.Cmd
	i.textInput, cmd = i.textInput.Update(msg)
	return i, cmd
}

// navigateHistory moves through the history.
// direction: 1 = older (up), -1 = newer (down)
func (i *Input) navigateHistory(direction int) {
	if len(i.history) == 0 {
		return
	}

	// Save current input as draft when starting to browse
	if i.historyIndex == -1 && direction == 1 {
		i.draft = i.textInput.Value()
	}

	newIndex := i.historyIndex + direction

	// Clamp to valid range
	if newIndex < -1 {
		newIndex = -1
	}
	if newIndex >= len(i.history) {
		newIndex = len(i.history) - 1
	}

	i.historyIndex = newIndex

	// Update input value
	if i.historyIndex == -1 {
		// Back to draft
		i.textInput.SetValue(i.draft)
		i.textInput.CursorEnd()
	} else {
		// Show history item (most recent is at end of slice)
		historyIdx := len(i.history) - 1 - i.historyIndex
		i.textInput.SetValue(i.history[historyIdx])
		i.textInput.CursorEnd()
	}
}

// View renders the input.
func (i Input) View() string {
	// Check if input is empty - render custom placeholder with background
	// The textinput component's placeholder doesn't respect our background color
	if i.textInput.Value() == "" {
		// Render prompt
		prompt := InputPromptStyle.Render(i.textInput.Prompt)

		// Render placeholder with remaining width
		placeholderStyle := lipgloss.NewStyle().
			Background(styles.ColorBg).
			Foreground(styles.ColorMuted)

		placeholder := prompt + placeholderStyle.Render(i.textInput.Placeholder)
		return InputBorderStyle.Width(i.width).Render(placeholder)
	}

	// The textinput renders its own content when focused or has text
	content := i.textInput.View()

	// Wrap content with background style at full width
	bgStyle := lipgloss.NewStyle().
		Background(styles.ColorBg).
		Width(i.width - 2)

	wrappedContent := bgStyle.Render(content)

	// Apply border (top only) with full width
	return InputBorderStyle.Width(i.width).Render(wrappedContent)
}
