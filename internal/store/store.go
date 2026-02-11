// Package store provides SQLite persistence for conversation history.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/config"
	"github.com/xonecas/mysis/internal/provider"
)

// Store handles database operations.
type Store struct {
	db *sql.DB
}

// Session represents a conversation session.
type Session struct {
	ID           string
	Name         *string
	Provider     string
	Model        string
	CreatedAt    time.Time
	LastActiveAt time.Time
}

// Message represents a stored message.
type Message struct {
	ID         int64
	SessionID  string
	Role       string
	Content    string
	ToolCallID *string
	ToolCalls  []provider.ToolCall
	Reasoning  *string
	CreatedAt  time.Time
}

// Open opens the database connection and ensures schema exists.
func Open() (*Store, error) {
	dataDir, err := config.EnsureDataDir()
	if err != nil {
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "mysis.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// initSchema creates tables if they don't exist.
func (s *Store) initSchema() error {
	// Create tables
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			name TEXT UNIQUE,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_active_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			tool_call_id TEXT,
			tool_calls TEXT,
			reasoning TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS session_credentials (
			session_id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			password TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_messages_session 
		ON messages(session_id, created_at);
	`)
	return err
}

// CreateSession creates a new session.
func (s *Store) CreateSession(id, provider, model string, name *string) error {
	query := `
		INSERT INTO sessions (id, name, provider, model, created_at, last_active_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	_, err := s.db.Exec(query, id, name, provider, model)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID.
func (s *Store) GetSession(id string) (*Session, error) {
	query := `
		SELECT id, name, provider, model, created_at, last_active_at
		FROM sessions
		WHERE id = ?
	`

	var sess Session
	var name sql.NullString
	err := s.db.QueryRow(query, id).Scan(
		&sess.ID,
		&name,
		&sess.Provider,
		&sess.Model,
		&sess.CreatedAt,
		&sess.LastActiveAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if name.Valid {
		sess.Name = &name.String
	}

	return &sess, nil
}

// GetSessionByName retrieves a session by name.
func (s *Store) GetSessionByName(name string) (*Session, error) {
	query := `
		SELECT id, name, provider, model, created_at, last_active_at
		FROM sessions
		WHERE name = ?
	`

	var sess Session
	var nameVal sql.NullString
	err := s.db.QueryRow(query, name).Scan(
		&sess.ID,
		&nameVal,
		&sess.Provider,
		&sess.Model,
		&sess.CreatedAt,
		&sess.LastActiveAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session by name: %w", err)
	}

	if nameVal.Valid {
		sess.Name = &nameVal.String
	}

	return &sess, nil
}

// ListSessions returns all sessions ordered by most recent.
func (s *Store) ListSessions(limit int) ([]Session, error) {
	query := `
		SELECT id, name, provider, model, created_at, last_active_at
		FROM sessions
		ORDER BY last_active_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var name sql.NullString
		if err := rows.Scan(
			&sess.ID,
			&name,
			&sess.Provider,
			&sess.Model,
			&sess.CreatedAt,
			&sess.LastActiveAt,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		if name.Valid {
			sess.Name = &name.String
		}

		sessions = append(sessions, sess)
	}

	return sessions, rows.Err()
}

// TouchSession updates the last_active_at timestamp.
func (s *Store) TouchSession(id string) error {
	query := `UPDATE sessions SET last_active_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.Exec(query, id)
	return err
}

// SaveMessage stores a message in the database.
func (s *Store) SaveMessage(sessionID string, msg provider.Message) error {
	// Marshal tool calls to JSON if present
	var toolCallsJSON *string
	if len(msg.ToolCalls) > 0 {
		data, err := json.Marshal(msg.ToolCalls)
		if err != nil {
			return fmt.Errorf("marshal tool calls: %w", err)
		}
		jsonStr := string(data)
		toolCallsJSON = &jsonStr
	}

	// Handle optional fields
	var toolCallID *string
	if msg.ToolCallID != "" {
		toolCallID = &msg.ToolCallID
	}

	// Reasoning: NULL if empty, otherwise the value
	var reasoning interface{}
	if msg.Reasoning == "" {
		reasoning = nil
	} else {
		reasoning = msg.Reasoning
	}

	query := `
		INSERT INTO messages (session_id, role, content, tool_call_id, tool_calls, reasoning)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query, sessionID, msg.Role, msg.Content, toolCallID, toolCallsJSON, reasoning)
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	// Touch session to update last_active_at
	return s.TouchSession(sessionID)
}

// LoadMessages retrieves all messages for a session.
func (s *Store) LoadMessages(sessionID string) ([]provider.Message, error) {
	query := `
		SELECT role, content, tool_call_id, tool_calls, reasoning, created_at
		FROM messages
		WHERE session_id = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []provider.Message
	for rows.Next() {
		var msg provider.Message
		var toolCallID sql.NullString
		var toolCallsJSON sql.NullString
		var reasoning sql.NullString
		var createdAt string

		if err := rows.Scan(&msg.Role, &msg.Content, &toolCallID, &toolCallsJSON, &reasoning, &createdAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		// Parse timestamp (SQLite CURRENT_TIMESTAMP uses ISO 8601 / RFC3339)
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			msg.CreatedAt = t
		} else {
			log.Warn().Err(err).Str("timestamp", createdAt).Msg("Failed to parse message timestamp")
		}

		if toolCallID.Valid {
			msg.ToolCallID = toolCallID.String
		}

		if toolCallsJSON.Valid {
			if err := json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls); err != nil {
				return nil, fmt.Errorf("unmarshal tool calls: %w", err)
			}
		}

		if reasoning.Valid {
			msg.Reasoning = reasoning.String
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// DeleteSession deletes a session and all its messages.
func (s *Store) DeleteSession(id string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := s.db.Exec(query, id)
	return err
}

// DeleteSessionByName deletes a session by name and all its messages.
func (s *Store) DeleteSessionByName(name string) error {
	query := `DELETE FROM sessions WHERE name = ?`
	result, err := s.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("delete session by name: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session '%s' not found", name)
	}

	return nil
}

// SaveCredentials stores game credentials for a session.
func (s *Store) SaveCredentials(sessionID, username, password string) error {
	query := `
		INSERT INTO session_credentials (session_id, username, password, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(session_id) DO UPDATE SET
			username = excluded.username,
			password = excluded.password,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.db.Exec(query, sessionID, username, password)
	if err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	return nil
}

// GetCredentials retrieves game credentials for a session.
func (s *Store) GetCredentials(sessionID string) (username, password string, err error) {
	query := `SELECT username, password FROM session_credentials WHERE session_id = ?`
	err = s.db.QueryRow(query, sessionID).Scan(&username, &password)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("get credentials: %w", err)
	}
	return username, password, nil
}
