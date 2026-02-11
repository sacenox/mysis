package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/features"
	"github.com/xonecas/mysis/internal/llm"
	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/session"
	"github.com/xonecas/mysis/internal/styles"
)

// App holds the application state
type App struct {
	provider        provider.Provider
	proxy           *mcp.Proxy
	tools           []mcp.Tool
	history         []provider.Message
	sessionMgr      *session.Manager
	sessionID       string
	autoplayService *features.Service // Autoplay service (display-agnostic)
	mu              sync.Mutex        // Protects history
}

// printWelcome displays the welcome banner.
func printWelcome(provider, model string, toolCount int, sessionInfo string) {
	fmt.Println(styles.Brand.Render("╔══════════════════════════════════════╗"))
	fmt.Println(styles.Brand.Render("║") + "      " + styles.BrandBold.Render("Mysis") + " - SpaceMolt Agent CLI     " + styles.Brand.Render("║"))
	fmt.Println(styles.Brand.Render("╚══════════════════════════════════════╝"))
	fmt.Println()
	fmt.Println(styles.Muted.Render(fmt.Sprintf("Provider: %s (%s)", provider, model)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("Tools: %d available", toolCount)))
	fmt.Println(styles.Muted.Render(sessionInfo))
	fmt.Println()
}

// Start initializes and starts the CLI application (without orchestration).
// This is the main entry point for CLI mode after all initialization is done.
func Start(
	ctx context.Context,
	sessionMgr *session.Manager,
	sessionID string,
	sessionInfo string,
	prov provider.Provider,
	proxy *mcp.Proxy,
	tools []mcp.Tool,
	history []provider.Message,
	autoplayMsg string,
	selectedProvider string,
	selectedModel string,
) error {
	// Nil checks for required dependencies
	if prov == nil {
		return fmt.Errorf("provider cannot be nil")
	}
	if proxy == nil {
		return fmt.Errorf("MCP proxy cannot be nil")
	}
	if sessionMgr == nil {
		return fmt.Errorf("session manager cannot be nil")
	}

	// Input validation
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}
	if sessionInfo == "" {
		return fmt.Errorf("sessionInfo cannot be empty")
	}

	// Print welcome message
	printWelcome(selectedProvider, selectedModel, len(tools), sessionInfo)

	// Start conversation loop
	app := &App{
		provider:   prov,
		proxy:      proxy,
		tools:      tools,
		history:    history,
		sessionMgr: sessionMgr,
		sessionID:  sessionID,
	}

	// Initialize autoplay service
	app.initAutoplayService()

	// Start autoplay if requested
	if autoplayMsg != "" {
		if err := app.startAutoplayFromFlag(ctx, autoplayMsg); err != nil {
			return fmt.Errorf("failed to start autoplay: %w", err)
		}
	}

	return app.runLoop(ctx)
}

// runLoop runs the main conversation loop.
func (app *App) runLoop(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Display prompt
		fmt.Print(styles.Brand.Render("> "))

		// Read user input
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Check for exit commands
		if input == "exit" || input == "quit" {
			fmt.Println(styles.Muted.Render("Goodbye!"))
			break
		}

		// Handle /autoplay commands
		if strings.HasPrefix(input, "/autoplay") {
			if err := app.handleAutoplayCommand(ctx, input); err != nil {
				fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
			}
			continue
		}

		// Add user message to history
		userMsg := provider.Message{
			Role:      "user",
			Content:   input,
			CreatedAt: time.Now(),
		}
		app.history = append(app.history, userMsg)

		// Save user message
		if err := app.sessionMgr.SaveMessage(app.sessionID, userMsg); err != nil {
			log.Warn().Err(err).Msg("Failed to save user message")
		}

		// Process turn (may involve multiple LLM calls if tools are used)
		if err := app.processTurn(ctx); err != nil {
			fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
			continue
		}

		fmt.Println() // Blank line after response
	}

	return scanner.Err()
}

// processTurn handles one conversation turn, which may involve tool calls
func (app *App) processTurn(ctx context.Context) error {
	// Get a snapshot of history for this turn
	app.mu.Lock()
	historyCopy := make([]provider.Message, len(app.history))
	copy(historyCopy, app.history)
	app.mu.Unlock()

	return llm.ProcessTurn(ctx, llm.ProcessTurnOptions{
		Provider:        app.provider,
		Proxy:           app.proxy,
		Tools:           app.tools,
		History:         historyCopy,
		OnMessage:       app.addMessage,
		MaxToolRounds:   20,
		HistoryKeepLast: 10,
	})
}

// addMessage adds a message to history and saves it to the database.
func (app *App) addMessage(msg provider.Message) {
	app.mu.Lock()
	app.history = append(app.history, msg)
	app.mu.Unlock()

	if err := app.sessionMgr.SaveMessage(app.sessionID, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to save message to database")
	}
}

// listSessionsCmd lists recent sessions.
// ListSessionsCmd lists all recent sessions.
func ListSessionsCmd(mgr *session.Manager) error {
	sessions, err := mgr.List(20)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	fmt.Println(styles.Brand.Render("Recent Sessions:"))
	fmt.Println()

	for _, sess := range sessions {
		fmt.Printf("%s  ", styles.Muted.Render(sess.ID[:8]))

		if sess.Name != nil {
			fmt.Print(styles.BrandBold.Render(*sess.Name))
		} else {
			fmt.Print(styles.Muted.Render("(anonymous)"))
		}

		fmt.Printf(" - %s (%s)\n", sess.Provider, sess.Model)

		elapsed := time.Since(sess.LastActiveAt)
		fmt.Printf("       %s\n", styles.Muted.Render(session.FormatDuration(elapsed)+" ago"))
		fmt.Println()
	}

	return nil
}

// deleteSessionCmd deletes a session by name.
// DeleteSessionCmd deletes a session by name.
func DeleteSessionCmd(mgr *session.Manager, name string) error {
	// Get session to show info before deletion
	sess, err := mgr.GetByName(name)
	if err != nil {
		return err
	}
	if sess == nil {
		return fmt.Errorf("session '%s' not found", name)
	}

	// Get message count
	messages, err := mgr.LoadHistory(sess.ID)
	if err != nil {
		return err
	}

	// Confirm deletion
	fmt.Printf("Delete session '%s'?\n", styles.BrandBold.Render(name))
	fmt.Printf("  ID: %s\n", sess.ID[:8])
	fmt.Printf("  Provider: %s (%s)\n", sess.Provider, sess.Model)
	fmt.Printf("  Messages: %d\n", len(messages))
	fmt.Printf("  Created: %s\n", session.FormatDuration(time.Since(sess.CreatedAt))+" ago")
	fmt.Println()

	// Delete
	if err := mgr.DeleteByName(name); err != nil {
		return err
	}

	fmt.Println(styles.Success.Render(fmt.Sprintf("Deleted session '%s'", name)))
	return nil
}

// initializeProviders creates and registers all configured providers
