package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/config"
	"github.com/xonecas/mysis/internal/llm"
	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/session"
	"github.com/xonecas/mysis/internal/store"
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
	autoplayEnabled bool
	autoplayMessage string
	autoplayCancel  context.CancelFunc
	mu              sync.Mutex // Protects autoplay state and history
}

// Run initializes and starts the CLI application.
func Run(version string) error {
	// Parse flags
	flags := ParseFlags(version)

	// Initialize logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// In TUI mode, log to file instead of stderr to avoid collision
	if flags.TUI {
		if err := setupFileLogging(flags.Debug); err != nil {
			return fmt.Errorf("failed to setup file logging: %w", err)
		}
	} else {
		// CLI mode: log to stderr
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		if flags.Debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}
	}

	// Check config path
	if flags.ConfigPath == "" {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: config file not found"))
		fmt.Fprintln(os.Stderr, "Tried: ./config.toml and ~/.config/mysis/config.toml")
		return fmt.Errorf("config file not found")
	}

	log.Info().
		Str("version", version).
		Str("config", flags.ConfigPath).
		Msg("Starting Mysis")

	// Load config
	cfg, err := config.Load(flags.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open database
	db, err := store.Open()
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create session manager
	sessionMgr := session.NewManager(db)

	// Handle --list-sessions flag
	if flags.ListSessions {
		return listSessionsCmd(sessionMgr)
	}

	// Handle --delete-session flag
	if flags.DeleteSession != "" {
		return deleteSessionCmd(sessionMgr, flags.DeleteSession)
	}

	// Load credentials
	creds, err := config.LoadCredentials()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load credentials, using empty credentials")
		creds = &config.Credentials{}
	}

	// Initialize provider registry
	registry := initializeProviders(cfg, creds)

	// Determine provider and model
	providerResult, err := sessionMgr.SelectProvider(cfg, flags.SessionName, flags.ProviderName)
	if err != nil {
		return err
	}
	selectedProvider := providerResult.Provider
	selectedModel := providerResult.Model

	// Verify provider exists in config
	providerCfg, ok := cfg.Providers[selectedProvider]
	if !ok {
		return fmt.Errorf("provider '%s' not found in config", selectedProvider)
	}

	// Create provider instance
	prov, err := registry.Create(selectedProvider, selectedModel, providerCfg.Temperature)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}
	defer prov.Close()

	log.Info().
		Str("provider", selectedProvider).
		Str("model", selectedModel).
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

	// Initialize or resume session
	sessionResult, err := sessionMgr.Initialize(flags.SessionName, selectedProvider, selectedModel)
	if err != nil {
		return err
	}
	sessionID := sessionResult.SessionID
	sessionInfo := sessionResult.SessionInfo

	// Register credential tools (session-scoped)
	proxy.RegisterTool(
		mcp.NewSaveCredentialsTool(),
		mcp.MakeSaveCredentialsHandler(db, sessionID),
	)
	proxy.RegisterTool(
		mcp.NewGetCredentialsTool(),
		mcp.MakeGetCredentialsHandler(db, sessionID),
	)
	log.Debug().
		Str("session_id", sessionID).
		Int("local_tools", proxy.LocalToolCount()).
		Msg("Registered local credential tools")

	// Get available tools (includes upstream + local credential tools)
	tools, err := proxy.ListTools(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list tools - continuing without tools")
		tools = []mcp.Tool{}
	} else {
		log.Info().Int("count", len(tools)).Msg("Tools available")
	}

	// Load message history
	history, err := sessionMgr.LoadHistory(sessionID)
	if err != nil {
		return err
	}

	// Load system prompt from markdown file if provided
	if flags.SystemFile != "" {
		systemPrompt, err := loadSystemPromptFromFile(flags.SystemFile)
		if err != nil {
			return err
		}
		if !historyHasSystemPrompt(history, systemPrompt) {
			history = prependSystemPrompt(history, systemPrompt)
		}
	}

	// Check if TUI mode requested
	if flags.TUI {
		// Use TUI mode
		return runTUI(ctx, sessionMgr, sessionID, prov, proxy, tools, history)
	}

	// Print welcome message (CLI mode only)
	printWelcome(selectedProvider, selectedModel, len(tools), sessionInfo)

	// Start conversation loop (CLI mode)
	app := &App{
		provider:   prov,
		proxy:      proxy,
		tools:      tools,
		history:    history,
		sessionMgr: sessionMgr,
		sessionID:  sessionID,
	}

	// Start autoplay if flag provided
	if flags.Autoplay != "" {
		if err := app.startAutoplayFromFlag(ctx, flags.Autoplay); err != nil {
			return fmt.Errorf("failed to start autoplay: %w", err)
		}
	}

	return app.runLoop(ctx)
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
func listSessionsCmd(mgr *session.Manager) error {
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
func deleteSessionCmd(mgr *session.Manager, name string) error {
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

func loadSystemPromptFromFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return "", fmt.Errorf("system prompt file must be markdown (.md or .markdown): %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read system prompt file: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", fmt.Errorf("system prompt file is empty: %s", path)
	}

	return content, nil
}

func historyHasSystemPrompt(history []provider.Message, content string) bool {
	for _, msg := range history {
		if msg.Role == "system" && msg.Content == content {
			return true
		}
	}
	return false
}

func prependSystemPrompt(history []provider.Message, content string) []provider.Message {
	systemMsg := provider.Message{Role: "system", Content: content}
	return append([]provider.Message{systemMsg}, history...)
}

// setupFileLogging configures zerolog to write to a file in the config directory.
func setupFileLogging(debug bool) error {
	// Get data directory
	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	// Create logs directory
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}

	// Create log file with timestamp
	logFile := filepath.Join(logDir, "mysis.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	// Set up multi-writer: file (JSON) + console writer for debugging
	var writers []io.Writer

	// Always write JSON to file
	writers = append(writers, file)

	// In debug mode, also write human-readable logs to a separate debug file
	if debug {
		debugFile := filepath.Join(logDir, "mysis-debug.log")
		debugFileWriter, err := os.OpenFile(debugFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open debug log file: %w", err)
		}
		consoleWriter := zerolog.ConsoleWriter{Out: debugFileWriter, TimeFormat: time.RFC3339}
		writers = append(writers, consoleWriter)
	}

	// Configure logger
	multi := io.MultiWriter(writers...)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	// Set log level
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Info().
		Str("log_file", logFile).
		Bool("debug", debug).
		Msg("File logging initialized for TUI mode")

	return nil
}
