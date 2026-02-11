package features

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/config"
	"github.com/xonecas/mysis/internal/provider"
)

// InitializeProviders initializes the provider registry from config.
// This is shared by both CLI and TUI modes.
func InitializeProviders(cfg *config.Config, creds *config.Credentials) *provider.Registry {
	registry := provider.NewRegistry()

	for name, provCfg := range cfg.Providers {
		// Detect provider type based on endpoint
		switch {
		case strings.Contains(provCfg.Endpoint, "localhost:11434"), strings.Contains(provCfg.Endpoint, "/ollama"):
			// Ollama provider
			factory := provider.NewOllamaFactory(name, provCfg.Endpoint)
			registry.RegisterFactory(name, factory)
			log.Debug().Str("name", name).Str("endpoint", provCfg.Endpoint).Msg("Registered Ollama provider")
		case strings.Contains(provCfg.Endpoint, "opencode.ai"):
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
		default:
			log.Warn().Str("name", name).Str("endpoint", provCfg.Endpoint).Msg("Unknown provider type")
		}
	}

	return registry
}

// LoadSystemPromptFromFile loads a system prompt from a markdown file.
func LoadSystemPromptFromFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return "", fmt.Errorf("system prompt file must be markdown (.md or .markdown): %s", path)
	}

	//nolint:gosec // G304: Path from validated config file
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

// HistoryHasSystemPrompt checks if the history already has a system prompt with the given content.
func HistoryHasSystemPrompt(history []provider.Message, content string) bool {
	for _, msg := range history {
		if msg.Role == "system" && msg.Content == content {
			return true
		}
	}
	return false
}

// PrependSystemPrompt prepends a system prompt to the history.
func PrependSystemPrompt(history []provider.Message, content string) []provider.Message {
	systemMsg := provider.Message{Role: "system", Content: content}
	return append([]provider.Message{systemMsg}, history...)
}

// SetupFileLogging configures zerolog to write to a file.
// This is used by TUI mode to avoid collision with the UI.
func SetupFileLogging(debug bool) error {
	// Get data directory
	dataDir, err := config.DataDir()
	if err != nil {
		return fmt.Errorf("get data directory: %w", err)
	}

	// Create logs directory
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}

	// Create log file
	logFile := filepath.Join(logDir, "mysis.log")
	//nolint:gosec // G304: Path from validated config file
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
		//nolint:gosec // G304: Debug log file path is constructed from validated data directory
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
		Msg("File logging initialized")

	return nil
}
