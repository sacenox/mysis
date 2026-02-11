package session

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/config"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/store"
)

// Manager handles session creation, resumption, and management.
type Manager struct {
	db *store.Store
}

// NewManager creates a new session manager.
func NewManager(db *store.Store) *Manager {
	return &Manager{db: db}
}

// InitializeResult holds the result of session initialization.
type InitializeResult struct {
	SessionID   string
	SessionInfo string
}

// Initialize creates or resumes a session.
func (m *Manager) Initialize(sessionName, provider, model string) (*InitializeResult, error) {
	var sessionID string
	var sessionInfo string

	if sessionName != "" {
		// Try to resume by name
		sess, err := m.db.GetSessionByName(sessionName)
		if err != nil {
			return nil, fmt.Errorf("failed to load session: %w", err)
		}
		if sess != nil {
			sessionID = sess.ID
			sessionInfo = fmt.Sprintf("Resumed session: %s", sessionName)
			log.Info().Str("session_id", sessionID).Str("name", sessionName).Msg("Resumed session")
		} else {
			// Create new named session
			sessionID = uuid.New().String()
			if err := m.db.CreateSession(sessionID, provider, model, &sessionName); err != nil {
				return nil, fmt.Errorf("failed to create session: %w", err)
			}
			sessionInfo = fmt.Sprintf("New session: %s", sessionName)
			log.Info().Str("session_id", sessionID).Str("name", sessionName).Msg("Created named session")
		}
	} else {
		// Create anonymous session
		sessionID = uuid.New().String()
		if err := m.db.CreateSession(sessionID, provider, model, nil); err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		sessionInfo = fmt.Sprintf("Session: %s", sessionID[:8])
		log.Info().Str("session_id", sessionID).Msg("Created anonymous session")
	}

	return &InitializeResult{
		SessionID:   sessionID,
		SessionInfo: sessionInfo,
	}, nil
}

// LoadHistory loads message history for a session.
func (m *Manager) LoadHistory(sessionID string) ([]provider.Message, error) {
	history, err := m.db.LoadMessages(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}
	if len(history) > 0 {
		log.Info().Int("count", len(history)).Msg("Loaded message history")
	}
	return history, nil
}

// SaveMessage saves a message to the session history.
func (m *Manager) SaveMessage(sessionID string, msg provider.Message) error {
	if err := m.db.SaveMessage(sessionID, msg); err != nil {
		log.Warn().Err(err).Msg("Failed to save message to database")
		return err
	}
	return nil
}

// SelectProviderResult holds the result of provider selection.
type SelectProviderResult struct {
	Provider string
	Model    string
}

// SelectProvider determines which provider and model to use.
func (m *Manager) SelectProvider(cfg *config.Config, sessionName, providerFlag string) (*SelectProviderResult, error) {
	var selectedProvider string
	var selectedModel string

	// Check if we're resuming a session to use its provider
	if sessionName != "" && providerFlag == "" {
		sess, err := m.db.GetSessionByName(sessionName)
		if err != nil {
			return nil, fmt.Errorf("failed to check session: %w", err)
		}
		if sess != nil {
			// Resuming existing session - use its provider
			selectedProvider = sess.Provider
			selectedModel = sess.Model
			log.Debug().
				Str("session", sessionName).
				Str("provider", selectedProvider).
				Str("model", selectedModel).
				Msg("Using provider from existing session")
			return &SelectProviderResult{
				Provider: selectedProvider,
				Model:    selectedModel,
			}, nil
		}
	}

	// If provider not determined from session, use flag or default
	selectedProvider = providerFlag
	if selectedProvider == "" {
		// Use default provider from config
		if cfg.DefaultProvider != "" {
			selectedProvider = cfg.DefaultProvider
		} else {
			// Fallback: use first provider (non-deterministic for backwards compatibility)
			for name := range cfg.Providers {
				selectedProvider = name
				break
			}
		}
	}

	if selectedProvider == "" {
		return nil, fmt.Errorf("no provider configured")
	}

	// Get model from config
	providerCfg, ok := cfg.Providers[selectedProvider]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found in config", selectedProvider)
	}
	selectedModel = providerCfg.Model

	return &SelectProviderResult{
		Provider: selectedProvider,
		Model:    selectedModel,
	}, nil
}

// List returns recent sessions.
func (m *Manager) List(limit int) ([]store.Session, error) {
	sessions, err := m.db.ListSessions(limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}

// DeleteByName deletes a session by name.
func (m *Manager) DeleteByName(name string) error {
	// Get session to verify it exists
	sess, err := m.db.GetSessionByName(name)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if sess == nil {
		return fmt.Errorf("session '%s' not found", name)
	}

	// Delete
	if err := m.db.DeleteSessionByName(name); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// GetByName retrieves a session by name.
func (m *Manager) GetByName(name string) (*store.Session, error) {
	sess, err := m.db.GetSessionByName(name)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

// FormatDuration formats a duration in human-readable form.
func FormatDuration(d time.Duration) string {
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
