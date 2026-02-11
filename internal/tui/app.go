// Package tui provides the terminal user interface for Mysis.
package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/styles"
)

// Model is the main TUI model.
type Model struct {
	conversation Conversation
	input        Input
	statusBar    StatusBar

	width  int
	height int

	// State
	autoplayActive  bool
	autoplayMessage string
	lastError       string

	// Callback to send messages
	onSendMessage func(string) error

	// Callback to execute commands
	onCommand func(string) error

	// Synchronization for conversation history access
	// Shared with Runner to protect concurrent access from background goroutines
	historyMu *sync.Mutex

	ready bool
	ctx   context.Context
}

// NewModel creates a new TUI model.
func NewModel(ctx context.Context) Model {
	return Model{
		ctx:          ctx,
		conversation: NewConversation(80, 20),
		input:        NewInput(80),
		statusBar:    NewStatusBar(80),
	}
}

// SetOnSendMessage sets the callback for sending messages.
func (m *Model) SetOnSendMessage(fn func(string) error) {
	m.onSendMessage = fn
}

// SetOnCommand sets the callback for executing commands.
func (m *Model) SetOnCommand(fn func(string) error) {
	m.onCommand = fn
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.statusBar.Init(),
		m.input.Focus(),
	)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate component heights
		// Layout: Conversation (fills) + Input (3 lines) + Status (2 lines)
		inputHeight := 3
		statusHeight := 1
		conversationHeight := m.height - inputHeight - statusHeight
		if conversationHeight < 5 {
			conversationHeight = 5
		}

		m.conversation.SetSize(m.width, conversationHeight)
		m.input.SetWidth(m.width)
		m.statusBar.SetWidth(m.width)

		m.ready = true

	case tea.KeyMsg:
		// Global keys
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Escape):
			// ESC stops autoplay if active
			if m.autoplayActive {
				// Stop autoplay in backend
				if m.onCommand != nil {
					_ = m.onCommand("/autoplay stop")
				}
				// Update local state
				m.autoplayActive = false
				m.statusBar.ClearAutoplayText()
			}
			return m, nil

		case key.Matches(msg, keys.Enter):
			// Send message or execute command
			value := strings.TrimSpace(m.input.Value())
			if value != "" {
				m.input.AddToHistory(value)
				m.input.Reset()

				// Check if it's a command
				if strings.HasPrefix(value, "/") {
					return m, m.executeCommand(value)
				}

				// Regular message
				return m, m.sendMessage(value)
			}
			return m, nil
		}

		// Pass to input for editing
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)

	case tea.MouseMsg:
		// Pass mouse events to conversation for scrolling
		var updated bool
		m.conversation, updated = m.conversation.Update(msg)
		if updated {
			return m, nil
		}

	case StatusBarTickMsg:
		// Update status bar animation
		var cmd tea.Cmd
		m.statusBar, cmd = m.statusBar.Update(msg)
		cmds = append(cmds, cmd)

	case MessageReceivedMsg:
		// Add message to conversation (protected by mutex for concurrent access)
		m.historyMu.Lock()
		m.conversation.AddMessage(msg.Message)
		m.historyMu.Unlock()
		cmds = append(cmds, m.statusBar.AnimateInfo())
		m.statusBar.ClearError()

	case ConversationUpdateMsg:
		// Trigger re-render of conversation (messages already added elsewhere)
		m.historyMu.Lock()
		m.conversation.updateContent()
		m.historyMu.Unlock()
		cmds = append(cmds, m.statusBar.AnimateInfo())
		m.statusBar.ClearError()

	case ErrorMsg:
		// Show error in status bar
		m.lastError = msg.Error
		cmds = append(cmds, m.statusBar.SetError(truncate(msg.Error, 100)))
		m.statusBar.ClearWarning() // Error takes priority

	case WarningMsg:
		// Show warning in status bar
		cmds = append(cmds, m.statusBar.SetWarning(truncate(msg.Warning, 100)))

	case AutoplayStartedMsg:
		m.autoplayActive = true
		m.autoplayMessage = msg.Message
		cmds = append(cmds, m.statusBar.SetAutoplayText(truncate(msg.Message, 50)))

	case AutoplayStoppedMsg:
		m.autoplayActive = false
		m.statusBar.ClearAutoplayText()

	case LLMActivityMsg:
		// Animate LLM connection icon
		cmds = append(cmds, m.statusBar.AnimateLLM())

	case MCPActivityMsg:
		// Animate MCP connection icon
		cmds = append(cmds, m.statusBar.AnimateMCP())
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Check minimum terminal size
	const minWidth = 80
	const minHeight = 20
	if m.width < minWidth || m.height < minHeight {
		warning := fmt.Sprintf(
			"Terminal too small!\n\nMinimum: %dx%d\nCurrent: %dx%d\n\nPlease resize.",
			minWidth, minHeight, m.width, m.height,
		)
		return ErrorMsg{Error: warning}.Error
	}

	// Build UI content first
	conversation := m.conversation.View()
	input := m.input.View()
	status := m.statusBar.View()

	// Join all sections
	content := conversation + "\n" + input + "\n" + status

	// Apply base style with background to fill entire terminal
	// This is the recommended way per bubbletea docs
	baseStyle := lipgloss.NewStyle().
		Background(styles.ColorBg).
		Width(m.width).
		Height(m.height)

	return baseStyle.Render(content)
}

// sendMessage sends a message through the callback.
func (m Model) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		if m.onSendMessage == nil {
			return ErrorMsg{Error: "no message handler configured"}
		}

		if err := m.onSendMessage(content); err != nil {
			log.Error().Err(err).Msg("Failed to send message")
			return ErrorMsg{Error: err.Error()}
		}

		return nil
	}
}

// executeCommand executes a slash command.
func (m Model) executeCommand(cmd string) tea.Cmd {
	return func() tea.Msg {
		// Handle built-in commands
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			return nil
		}

		switch parts[0] {
		case "/autoplay":
			// Pass to runner for backend execution
			if m.onCommand != nil {
				if err := m.onCommand(cmd); err != nil {
					return ErrorMsg{Error: err.Error()}
				}
			}
			return nil

		case "/exit", "/quit":
			return tea.Quit()

		default:
			// Pass to external command handler
			if m.onCommand != nil {
				if err := m.onCommand(cmd); err != nil {
					return ErrorMsg{Error: err.Error()}
				}
			}
		}

		return nil
	}
}

// AddMessage adds a message to the conversation (called from external code).
func (m *Model) AddMessage(msg provider.Message) {
	m.conversation.AddMessage(msg)
}

// SetMessages sets all conversation messages.
func (m *Model) SetMessages(messages []provider.Message) {
	m.conversation.SetMessages(messages)
}

// Key bindings
var keys = struct {
	Quit   key.Binding
	Escape key.Binding
	Enter  key.Binding
}{
	Quit:   key.NewBinding(key.WithKeys("ctrl+c")),
	Escape: key.NewBinding(key.WithKeys("esc")),
	Enter:  key.NewBinding(key.WithKeys("enter")),
}

// Message types for external communication
type (
	// MessageReceivedMsg is sent when a new message is received.
	MessageReceivedMsg struct {
		Message provider.Message
	}

	// ConversationUpdateMsg triggers a re-render without adding messages (already added).
	ConversationUpdateMsg struct{}

	// ErrorMsg is sent when an error occurs.
	ErrorMsg struct {
		Error string
	}

	// WarningMsg is sent when a warning occurs.
	WarningMsg struct {
		Warning string
	}

	// AutoplayStartedMsg is sent when autoplay starts.
	AutoplayStartedMsg struct {
		Message string
	}

	// AutoplayStoppedMsg is sent when autoplay stops.
	AutoplayStoppedMsg struct{}

	// LLMActivityMsg is sent when LLM activity occurs.
	LLMActivityMsg struct{}

	// MCPActivityMsg is sent when MCP activity occurs.
	MCPActivityMsg struct{}
)

// Helper functions

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
