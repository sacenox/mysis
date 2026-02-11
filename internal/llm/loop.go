package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/mysis/internal/mcp"
	"github.com/xonecas/mysis/internal/provider"
	"github.com/xonecas/mysis/internal/store"
	"github.com/xonecas/mysis/internal/styles"
)

// MessageCallback is called when a message should be added to history and saved.
type MessageCallback func(msg provider.Message)

// ToolCallCallback is called when tool calls are about to be executed.
type ToolCallCallback func()

// ProcessTurnOptions holds configuration for processing a turn.
type ProcessTurnOptions struct {
	Provider        provider.Provider
	Proxy           *mcp.Proxy
	Tools           []mcp.Tool
	History         []provider.Message
	OnMessage       MessageCallback
	OnToolCall      ToolCallCallback // Optional: called before executing tool calls
	MaxToolRounds   int
	HistoryKeepLast int
	SuppressOutput  bool // If true, suppress fmt.Println output (for TUI mode)
}

// ProcessTurn handles one conversation turn, which may involve tool calls.
// It returns an error if the LLM call fails or max rounds are exceeded.
func ProcessTurn(ctx context.Context, opts ProcessTurnOptions) error {
	if opts.MaxToolRounds == 0 {
		opts.MaxToolRounds = 20
	}
	if opts.HistoryKeepLast == 0 {
		opts.HistoryKeepLast = 10
	}

	for round := 0; round < opts.MaxToolRounds; round++ {
		// Compress history before sending to LLM
		// Keep last N turns full, compress older state queries
		compressedHistory := store.CompressHistory(opts.History, opts.HistoryKeepLast)

		// Log compression stats
		if len(compressedHistory) < len(opts.History) {
			originalTokens := store.EstimateTokenCount(opts.History)
			compressedTokens := store.EstimateTokenCount(compressedHistory)
			log.Debug().
				Int("original_msgs", len(opts.History)).
				Int("compressed_msgs", len(compressedHistory)).
				Int("original_tokens", originalTokens).
				Int("compressed_tokens", compressedTokens).
				Int("saved_tokens", originalTokens-compressedTokens).
				Msg("History compressed")
		}

		// Convert MCP tools to provider format
		providerTools := make([]provider.Tool, len(opts.Tools))
		for i, t := range opts.Tools {
			providerTools[i] = provider.Tool{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			}
		}

		// Call LLM with compressed history
		resp, err := opts.Provider.ChatWithTools(ctx, compressedHistory, providerTools)
		if err != nil {
			return fmt.Errorf("LLM call failed: %w", err)
		}

		// Display reasoning if present (CLI mode only)
		if resp.Reasoning != "" && !opts.SuppressOutput {
			displayReasoning(resp.Reasoning)
		}

		// If no tool calls, display text response and we're done
		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" && !opts.SuppressOutput {
				fmt.Println(resp.Content)
			}

			// Add assistant response to history
			assistantMsg := provider.Message{
				Role:      "assistant",
				Content:   resp.Content,
				Reasoning: resp.Reasoning,
				CreatedAt: time.Now(),
			}
			opts.OnMessage(assistantMsg)
			opts.History = append(opts.History, assistantMsg)

			return nil
		}

		// Tool calls present - add assistant message with tool calls to history
		assistantMsg := provider.Message{
			Role:      "assistant",
			Content:   resp.Content,
			Reasoning: resp.Reasoning,
			ToolCalls: resp.ToolCalls,
			CreatedAt: time.Now(),
		}
		opts.OnMessage(assistantMsg)
		opts.History = append(opts.History, assistantMsg)

		// Notify about tool calls if callback provided
		if opts.OnToolCall != nil {
			opts.OnToolCall()
		}

		// Execute each tool call and update history
		toolResults, err := executeToolCalls(ctx, opts.Proxy, resp.ToolCalls, opts.OnMessage, opts.SuppressOutput)
		if err != nil {
			return err
		}
		opts.History = append(opts.History, toolResults...)

		// Continue loop to let LLM process tool results
	}

	return fmt.Errorf("too many tool call rounds (limit: %d)", opts.MaxToolRounds)
}

// displayReasoning shows the LLM's reasoning in a compact format.
func displayReasoning(reasoning string) {
	// Trim excessive whitespace and collapse multiple spaces/newlines
	reasoning = strings.TrimSpace(reasoning)
	reasoning = strings.Join(strings.Fields(reasoning), " ")

	// Truncate if too long (from the end per design spec)
	if len(reasoning) > 200 {
		reasoning = "..." + reasoning[len(reasoning)-197:]
	}

	fmt.Println(styles.Muted.Render("∴ " + reasoning))
}

// executeToolCalls executes a list of tool calls and adds results to history.
// Returns the list of tool result messages that were added.
func executeToolCalls(ctx context.Context, proxy *mcp.Proxy, toolCalls []provider.ToolCall, onMessage MessageCallback, suppressOutput bool) ([]provider.Message, error) {
	var toolResults []provider.Message

	for _, toolCall := range toolCalls {
		if !suppressOutput {
			fmt.Print(styles.Secondary.Render(fmt.Sprintf("⚙ %s", toolCall.Name)))
		}

		// Show arguments (truncated if long)
		displayToolArguments(toolCall.Arguments, suppressOutput)

		// Execute tool via MCP proxy
		result, err := proxy.CallTool(ctx, toolCall.Name, toolCall.Arguments)

		if err != nil {
			if !suppressOutput {
				fmt.Println(styles.Error.Render(" ✗"))
				fmt.Println(styles.Error.Render("  Error: " + err.Error()))
			}

			// Add error result to history
			toolMsg := provider.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("Error: %v", err),
				ToolCallID: toolCall.ID,
				CreatedAt:  time.Now(),
			}
			onMessage(toolMsg)
			toolResults = append(toolResults, toolMsg)
			continue
		}

		// Check if result is an error
		if result.IsError {
			if !suppressOutput {
				fmt.Println(styles.Error.Render(" ✗"))
			}
			errText := extractTextFromContent(result.Content)
			if errText != "" && !suppressOutput {
				fmt.Println(styles.Error.Render("  " + errText))
			}

			// Add error result to history
			toolMsg := provider.Message{
				Role:       "tool",
				Content:    errText,
				ToolCallID: toolCall.ID,
				CreatedAt:  time.Now(),
			}
			onMessage(toolMsg)
			toolResults = append(toolResults, toolMsg)
			continue
		}

		// Success
		if !suppressOutput {
			fmt.Println(styles.Success.Render(" ✓"))
		}

		// Extract and display result
		resultText := extractTextFromContent(result.Content)
		displayToolResult(resultText, suppressOutput)

		// Add tool result to history
		toolMsg := provider.Message{
			Role:       "tool",
			Content:    resultText,
			ToolCallID: toolCall.ID,
			CreatedAt:  time.Now(),
		}
		onMessage(toolMsg)
		toolResults = append(toolResults, toolMsg)
	}

	return toolResults, nil
}

// displayToolArguments shows tool arguments in a truncated format.
func displayToolArguments(arguments json.RawMessage, suppressOutput bool) {
	if suppressOutput {
		return
	}

	var args map[string]interface{}
	if err := json.Unmarshal(arguments, &args); err == nil {
		argsStr, _ := json.Marshal(args)
		if len(argsStr) > 60 {
			argsStr = argsStr[:57]
			argsStr = append(argsStr, '.', '.', '.')
		}
		fmt.Print(styles.Muted.Render(string(argsStr)))
	}
}

// displayToolResult shows tool result in a truncated format.
func displayToolResult(resultText string, suppressOutput bool) {
	if suppressOutput {
		return
	}

	if len(resultText) > 100 {
		preview := resultText[:97] + "..."
		fmt.Println(styles.Muted.Render("  " + preview))
	} else if resultText != "" {
		fmt.Println(styles.Muted.Render("  " + resultText))
	}
}

// extractTextFromContent extracts text from MCP content blocks.
func extractTextFromContent(content []mcp.ContentBlock) string {
	var text string
	for _, block := range content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}
