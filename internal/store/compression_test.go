package store

import (
	"testing"

	"github.com/xonecas/mysis/internal/provider"
)

func TestCompressHistory_NoCompression(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	compressed := CompressHistory(messages, 10)

	if len(compressed) != len(messages) {
		t.Errorf("expected no compression, got %d messages from %d", len(compressed), len(messages))
	}
}

func TestCompressHistory_StateQueries(t *testing.T) {
	messages := []provider.Message{
		// Turn 1
		{Role: "user", Content: "what's my status?"},
		{Role: "assistant", Content: "", ToolCalls: []provider.ToolCall{
			{ID: "call1", Name: "get_status"},
		}},
		{Role: "tool", Content: "OLD STATUS DATA: very long result here...", ToolCallID: "call1"},
		{Role: "assistant", Content: "You're doing well"},

		// Turn 2
		{Role: "user", Content: "status again?"},
		{Role: "assistant", Content: "", ToolCalls: []provider.ToolCall{
			{ID: "call2", Name: "get_status"},
		}},
		{Role: "tool", Content: "NEW STATUS DATA: latest result", ToolCallID: "call2"},
		{Role: "assistant", Content: "Here's the update"},

		// Turn 3 (recent)
		{Role: "user", Content: "navigate somewhere"},
		{Role: "assistant", Content: "navigating..."},
	}

	// Keep last 2 turns full, compress older
	compressed := CompressHistory(messages, 2)

	// Find the old get_status result
	var oldStatusResult *provider.Message
	for i := range compressed {
		if compressed[i].Role == "tool" && compressed[i].ToolCallID == "call1" {
			oldStatusResult = &compressed[i]
			break
		}
	}

	if oldStatusResult == nil {
		t.Fatal("old status result not found in compressed history")
	}

	if oldStatusResult.Content != compressedToolResult {
		t.Errorf("old status result not compressed, got: %s", oldStatusResult.Content)
	}

	// Find the new get_status result
	var newStatusResult *provider.Message
	for i := range compressed {
		if compressed[i].Role == "tool" && compressed[i].ToolCallID == "call2" {
			newStatusResult = &compressed[i]
			break
		}
	}

	if newStatusResult == nil {
		t.Fatal("new status result not found in compressed history")
	}

	if newStatusResult.Content != "NEW STATUS DATA: latest result" {
		t.Errorf("new status result was compressed when it shouldn't be, got: %s", newStatusResult.Content)
	}
}

func TestCompressHistory_AuthToolsPreserved(t *testing.T) {
	messages := []provider.Message{
		// Turn 1 (old)
		{Role: "user", Content: "login"},
		{Role: "assistant", Content: "", ToolCalls: []provider.ToolCall{
			{ID: "call1", Name: "login"},
		}},
		{Role: "tool", Content: "Login successful with credentials", ToolCallID: "call1"},
		{Role: "assistant", Content: "Logged in"},

		// Turn 2
		{Role: "user", Content: "status?"},
		{Role: "assistant", Content: "", ToolCalls: []provider.ToolCall{
			{ID: "call2", Name: "get_status"},
		}},
		{Role: "tool", Content: "Some status data", ToolCallID: "call2"},
		{Role: "assistant", Content: "Here's your status"},

		// Turn 3 (recent)
		{Role: "user", Content: "navigate"},
		{Role: "assistant", Content: "navigating..."},
	}

	// Keep last 1 turn full, compress older
	compressed := CompressHistory(messages, 1)

	// Find the login result
	var loginResult *provider.Message
	for i := range compressed {
		if compressed[i].Role == "tool" && compressed[i].ToolCallID == "call1" {
			loginResult = &compressed[i]
			break
		}
	}

	if loginResult == nil {
		t.Fatal("login result not found in compressed history")
	}

	// Auth tools should NEVER be compressed
	if loginResult.Content != "Login successful with credentials" {
		t.Errorf("login result was compressed, got: %s", loginResult.Content)
	}
}

func TestEstimateTokenCount(t *testing.T) {
	messages := []provider.Message{
		{Role: "user", Content: "hello world"},               // ~3 tokens + 4 overhead
		{Role: "assistant", Content: "hi there how are you"}, // ~5 tokens + 4 overhead
	}

	tokens := EstimateTokenCount(messages)

	// Should be roughly (12 + 20) / 4 + 8 = ~16
	if tokens < 10 || tokens > 20 {
		t.Errorf("token estimate seems off: %d", tokens)
	}
}

func TestIsStateQueryTool(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"get_status", true},
		{"get_ship", true},
		{"get_map", true},
		{"GET_STATUS", true}, // case insensitive
		{"login", false},
		{"travel", false},
		{"mine", false},
	}

	for _, tt := range tests {
		got := isStateQueryTool(tt.name)
		if got != tt.want {
			t.Errorf("isStateQueryTool(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsAuthTool(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"login", true},
		{"register", true},
		{"logout", true},
		{"LOGIN", true}, // case insensitive
		{"get_status", false},
		{"travel", false},
	}

	for _, tt := range tests {
		got := isAuthTool(tt.name)
		if got != tt.want {
			t.Errorf("isAuthTool(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
