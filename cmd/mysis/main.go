package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/cli"
	"github.com/xonecas/mysis/internal/config"
	"github.com/xonecas/mysis/internal/features"
	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/session"
	"github.com/xonecas/mysis/internal/store"
	"github.com/xonecas/mysis/internal/styles"
	"github.com/xonecas/mysis/internal/tui"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// Parse flags
	flags := features.ParseFlags()

	// Handle version flag
	if flags.ShowVersion {
		cli.PrintVersion(Version)
		os.Exit(0)
	}

	// Handle help flag
	if flags.ShowHelp {
		cli.PrintHelp(Version)
		os.Exit(0)
	}

	// Initialize logging
	if err := setupLogging(flags); err != nil {
		return err
	}

	// Check config path
	if flags.ConfigPath == "" {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: config file not found"))
		fmt.Fprintln(os.Stderr, "Tried: ./config.toml and ~/.config/mysis/config.toml")
		return fmt.Errorf("config file not found")
	}

	log.Info().
		Str("version", Version).
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
		return cli.ListSessionsCmd(sessionMgr)
	}

	// Handle --delete-session flag
	if flags.DeleteSession != "" {
		return cli.DeleteSessionCmd(sessionMgr, flags.DeleteSession)
	}

	// Load credentials
	creds, err := config.LoadCredentials()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load credentials, using empty credentials")
		creds = &config.Credentials{}
	}

	// Initialize provider registry
	registry := features.InitializeProviders(cfg, creds)

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
		systemPrompt, err := features.LoadSystemPromptFromFile(flags.SystemFile)
		if err != nil {
			return err
		}
		if !features.HistoryHasSystemPrompt(history, systemPrompt) {
			history = features.PrependSystemPrompt(history, systemPrompt)
		}
	}

	// Delegate to TUI or CLI based on flag
	if flags.TUI {
		// Use TUI mode
		return tui.Start(ctx, sessionMgr, sessionID, prov, proxy, tools, history)
	}

	// Use CLI mode
	return cli.Start(ctx, sessionMgr, sessionID, sessionInfo, prov, proxy, tools, history, flags.Autoplay, selectedProvider, selectedModel)
}

func setupLogging(flags *features.Flags) error {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	if flags.TUI {
		// TUI mode: log to file to avoid collision with UI
		return features.SetupFileLogging(flags.Debug)
	}

	// CLI mode: log to stderr
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if flags.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	return nil
}
