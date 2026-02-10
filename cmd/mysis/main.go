package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/config"
	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/store"
	"github.com/xonecas/mysis/internal/styles"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	// Parse flags
	var (
		showVersion  = flag.Bool("version", false, "Show version and exit")
		configPath   = flag.String("config", "", "Path to config file")
		debug        = flag.Bool("debug", false, "Enable debug logging")
		providerName = flag.String("p", "", "Provider name (overrides default from config)")
		sessionName  = flag.String("session", "", "Session name (resume or create named session)")
		listSessions = flag.Bool("list-sessions", false, "List recent sessions and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Mysis %s\n", Version)
		os.Exit(0)
	}

	// Initialize logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Find config file
	if *configPath == "" {
		// Try current directory first, then ~/.config/mysis/
		if _, err := os.Stat("config.toml"); err == nil {
			*configPath = "config.toml"
		} else {
			dataDir, err := config.DataDir()
			if err == nil {
				*configPath = dataDir + "/config.toml"
			}
		}
	}

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: config file not found"))
		fmt.Fprintln(os.Stderr, "Tried: ./config.toml and ~/.config/mysis/config.toml")
		os.Exit(1)
	}

	log.Info().
		Str("version", Version).
		Str("config", *configPath).
		Msg("Starting Mysis")

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to load config: "+err.Error()))
		os.Exit(1)
	}

	// Open database
	db, err := store.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to open database: "+err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// Handle --list-sessions flag
	if *listSessions {
		if err := listSessionsCmd(db); err != nil {
			fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
			os.Exit(1)
		}
		return
	}

	// Load credentials
	creds, err := config.LoadCredentials()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load credentials, using empty credentials")
		creds = &config.Credentials{}
	}

	// Initialize provider registry
	registry := initializeProviders(cfg, creds)

	// Select provider
	selectedProvider := *providerName
	if selectedProvider == "" {
		// Use first provider from config
		for name := range cfg.Providers {
			selectedProvider = name
			break
		}
	}

	if selectedProvider == "" {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: no provider configured"))
		os.Exit(1)
	}

	providerCfg, ok := cfg.Providers[selectedProvider]
	if !ok {
		fmt.Fprintln(os.Stderr, styles.Error.Render(fmt.Sprintf("Error: provider '%s' not found in config", selectedProvider)))
		os.Exit(1)
	}

	prov, err := registry.Create(selectedProvider, providerCfg.Model, providerCfg.Temperature)
	if err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to create provider: "+err.Error()))
		os.Exit(1)
	}
	defer prov.Close()

	log.Info().
		Str("provider", selectedProvider).
		Str("model", providerCfg.Model).
		Msg("Provider initialized")

	// Initialize MCP client
	ctx := context.Background()
	mcpClient := mcp.NewClient(cfg.MCP.Upstream)
	proxy := mcp.NewProxy(mcpClient)

	if err := proxy.Initialize(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to initialize MCP - continuing without game tools")
	} else {
		log.Info().Str("upstream", cfg.MCP.Upstream).Msg("MCP proxy initialized")
	}
	defer proxy.Close()

	// Get available tools
	tools, err := proxy.ListTools(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list tools - continuing without tools")
		tools = []mcp.Tool{}
	} else {
		log.Info().Int("count", len(tools)).Msg("Tools available")
	}

	// Initialize or resume session
	var sessionID string
	var sessionInfo string
	if *sessionName != "" {
		// Try to resume by name
		sess, err := db.GetSessionByName(*sessionName)
		if err != nil {
			fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to load session: "+err.Error()))
			os.Exit(1)
		}
		if sess != nil {
			sessionID = sess.ID
			sessionInfo = fmt.Sprintf("Resumed session: %s", *sessionName)
			log.Info().Str("session_id", sessionID).Str("name", *sessionName).Msg("Resumed session")
		} else {
			// Create new named session
			sessionID = uuid.New().String()
			if err := db.CreateSession(sessionID, selectedProvider, providerCfg.Model, sessionName); err != nil {
				fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to create session: "+err.Error()))
				os.Exit(1)
			}
			sessionInfo = fmt.Sprintf("New session: %s", *sessionName)
			log.Info().Str("session_id", sessionID).Str("name", *sessionName).Msg("Created named session")
		}
	} else {
		// Create anonymous session
		sessionID = uuid.New().String()
		if err := db.CreateSession(sessionID, selectedProvider, providerCfg.Model, nil); err != nil {
			fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to create session: "+err.Error()))
			os.Exit(1)
		}
		sessionInfo = fmt.Sprintf("Session: %s", sessionID[:8])
		log.Info().Str("session_id", sessionID).Msg("Created anonymous session")
	}

	// Load message history
	history, err := db.LoadMessages(sessionID)
	if err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to load history: "+err.Error()))
		os.Exit(1)
	}
	if len(history) > 0 {
		log.Info().Int("count", len(history)).Msg("Loaded message history")
	}

	// Print welcome message
	fmt.Println(styles.Brand.Render("╔══════════════════════════════════════╗"))
	fmt.Println(styles.Brand.Render("║") + "  " + styles.BrandBold.Render("Mysis") + " - SpaceMolt Agent CLI         " + styles.Brand.Render("║"))
	fmt.Println(styles.Brand.Render("╚══════════════════════════════════════╝"))
	fmt.Println()
	fmt.Println(styles.Muted.Render(fmt.Sprintf("Provider: %s (%s)", selectedProvider, providerCfg.Model)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("Tools: %d available", len(tools))))
	fmt.Println(styles.Muted.Render(sessionInfo))
	fmt.Println()

	// Start conversation loop
	app := &App{
		provider:  prov,
		proxy:     proxy,
		tools:     tools,
		history:   history,
		db:        db,
		sessionID: sessionID,
	}

	if err := app.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}

// App holds the application state
type App struct {
	provider        provider.Provider
	proxy           *mcp.Proxy
	tools           []mcp.Tool
	history         []provider.Message
	db              *store.Store
	sessionID       string
	autoplayEnabled bool
	autoplayMessage string
	autoplayCancel  context.CancelFunc
	mu              sync.Mutex // Protects autoplay state and history
}

// Run starts the conversation loop
func (app *App) Run(ctx context.Context) error {
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
			Role:    "user",
			Content: input,
		}
		app.history = append(app.history, userMsg)

		// Save user message
		if err := app.db.SaveMessage(app.sessionID, userMsg); err != nil {
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
	maxToolRounds := 20 // Prevent infinite loops

	for round := 0; round < maxToolRounds; round++ {
		// Get a snapshot of history for this round
		app.mu.Lock()
		historyCopy := make([]provider.Message, len(app.history))
		copy(historyCopy, app.history)
		app.mu.Unlock()

		// Compress history before sending to LLM
		// Keep last 10 turns full, compress older state queries
		compressedHistory := store.CompressHistory(historyCopy, 10)

		// Log compression stats
		if len(compressedHistory) < len(historyCopy) {
			originalTokens := store.EstimateTokenCount(historyCopy)
			compressedTokens := store.EstimateTokenCount(compressedHistory)
			log.Debug().
				Int("original_msgs", len(historyCopy)).
				Int("compressed_msgs", len(compressedHistory)).
				Int("original_tokens", originalTokens).
				Int("compressed_tokens", compressedTokens).
				Int("saved_tokens", originalTokens-compressedTokens).
				Msg("History compressed")
		}

		// Convert MCP tools to provider format
		providerTools := make([]provider.Tool, len(app.tools))
		for i, t := range app.tools {
			providerTools[i] = provider.Tool{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			}
		}

		// Call LLM with compressed history
		resp, err := app.provider.ChatWithTools(ctx, compressedHistory, providerTools)
		if err != nil {
			return fmt.Errorf("LLM call failed: %w", err)
		}

		// Display reasoning if present
		if resp.Reasoning != "" {
			// Trim excessive whitespace and collapse multiple spaces/newlines
			reasoning := strings.TrimSpace(resp.Reasoning)
			reasoning = strings.Join(strings.Fields(reasoning), " ")

			// Truncate if too long
			if len(reasoning) > 200 {
				reasoning = reasoning[:197] + "..."
			}

			fmt.Println(styles.Muted.Render("∴ " + reasoning))
		}

		// If no tool calls, display text response and we're done
		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" {
				fmt.Println(resp.Content)
			}

			// Add assistant response to history
			app.addMessage(provider.Message{
				Role:    "assistant",
				Content: resp.Content,
			})

			return nil
		}

		// Tool calls present - add assistant message with tool calls to history
		app.addMessage(provider.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call
		for _, toolCall := range resp.ToolCalls {
			fmt.Print(styles.Secondary.Render(fmt.Sprintf("⚙ %s", toolCall.Name)))

			// Show arguments (truncated if long)
			var args map[string]interface{}
			if err := json.Unmarshal(toolCall.Arguments, &args); err == nil {
				argsStr, _ := json.Marshal(args)
				if len(argsStr) > 60 {
					argsStr = argsStr[:57]
					argsStr = append(argsStr, '.', '.', '.')
				}
				fmt.Print(styles.Muted.Render(string(argsStr)))
			}

			// Execute tool via MCP proxy
			result, err := app.proxy.CallTool(ctx, toolCall.Name, toolCall.Arguments)

			if err != nil {
				fmt.Println(styles.Error.Render(" ✗"))
				fmt.Println(styles.Error.Render("  Error: " + err.Error()))

				// Add error result to history
				app.addMessage(provider.Message{
					Role:       "tool",
					Content:    fmt.Sprintf("Error: %v", err),
					ToolCallID: toolCall.ID,
				})
				continue
			}

			// Check if result is an error
			if result.IsError {
				fmt.Println(styles.Error.Render(" ✗"))
				// Extract error text from content blocks
				var errText string
				for _, block := range result.Content {
					if block.Type == "text" {
						errText += block.Text
					}
				}
				if errText != "" {
					fmt.Println(styles.Error.Render("  " + errText))
				}

				// Add error result to history
				app.addMessage(provider.Message{
					Role:       "tool",
					Content:    errText,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			// Success
			fmt.Println(styles.Success.Render(" ✓"))

			// Extract result content
			var resultText string
			for _, block := range result.Content {
				if block.Type == "text" {
					resultText += block.Text
				}
			}

			// Show result preview (truncated)
			if len(resultText) > 100 {
				preview := resultText[:97] + "..."
				fmt.Println(styles.Muted.Render("  " + preview))
			} else if resultText != "" {
				fmt.Println(styles.Muted.Render("  " + resultText))
			}

			// Add tool result to history
			app.addMessage(provider.Message{
				Role:       "tool",
				Content:    resultText,
				ToolCallID: toolCall.ID,
			})
		}

		// Continue loop to let LLM process tool results
	}

	return fmt.Errorf("too many tool call rounds (limit: %d)", maxToolRounds)
}

// addMessage adds a message to history and saves it to the database.
func (app *App) addMessage(msg provider.Message) {
	app.mu.Lock()
	app.history = append(app.history, msg)
	app.mu.Unlock()

	if err := app.db.SaveMessage(app.sessionID, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to save message to database")
	}
}

// listSessionsCmd lists recent sessions.
func listSessionsCmd(db *store.Store) error {
	sessions, err := db.ListSessions(20)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
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
		fmt.Printf("       %s\n", styles.Muted.Render(formatDuration(elapsed)+" ago"))
		fmt.Println()
	}

	return nil
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// initializeProviders creates and registers all configured providers
func initializeProviders(cfg *config.Config, creds *config.Credentials) *provider.Registry {
	registry := provider.NewRegistry()

	for name, provCfg := range cfg.Providers {
		// Detect provider type based on endpoint
		if strings.Contains(provCfg.Endpoint, "localhost:11434") || strings.Contains(provCfg.Endpoint, "/ollama") {
			// Ollama provider
			factory := provider.NewOllamaFactory(name, provCfg.Endpoint)
			registry.RegisterFactory(name, factory)
			log.Debug().Str("name", name).Str("endpoint", provCfg.Endpoint).Msg("Registered Ollama provider")
		} else if strings.Contains(provCfg.Endpoint, "opencode.ai") {
			// OpenCode Zen provider
			keyName := provCfg.APIKeyName
			if keyName == "" {
				keyName = name
			}
			apiKey := creds.GetAPIKey(keyName)
			if apiKey == "" {
				log.Warn().Str("name", name).Str("key_name", keyName).Msg("No API key found for provider")
				continue
			}
			factory := provider.NewOpenCodeFactory(name, provCfg.Endpoint, apiKey)
			registry.RegisterFactory(name, factory)
			log.Debug().Str("name", name).Str("endpoint", provCfg.Endpoint).Msg("Registered OpenCode provider")
		} else {
			log.Warn().Str("name", name).Str("endpoint", provCfg.Endpoint).Msg("Unknown provider type")
		}
	}

	return registry
}

// handleAutoplayCommand handles /autoplay commands
func (app *App) handleAutoplayCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)

	if len(parts) == 1 {
		// Just "/autoplay" - show status
		app.mu.Lock()
		enabled := app.autoplayEnabled
		message := app.autoplayMessage
		app.mu.Unlock()

		if enabled {
			fmt.Println(styles.Secondary.Render(fmt.Sprintf("Autoplay active: \"%s\"", message)))
		} else {
			fmt.Println(styles.Muted.Render("Autoplay not active"))
			fmt.Println(styles.Muted.Render("Usage: /autoplay <message>"))
		}
		return nil
	}

	// Check for "stop" subcommand
	if parts[1] == "stop" {
		app.mu.Lock()
		if app.autoplayEnabled {
			app.autoplayEnabled = false
			if app.autoplayCancel != nil {
				app.autoplayCancel()
				app.autoplayCancel = nil
			}
			app.mu.Unlock()
			fmt.Println(styles.Success.Render("Autoplay stopped"))
		} else {
			app.mu.Unlock()
			fmt.Println(styles.Muted.Render("Autoplay not active"))
		}
		return nil
	}

	// Join all parts after /autoplay as the message
	message := strings.Join(parts[1:], " ")

	if message == "" {
		return fmt.Errorf("missing message for autoplay")
	}

	// Start autoplay
	app.mu.Lock()
	if app.autoplayEnabled {
		app.mu.Unlock()
		return fmt.Errorf("autoplay already running - use '/autoplay stop' first")
	}

	app.autoplayEnabled = true
	app.autoplayMessage = message

	// Create cancelable context for autoplay goroutine
	autoplayCtx, cancel := context.WithCancel(ctx)
	app.autoplayCancel = cancel
	app.mu.Unlock()

	fmt.Println(styles.Secondary.Render(fmt.Sprintf("Autoplay started: \"%s\"", message)))
	fmt.Println(styles.Muted.Render("Interval: 200s (20 tool rounds × 10s/tick)"))
	fmt.Println(styles.Muted.Render("Type '/autoplay stop' to stop"))

	// Start autoplay loop in background
	go app.runAutoplay(autoplayCtx)

	return nil
}

// runAutoplay sends the autoplay message every 200 seconds (20 tool rounds × 10s/tick)
func (app *App) runAutoplay(ctx context.Context) {
	log.Debug().Msg("Autoplay goroutine started")

	// maxToolRounds (20) × game tick time (10s) = 200 seconds
	const autoplayInterval = 200 * time.Second

	defer func() {
		app.mu.Lock()
		app.autoplayEnabled = false
		app.autoplayCancel = nil
		app.mu.Unlock()
		fmt.Println(styles.Muted.Render("Autoplay stopped"))
		log.Debug().Msg("Autoplay goroutine exiting")
	}()

	// Send first message immediately
	log.Debug().Msg("Sending first autoplay message")
	if err := app.sendAutoplayMessage(ctx); err != nil {
		log.Error().Err(err).Msg("Autoplay failed to send first message")
		return
	}
	log.Debug().Msg("First autoplay message sent successfully")

	// Then wait and send subsequent messages
	ticker := time.NewTicker(autoplayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			app.mu.Lock()
			enabled := app.autoplayEnabled
			app.mu.Unlock()

			if !enabled {
				return
			}

			if err := app.sendAutoplayMessage(ctx); err != nil {
				return
			}
		}
	}
}

// sendAutoplayMessage sends the autoplay message and processes the turn
func (app *App) sendAutoplayMessage(ctx context.Context) error {
	log.Debug().Msg("sendAutoplayMessage called")

	app.mu.Lock()
	enabled := app.autoplayEnabled
	message := app.autoplayMessage
	app.mu.Unlock()

	log.Debug().Bool("enabled", enabled).Str("message", message).Msg("Autoplay state")

	if !enabled {
		return fmt.Errorf("autoplay disabled")
	}

	fmt.Println(styles.Muted.Render("─── Autoplay Turn ───"))
	fmt.Println(styles.Brand.Render("> ") + message)
	log.Debug().Msg("About to process turn")

	// Send autoplay message
	userMsg := provider.Message{
		Role:    "user",
		Content: message,
	}

	app.mu.Lock()
	app.history = append(app.history, userMsg)
	app.mu.Unlock()

	if err := app.db.SaveMessage(app.sessionID, userMsg); err != nil {
		log.Warn().Err(err).Msg("Failed to save autoplay message")
	}

	// Process turn
	if err := app.processTurn(ctx); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
		// Don't stop autoplay on errors - just log and continue
		log.Warn().Err(err).Msg("Autoplay turn failed, continuing...")
	}

	fmt.Println() // Blank line after response
	log.Debug().Msg("Autoplay message sent successfully")
	return nil
}
