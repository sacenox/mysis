package store

import (
	"testing"
)

func TestCredentialStorage(t *testing.T) {
	store, err := Open()
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	sessionID := "test-cred-session"

	// Create a test session
	if err := store.CreateSession(sessionID, "opencode", "test-model", nil); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer func() { _ = store.DeleteSession(sessionID) }()

	t.Run("save and retrieve credentials", func(t *testing.T) {
		username := "testuser"
		password := "testpass123"

		// Save credentials
		if err := store.SaveCredentials(sessionID, username, password); err != nil {
			t.Fatalf("failed to save credentials: %v", err)
		}

		// Retrieve credentials
		gotUsername, gotPassword, err := store.GetCredentials(sessionID)
		if err != nil {
			t.Fatalf("failed to get credentials: %v", err)
		}

		if gotUsername != username {
			t.Errorf("Username = %q, want %q", gotUsername, username)
		}

		if gotPassword != password {
			t.Errorf("Password = %q, want %q", gotPassword, password)
		}
	})

	t.Run("update existing credentials", func(t *testing.T) {
		// Save initial credentials
		_ = store.SaveCredentials(sessionID, "user1", "pass1")

		// Update credentials
		newUsername := "user2"
		newPassword := "pass2"
		if err := store.SaveCredentials(sessionID, newUsername, newPassword); err != nil {
			t.Fatalf("failed to update credentials: %v", err)
		}

		// Retrieve updated credentials
		gotUsername, gotPassword, err := store.GetCredentials(sessionID)
		if err != nil {
			t.Fatalf("failed to get credentials: %v", err)
		}

		if gotUsername != newUsername {
			t.Errorf("Username = %q, want %q", gotUsername, newUsername)
		}

		if gotPassword != newPassword {
			t.Errorf("Password = %q, want %q", gotPassword, newPassword)
		}
	})

	t.Run("no credentials for session", func(t *testing.T) {
		nonExistentSession := "no-such-session"

		username, password, err := store.GetCredentials(nonExistentSession)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if username != "" || password != "" {
			t.Errorf("expected empty credentials, got username=%q password=%q", username, password)
		}
	})

	t.Run("credentials deleted with session", func(t *testing.T) {
		// Create new session
		tempSession := "temp-session"
		_ = store.CreateSession(tempSession, "opencode", "test-model", nil)

		// Save credentials
		_ = store.SaveCredentials(tempSession, "tempuser", "temppass")

		// Verify credentials exist
		username, _, _ := store.GetCredentials(tempSession)
		if username != "tempuser" {
			t.Fatal("credentials not saved")
		}

		// Delete session
		_ = store.DeleteSession(tempSession)

		// Verify credentials are gone
		username, password, err := store.GetCredentials(tempSession)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if username != "" || password != "" {
			t.Errorf("credentials should be deleted with session")
		}
	})
}

func TestCredentialSessionIsolation(t *testing.T) {
	store, err := Open()
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	session1 := "session-1"
	session2 := "session-2"

	// Create sessions
	_ = store.CreateSession(session1, "opencode", "test-model", nil)
	_ = store.CreateSession(session2, "opencode", "test-model", nil)
	defer func() { _ = store.DeleteSession(session1) }()
	defer func() { _ = store.DeleteSession(session2) }()

	// Save different credentials for each session
	_ = store.SaveCredentials(session1, "user1", "pass1")
	_ = store.SaveCredentials(session2, "user2", "pass2")

	// Verify session 1
	username1, password1, err := store.GetCredentials(session1)
	if err != nil {
		t.Fatalf("failed to get session1 credentials: %v", err)
	}

	if username1 != "user1" || password1 != "pass1" {
		t.Errorf("session1: got %q/%q, want user1/pass1", username1, password1)
	}

	// Verify session 2
	username2, password2, err := store.GetCredentials(session2)
	if err != nil {
		t.Fatalf("failed to get session2 credentials: %v", err)
	}

	if username2 != "user2" || password2 != "pass2" {
		t.Errorf("session2: got %q/%q, want user2/pass2", username2, password2)
	}
}
