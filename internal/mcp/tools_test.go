package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

// mockCredentialStore is a mock implementation for testing.
type mockCredentialStore struct {
	credentials map[string]struct{ username, password string }
}

func newMockCredentialStore() *mockCredentialStore {
	return &mockCredentialStore{
		credentials: make(map[string]struct{ username, password string }),
	}
}

func (m *mockCredentialStore) SaveCredentials(sessionID, username, password string) error {
	m.credentials[sessionID] = struct{ username, password string }{username, password}
	return nil
}

func (m *mockCredentialStore) GetCredentials(sessionID string) (string, string, error) {
	cred, exists := m.credentials[sessionID]
	if !exists {
		return "", "", nil
	}
	return cred.username, cred.password, nil
}

func TestSaveCredentialsTool(t *testing.T) {
	store := newMockCredentialStore()
	sessionID := "test-session-123"
	handler := MakeSaveCredentialsHandler(store, sessionID)

	tests := []struct {
		name      string
		args      SaveCredentialsArgs
		wantError bool
		wantText  string
	}{
		{
			name:      "valid credentials",
			args:      SaveCredentialsArgs{Username: "player1", Password: "secret123"},
			wantError: false,
			wantText:  "Credentials saved successfully for user 'player1'",
		},
		{
			name:      "empty username",
			args:      SaveCredentialsArgs{Username: "", Password: "secret123"},
			wantError: true,
			wantText:  "Username cannot be empty",
		},
		{
			name:      "empty password",
			args:      SaveCredentialsArgs{Username: "player1", Password: ""},
			wantError: true,
			wantText:  "Password cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsJSON, _ := json.Marshal(tt.args)
			result, err := handler(context.Background(), argsJSON)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError != tt.wantError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantError)
			}

			if len(result.Content) == 0 {
				t.Fatal("expected content blocks")
			}

			if result.Content[0].Text != tt.wantText {
				t.Errorf("Text = %q, want %q", result.Content[0].Text, tt.wantText)
			}
		})
	}
}

func TestGetCredentialsTool(t *testing.T) {
	store := newMockCredentialStore()
	sessionID := "test-session-456"
	handler := MakeGetCredentialsHandler(store, sessionID)

	t.Run("no credentials saved", func(t *testing.T) {
		result, err := handler(context.Background(), json.RawMessage("{}"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Error("expected no error")
		}

		if result.Content[0].Text != "No credentials saved for this session" {
			t.Errorf("unexpected text: %s", result.Content[0].Text)
		}
	})

	t.Run("credentials exist", func(t *testing.T) {
		// Save credentials first
		_ = store.SaveCredentials(sessionID, "player2", "pass456")

		result, err := handler(context.Background(), json.RawMessage("{}"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Error("expected no error")
		}

		var creds GetCredentialsResult
		if err := json.Unmarshal([]byte(result.Content[0].Text), &creds); err != nil {
			t.Fatalf("failed to parse result: %v", err)
		}

		if creds.Username != "player2" {
			t.Errorf("Username = %q, want %q", creds.Username, "player2")
		}

		if creds.Password != "pass456" {
			t.Errorf("Password = %q, want %q", creds.Password, "pass456")
		}
	})
}

func TestCredentialToolDefinitions(t *testing.T) {
	t.Run("save_credentials tool", func(t *testing.T) {
		tool := NewSaveCredentialsTool()

		if tool.Name != "save_credentials" {
			t.Errorf("Name = %q, want %q", tool.Name, "save_credentials")
		}

		if tool.Description == "" {
			t.Error("Description should not be empty")
		}

		if len(tool.InputSchema) == 0 {
			t.Error("InputSchema should not be empty")
		}

		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Fatalf("failed to parse schema: %v", err)
		}

		if schema["type"] != "object" {
			t.Error("schema type should be object")
		}
	})

	t.Run("get_credentials tool", func(t *testing.T) {
		tool := NewGetCredentialsTool()

		if tool.Name != "get_credentials" {
			t.Errorf("Name = %q, want %q", tool.Name, "get_credentials")
		}

		if tool.Description == "" {
			t.Error("Description should not be empty")
		}

		if len(tool.InputSchema) == 0 {
			t.Error("InputSchema should not be empty")
		}
	})
}

func TestSessionIsolation(t *testing.T) {
	store := newMockCredentialStore()

	// Session 1
	session1 := "session-1"
	handler1 := MakeSaveCredentialsHandler(store, session1)
	args1, _ := json.Marshal(SaveCredentialsArgs{Username: "user1", Password: "pass1"})
	_, _ = handler1(context.Background(), args1)

	// Session 2
	session2 := "session-2"
	handler2 := MakeSaveCredentialsHandler(store, session2)
	args2, _ := json.Marshal(SaveCredentialsArgs{Username: "user2", Password: "pass2"})
	_, _ = handler2(context.Background(), args2)

	// Verify session 1 credentials
	getHandler1 := MakeGetCredentialsHandler(store, session1)
	result1, _ := getHandler1(context.Background(), json.RawMessage("{}"))
	var creds1 GetCredentialsResult
	_ = json.Unmarshal([]byte(result1.Content[0].Text), &creds1)

	if creds1.Username != "user1" {
		t.Errorf("Session 1: Username = %q, want %q", creds1.Username, "user1")
	}

	// Verify session 2 credentials
	getHandler2 := MakeGetCredentialsHandler(store, session2)
	result2, _ := getHandler2(context.Background(), json.RawMessage("{}"))
	var creds2 GetCredentialsResult
	_ = json.Unmarshal([]byte(result2.Content[0].Text), &creds2)

	if creds2.Username != "user2" {
		t.Errorf("Session 2: Username = %q, want %q", creds2.Username, "user2")
	}
}
