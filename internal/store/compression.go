package store

import (
	"encoding/json"
	"strings"

	"github.com/xonecas/mysis/internal/provider"
)

// ToolCategory represents the category of a tool for compression purposes.
type ToolCategory int

const (
	// ToolCategoryAuth - Authentication tools (never compress)
	ToolCategoryAuth ToolCategory = iota
	// ToolCategoryState - State query tools (compress old results, keep latest)
	ToolCategoryState
	// ToolCategoryAction - Action tools (keep full history)
	ToolCategoryAction
)

// compressedToolResult is a marker for compressed content
const compressedToolResult = "[compressed - old state data]"

// isStateQueryTool returns true if the tool is a state query that can be compressed.
func isStateQueryTool(toolName string) bool {
	stateTools := []string{
		"get_status",
		"get_ship",
		"get_system",
		"get_sector",
		"get_galaxy",
		"get_map",
		"get_players",
		"get_leaderboard",
		"get_market",
		"get_cargo",
		"captains_log_list",
	}

	toolName = strings.ToLower(toolName)
	for _, st := range stateTools {
		if toolName == st {
			return true
		}
	}
	return false
}

// isAuthTool returns true if the tool is authentication-related (never compress).
func isAuthTool(toolName string) bool {
	authTools := []string{
		"login",
		"register",
		"logout",
	}

	toolName = strings.ToLower(toolName)
	for _, at := range authTools {
		if toolName == at {
			return true
		}
	}
	return false
}

// CompressHistory compresses old tool results while preserving recent context.
func CompressHistory(messages []provider.Message, keepFullTurns int) []provider.Message {
	if len(messages) == 0 {
		return messages
	}

	// Count turns (user messages)
	turnCount := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			turnCount++
		}
	}

	// If we have fewer turns than the threshold, no compression needed
	if turnCount <= keepFullTurns {
		return messages
	}

	// Find the cutoff point (first message of the turn that should be kept)
	// If keepFullTurns=2, we want to keep the last 2 turns and compress everything before
	currentTurn := 0
	cutoffIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			currentTurn++
			if currentTurn == keepFullTurns {
				// This is the first user message of the Nth-from-last turn
				// The cutoff is this message (we keep from here onwards)
				cutoffIndex = i
				break
			}
		}
	}

	if cutoffIndex == -1 || cutoffIndex == 0 {
		return messages
	}

	// Build compressed history
	compressed := make([]provider.Message, 0, len(messages))

	// Second pass: compress old messages
	for i := 0; i < cutoffIndex; i++ {
		msg := messages[i]

		// Keep user messages and assistant messages (they're small)
		if msg.Role == "user" || msg.Role == "assistant" {
			compressed = append(compressed, msg)
			continue
		}

		// Handle tool results
		if msg.Role == "tool" {
			// Find the tool name from the assistant message
			toolName := findToolNameForResult(messages, i)

			// Never compress auth tools
			if isAuthTool(toolName) {
				compressed = append(compressed, msg)
				continue
			}

			// For state queries in old section, always compress
			if isStateQueryTool(toolName) {
				compressedMsg := msg
				compressedMsg.Content = compressedToolResult
				compressed = append(compressed, compressedMsg)
				continue
			}

			// For action tools, compress if result is too long
			if len(msg.Content) > 500 {
				compressedMsg := msg
				compressedMsg.Content = msg.Content[:200] + "... [truncated]"
				compressed = append(compressed, compressedMsg)
			} else {
				compressed = append(compressed, msg)
			}
		}
	}

	// Add all recent messages (after cutoff) unchanged
	compressed = append(compressed, messages[cutoffIndex:]...)

	return compressed
}

// findToolNameForResult finds the tool name for a tool result message
// by looking back for the assistant message with the matching tool call.
func findToolNameForResult(messages []provider.Message, resultIndex int) string {
	if resultIndex >= len(messages) || messages[resultIndex].Role != "tool" {
		return ""
	}

	toolCallID := messages[resultIndex].ToolCallID

	// Search backwards for the assistant message with this tool call
	for i := resultIndex - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if tc.ID == toolCallID {
					return tc.Name
				}
			}
		}
	}

	return ""
}

// EstimateTokenCount provides a rough estimate of token count for history.
// This is used for logging and debugging, not exact.
func EstimateTokenCount(messages []provider.Message) int {
	total := 0
	for _, msg := range messages {
		// Rough estimate: ~4 characters per token
		total += len(msg.Content) / 4

		// Add tool calls
		if len(msg.ToolCalls) > 0 {
			data, _ := json.Marshal(msg.ToolCalls)
			total += len(data) / 4
		}

		// Add role overhead
		total += 4
	}
	return total
}
