package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/features"
	"github.com/xonecas/mysis/internal/llm"
	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/session"
)

// Runner manages the TUI application lifecycle.
type Runner struct {
	program         *tea.Program
	sessionMgr      *session.Manager
	sessionID       string
	provider        provider.Provider
	proxy           *mcp.Proxy
	tools           []mcp.Tool
	autoplayService *features.Service // Autoplay service (display-agnostic)

	// Conversation history maintained by runner
	// This is the source of truth for history, separate from the TUI display
	history   []provider.Message
	historyMu sync.Mutex
}

// NewRunner creates a new TUI runner.
func NewRunner(
	ctx context.Context,
	sessionMgr *session.Manager,
	sessionID string,
	prov provider.Provider,
	proxy *mcp.Proxy,
	tools []mcp.Tool,
	history []provider.Message,
) (*Runner, error) {
	// P2: Validate critical dependencies
	if prov == nil {
		return nil, fmt.Errorf("provider cannot be nil")
	}
	if proxy == nil {
		return nil, fmt.Errorf("proxy cannot be nil")
	}

	model := NewModel(ctx)
	model.SetMessages(history)

	r := &Runner{
		sessionMgr: sessionMgr,
		sessionID:  sessionID,
		provider:   prov,
		proxy:      proxy,
		tools:      tools,
		history:    history, // Keep our own copy of history
	}

	// P0: Connect the mutex between Runner and Model
	model.historyMu = &r.historyMu

	// Set up message callback
	model.SetOnSendMessage(r.handleSendMessage)
	model.SetOnCommand(r.handleCommand)

	// Create bubbletea program
	r.program = tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Initialize autoplay service
	r.initAutoplayService()

	return r, nil
}

// Run starts the TUI application.
func (r *Runner) Run() error {
	_, err := r.program.Run()
	return err
}

// Start creates a TUI runner and starts the application.
// This is the main entry point for TUI mode.
func Start(
	ctx context.Context,
	sessionMgr *session.Manager,
	sessionID string,
	prov provider.Provider,
	proxy *mcp.Proxy,
	tools []mcp.Tool,
	history []provider.Message,
) error {
	runner, err := NewRunner(ctx, sessionMgr, sessionID, prov, proxy, tools, history)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}
	return runner.Run()
}

// handleSendMessage sends a message through the LLM loop.
func (r *Runner) handleSendMessage(content string) error {
	// Create user message
	userMsg := provider.Message{
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}

	// Add to our history and get a copy for processing
	r.historyMu.Lock()
	r.history = append(r.history, userMsg)
	historyCopy := make([]provider.Message, len(r.history))
	copy(historyCopy, r.history)
	r.historyMu.Unlock()

	// Send user message to TUI for display
	r.program.Send(MessageReceivedMsg{Message: userMsg})

	// Save user message
	if err := r.sessionMgr.SaveMessage(r.sessionID, userMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to save user message")
	}

	// Process turn in background with panic recovery
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().Interface("panic", rec).Msg("Panic in processTurn goroutine")
				r.program.Send(ErrorMsg{Error: fmt.Sprintf("Internal error: %v", rec)})
			}
		}()
		// Use background context for normal messages (no cancellation needed)
		r.processTurn(context.Background(), historyCopy)
	}()

	return nil
}

// processTurn handles LLM processing and tool calls.
func (r *Runner) processTurn(ctx context.Context, history []provider.Message) {

	// User message is already in history (added synchronously in handleSendMessage)
	// No need to append it again

	// Notify TUI of LLM activity
	r.program.Send(LLMActivityMsg{})

	// Process turn
	err := llm.ProcessTurn(ctx, llm.ProcessTurnOptions{
		Provider:        r.provider,
		Proxy:           r.proxy,
		Tools:           r.tools,
		History:         history,
		OnMessage:       r.onMessage,
		OnToolCall:      r.onToolCall,
		MaxToolRounds:   20,
		HistoryKeepLast: 10,
		SuppressOutput:  true, // Suppress stdout in TUI mode
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to process turn")
		r.program.Send(ErrorMsg{Error: err.Error()})
	}
}

// trimHistory trims the history to keep only the last 100 messages.
// P1: Prevents unbounded memory growth.
// Must be called with historyMu held.
func (r *Runner) trimHistory() {
	const maxHistorySize = 100
	if len(r.history) > maxHistorySize {
		// Keep the last 100 messages
		r.history = r.history[len(r.history)-maxHistorySize:]
		log.Debug().Int("trimmed_to", maxHistorySize).Msg("Trimmed history to prevent unbounded growth")
	}
}

// onMessage is called when a message is added during LLM processing.
func (r *Runner) onMessage(msg provider.Message) {
	// Add to our history
	r.historyMu.Lock()
	r.history = append(r.history, msg)
	r.trimHistory()
	r.historyMu.Unlock()

	// Send to TUI for display
	r.program.Send(MessageReceivedMsg{Message: msg})

	// Save to database
	if err := r.sessionMgr.SaveMessage(r.sessionID, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to save message")
	}
}

// onToolCall is called when tool calls are about to be executed.
func (r *Runner) onToolCall() {
	// Notify TUI of MCP activity
	r.program.Send(MCPActivityMsg{})
}

// handleCommand handles slash commands.
func (r *Runner) handleCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "/autoplay":
		return r.handleAutoplayCommand(cmd)
	default:
		log.Info().Str("command", cmd).Msg("Unknown command")
	}

	return nil
}

// SendMessage sends a message to the TUI (for external use).
func (r *Runner) SendMessage(msg provider.Message) {
	r.program.Send(MessageReceivedMsg{Message: msg})
}

// SendError sends an error to the TUI (for external use).
func (r *Runner) SendError(err string) {
	r.program.Send(ErrorMsg{Error: err})
}

// NotifyLLMActivity notifies the TUI of LLM activity.
func (r *Runner) NotifyLLMActivity() {
	r.program.Send(LLMActivityMsg{})
}

// NotifyMCPActivity notifies the TUI of MCP activity.
func (r *Runner) NotifyMCPActivity() {
	r.program.Send(MCPActivityMsg{})
}

// Stop stops the TUI application.
func (r *Runner) Stop() {
	if r.program != nil {
		r.program.Send(tea.Quit())
	}
}

// initAutoplayService initializes the autoplay service with TUI-specific callbacks.
// This should be called once when creating the Runner.
func (r *Runner) initAutoplayService() {
	r.autoplayService = features.NewAutoplayService(features.AutoplayCallbacks{
		OnStarted: func(message string, interval time.Duration) {
			// Send started message to TUI - use goroutine to avoid deadlock if called from Update
			go r.program.Send(AutoplayStartedMsg{Message: message})
		},
		OnStopped: func() {
			r.program.Send(AutoplayStoppedMsg{})
		},
		OnTurn: func(ctx context.Context, message string) error {
			// Create user message
			userMsg := provider.Message{
				Role:      "user",
				Content:   message,
				CreatedAt: time.Now(),
			}

			// Add to our history
			r.historyMu.Lock()
			r.history = append(r.history, userMsg)
			historyCopy := make([]provider.Message, len(r.history))
			copy(historyCopy, r.history)
			r.historyMu.Unlock()

			// Send message to TUI for display
			r.program.Send(MessageReceivedMsg{Message: userMsg})

			// Save user message
			if err := r.sessionMgr.SaveMessage(r.sessionID, userMsg); err != nil {
				log.Warn().Err(err).Msg("Failed to save autoplay message")
			}

			// Process turn (synchronously for autoplay to prevent overlapping turns)
			// Use background context - let the current turn complete even if autoplay is stopped
			// The autoplay loop will check ctx.Done() after this returns
			r.processTurn(context.Background(), historyCopy)

			return nil
		},
		OnError: func(err error) {
			log.Error().Err(err).Msg("Autoplay error")
			r.program.Send(ErrorMsg{Error: err.Error()})
		},
	})
}

// handleAutoplayCommand handles the /autoplay command.
func (r *Runner) handleAutoplayCommand(cmd string) error {
	parts := strings.Fields(cmd)

	// Check for "stop" subcommand
	if len(parts) >= 2 && parts[1] == "stop" {
		if err := r.autoplayService.Stop(); err != nil {
			return err
		}
		log.Info().Msg("Autoplay stopped")
		// The AutoplayStoppedMsg will be sent by the service's OnStopped callback
		return nil
	}

	// Start autoplay - need a message
	if len(parts) < 2 {
		return fmt.Errorf("usage: /autoplay <message> or /autoplay stop")
	}

	message := strings.Join(parts[1:], " ")
	if message == "" {
		return fmt.Errorf("missing message for autoplay")
	}

	// Start autoplay
	if err := r.autoplayService.Start(context.Background(), message); err != nil {
		return fmt.Errorf("%s - use '/autoplay stop' first", err.Error())
	}

	return nil
}
