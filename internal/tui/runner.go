package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/constants"
	"github.com/xonecas/mysis/internal/llm"
	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/session"
)

// Runner manages the TUI application lifecycle.
type Runner struct {
	model      *Model
	program    *tea.Program
	sessionMgr *session.Manager
	sessionID  string
	provider   provider.Provider
	proxy      *mcp.Proxy
	tools      []mcp.Tool

	// Autoplay state
	autoplayEnabled bool
	autoplayMessage string
	autoplayCancel  context.CancelFunc
	mu              sync.Mutex

	// History synchronization
	// Protects access to model.conversation.messages from background goroutines
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
) *Runner {
	model := NewModel(ctx)
	model.SetMessages(history)

	r := &Runner{
		model:      &model,
		sessionMgr: sessionMgr,
		sessionID:  sessionID,
		provider:   prov,
		proxy:      proxy,
		tools:      tools,
	}

	// Share history mutex with Model for synchronized access
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

	return r
}

// Run starts the TUI application.
func (r *Runner) Run() error {
	_, err := r.program.Run()
	return err
}

// handleSendMessage sends a message through the LLM loop.
func (r *Runner) handleSendMessage(content string) error {
	// Add user message to history
	userMsg := provider.Message{
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}

	// Add to conversation synchronously BEFORE copying history
	// This ensures the history copy includes all messages up to and including this user message
	// We add it directly here instead of via MessageReceivedMsg to avoid race condition
	// where history is copied before the message event is processed by the TUI Update loop
	r.historyMu.Lock()
	r.model.conversation.AddMessage(userMsg)
	r.historyMu.Unlock()

	// Trigger TUI re-render (message already added above, just need display update)
	r.program.Send(ConversationUpdateMsg{})

	// Save user message
	if err := r.sessionMgr.SaveMessage(r.sessionID, userMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to save user message")
	}

	// Get a safe copy of conversation history before spawning goroutine
	// Now this includes the user message we just added
	historyCopy := r.getConversationHistory()

	// Process turn in background with panic recovery
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().Interface("panic", rec).Msg("Panic in processTurn goroutine")
				r.program.Send(ErrorMsg{Error: fmt.Sprintf("Internal error: %v", rec)})
			}
		}()
		// Use background context for normal messages (no cancellation needed)
		r.processTurn(context.Background(), userMsg, historyCopy)
	}()

	return nil
}

// getConversationHistory returns a safe copy of the conversation messages.
// Protected by mutex to prevent race condition with bubbletea's Update method.
func (r *Runner) getConversationHistory() []provider.Message {
	r.historyMu.Lock()
	defer r.historyMu.Unlock()

	// Make a safe copy of the conversation messages
	// This slice is modified by bubbletea's Update method in the UI goroutine,
	// but we read it from background goroutines (processTurn, autoplay).
	history := make([]provider.Message, len(r.model.conversation.messages))
	copy(history, r.model.conversation.messages)
	return history
}

// processTurn handles LLM processing and tool calls.
func (r *Runner) processTurn(ctx context.Context, userMsg provider.Message, history []provider.Message) {

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

// onMessage is called when a message is added during LLM processing.
func (r *Runner) onMessage(msg provider.Message) {
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

// handleAutoplayCommand handles the /autoplay command.
func (r *Runner) handleAutoplayCommand(cmd string) error {
	parts := strings.Fields(cmd)

	// Check for "stop" subcommand
	if len(parts) >= 2 && parts[1] == "stop" {
		r.mu.Lock()
		if r.autoplayEnabled {
			r.autoplayEnabled = false
			if r.autoplayCancel != nil {
				r.autoplayCancel()
				r.autoplayCancel = nil
			}
			r.mu.Unlock()

			log.Info().Msg("Autoplay stopped")
			// Don't call program.Send() here - causes deadlock when called from Update
			// The AutoplayStoppedMsg will be sent by runAutoplay's defer cleanup
		} else {
			r.mu.Unlock()
			return fmt.Errorf("autoplay not active")
		}
		return nil
	}

	// Start autoplay - need a message
	if len(parts) < 2 {
		return fmt.Errorf("Usage: /autoplay <message> or /autoplay stop")
	}

	message := strings.Join(parts[1:], " ")
	if message == "" {
		return fmt.Errorf("missing message for autoplay")
	}

	// Check if already running
	r.mu.Lock()
	if r.autoplayEnabled {
		r.mu.Unlock()
		return fmt.Errorf("autoplay already running - use '/autoplay stop' first")
	}

	r.autoplayEnabled = true
	r.autoplayMessage = message

	// Create cancelable context for autoplay goroutine
	autoplayCtx, cancel := context.WithCancel(context.Background())
	r.autoplayCancel = cancel
	r.mu.Unlock()

	// Send started message to TUI - use goroutine to avoid deadlock if called from Update
	go r.program.Send(AutoplayStartedMsg{Message: message})

	log.Info().
		Str("message", message).
		Dur("interval", constants.AutoplayInterval).
		Msg("Autoplay started")

	// Start autoplay loop in background
	go r.runAutoplay(autoplayCtx)

	return nil
}

// runAutoplay sends the autoplay message at intervals.
func (r *Runner) runAutoplay(ctx context.Context) {
	log.Debug().Msg("Autoplay goroutine started")

	defer func() {
		// Panic recovery
		if rec := recover(); rec != nil {
			log.Error().Interface("panic", rec).Msg("Panic in autoplay goroutine")
			r.program.Send(ErrorMsg{Error: fmt.Sprintf("Autoplay error: %v", rec)})
		}

		// Normal cleanup
		r.mu.Lock()
		r.autoplayEnabled = false
		r.autoplayCancel = nil
		r.mu.Unlock()
		r.program.Send(AutoplayStoppedMsg{})
		log.Debug().Msg("Autoplay goroutine exiting")
	}()

	// Send first message immediately
	log.Debug().Msg("Sending first autoplay message")
	if err := r.sendAutoplayMessage(ctx); err != nil {
		log.Error().Err(err).Msg("Autoplay failed to send first message")
		return
	}
	log.Debug().Msg("First autoplay message sent successfully")

	// Check if canceled during first message processing
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Then wait and send subsequent messages
	ticker := time.NewTicker(constants.AutoplayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			enabled := r.autoplayEnabled
			r.mu.Unlock()

			if !enabled {
				return
			}

			if err := r.sendAutoplayMessage(ctx); err != nil {
				log.Warn().Err(err).Msg("Autoplay turn failed, stopping")
				return
			}

			// Check if canceled immediately after processing turn
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}

// sendAutoplayMessage sends the autoplay message and processes the turn.
func (r *Runner) sendAutoplayMessage(ctx context.Context) error {
	log.Debug().Msg("sendAutoplayMessage called")

	r.mu.Lock()
	enabled := r.autoplayEnabled
	message := r.autoplayMessage
	r.mu.Unlock()

	log.Debug().Bool("enabled", enabled).Str("message", message).Msg("Autoplay state")

	if !enabled {
		return fmt.Errorf("autoplay disabled")
	}

	// Create user message
	userMsg := provider.Message{
		Role:      "user",
		Content:   message,
		CreatedAt: time.Now(),
	}

	// Add to conversation synchronously BEFORE copying history
	// This ensures the history copy includes the autoplay message
	// Same fix as handleSendMessage to avoid race condition
	r.historyMu.Lock()
	r.model.conversation.AddMessage(userMsg)
	r.historyMu.Unlock()

	// Trigger TUI re-render (message already added above)
	r.program.Send(ConversationUpdateMsg{})

	// Save user message
	if err := r.sessionMgr.SaveMessage(r.sessionID, userMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to save autoplay message")
	}

	// Get safe copy of history before processing
	// Now this includes the autoplay message we just added
	historyCopy := r.getConversationHistory()

	// Process turn (synchronously for autoplay to prevent overlapping turns)
	// Use background context - let the current turn complete even if autoplay is stopped
	// The autoplay loop will check ctx.Done() after this returns
	r.processTurn(context.Background(), userMsg, historyCopy)

	log.Debug().Msg("Autoplay message sent successfully")
	return nil
}
