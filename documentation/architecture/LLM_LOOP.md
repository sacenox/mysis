# LLM Processing Loop

Single-turn conversation processing with multi-round tool call support.

## Overview

The LLM loop handles one conversation turn, which may involve multiple rounds of LLM calls and tool executions. A **turn** begins when the user sends a message and ends when the LLM provides a final text response without tool calls.

**Key principle:** The loop accumulates history during the turn so the LLM sees its own tool calls and results across rounds.

## Architecture

```
User Input → ProcessTurn → [Round 0, Round 1, ..., Round N] → Final Response
                              ↓
                          Compress History → LLM Call → Tool Execution
                              ↑_______________|
```

## Flow Diagram

```
processTurn (cli.go:266-282)
  │
  ├─ Snapshot app.history → historyCopy
  │
  └─ ProcessTurn (llm/loop.go:32-116)
      │
      └─ Loop: round = 0 to MaxToolRounds (20)
          │
          ├─ Compress historyCopy → compressedHistory
          │
          ├─ LLM Call with compressedHistory + tools
          │    └─ Returns: content, tool_calls, reasoning
          │
          ├─ If no tool_calls:
          │    ├─ Display content
          │    ├─ OnMessage(assistant_msg)
          │    ├─ Append to historyCopy
          │    └─ RETURN (turn complete)
          │
          └─ If tool_calls present:
               ├─ OnMessage(assistant_msg_with_tools)
               ├─ Append to historyCopy
               │
               ├─ executeToolCalls:
               │    └─ For each tool:
               │         ├─ Execute via MCP proxy
               │         ├─ OnMessage(tool_result)
               │         └─ Collect tool_result
               │
               ├─ Append all tool_results to historyCopy
               └─ Continue to next round
```

## Implementation Details

### Entry Point: `ProcessTurn`

**Location:** `internal/llm/loop.go:32-116`

**Signature:**

```go
func ProcessTurn(ctx context.Context, opts ProcessTurnOptions) error
```

**Options:**

- `Provider`: LLM provider interface (Ollama, OpenCode Zen, etc.)
- `Proxy`: MCP proxy for tool execution
- `Tools`: Available tools for this turn
- `History`: Initial conversation history snapshot
- `OnMessage`: Callback to save messages to database and `app.history`
- `MaxToolRounds`: Maximum iterations (default: 20)
- `HistoryKeepLast`: How many recent turns to keep uncompressed (default: 10)

### Round Loop

Each round performs:

1. **History Compression** (line 43)
   - Compresses old state query results
   - Keeps recent turns full
   - See `internal/store/compression.go` for details

2. **LLM Call** (line 69)
   - Sends compressed history + available tools
   - Receives response with optional tool calls

3. **Response Handling**
   - **No tool calls** (lines 80-93): Display content, save message, update history, exit
   - **Tool calls present** (lines 96-110): Save assistant message, execute tools, update history, continue

4. **History Update** (Critical Fix - Bug #1)
   - After `OnMessage()` callback, message is also appended to `opts.History`
   - This ensures next round sees messages from previous rounds
   - Without this, LLM would repeat tool calls indefinitely

### Tool Execution: `executeToolCalls`

**Location:** `internal/llm/loop.go:130-195`

**Signature:**

```go
func executeToolCalls(ctx context.Context, proxy *mcp.Proxy, toolCalls []provider.ToolCall, onMessage MessageCallback) ([]provider.Message, error)
```

**Flow:**

1. For each tool call:
   - Display tool name and arguments (truncated)
   - Execute via MCP proxy
   - Display result (success ✓ or error ✗)
   - Create tool result message
   - Call `onMessage()` to save to database
   - Collect message for return

2. Return all tool result messages
   - Caller appends to `opts.History`
   - Ensures next round sees tool results

### Message Callback: `OnMessage`

**Implementation:** `internal/cli/cli.go:285-293`

```go
func (app *App) addMessage(msg provider.Message) {
    app.mu.Lock()
    app.history = append(app.history, msg)
    app.mu.Unlock()

    if err := app.sessionMgr.SaveMessage(app.sessionID, msg); err != nil {
        log.Warn().Err(err).Msg("Failed to save message to database")
    }
}
```

**Purpose:**

- Updates canonical `app.history` with mutex protection
- Saves message to SQLite database for session persistence
- Called for every message: user, assistant, tool results

**Note:** The callback updates `app.history` but NOT `opts.History`. The caller (`ProcessTurn`) must also update `opts.History` after calling the callback.

## History Management

### Snapshot vs Reference

**Design Decision:** Pass history snapshot, not reference.

**Rationale:**

- Prevents concurrent modification during processing
- Clean separation between "input to turn" and "output from turn"
- Next turn gets fresh snapshot with all previous messages

**Implementation:**

```go
// cli.go:268-271
app.mu.Lock()
historyCopy := make([]provider.Message, len(app.history))
copy(historyCopy, app.history)
app.mu.Unlock()
```

### History Accumulation During Turn

**Critical Fix (2026-02-10):** After each message is saved via `OnMessage()`, it's also appended to `opts.History`.

**Before Fix:**

```go
opts.OnMessage(assistantMsg)
// opts.History unchanged - next round uses stale history
```

**After Fix:**

```go
opts.OnMessage(assistantMsg)
opts.History = append(opts.History, assistantMsg)
// Next round sees this message
```

**Why This Matters:**

Round 0:

- `opts.History = [user: "check status"]`
- LLM calls `get_status`
- Tool result saved via `OnMessage` to `app.history`
- **Before fix:** `opts.History` still `[user: "check status"]`
- **After fix:** `opts.History = [user: "check status", assistant: tool_call, tool: result]`

Round 1:

- **Before fix:** LLM sees compressed `[user: "check status"]` → calls `get_status` again (doesn't see result)
- **After fix:** LLM sees compressed `[user: "check status", assistant: tool_call, tool: result]` → processes result

## Compression During Turn

**Key Insight:** Compression operates on `opts.History`, which now includes messages from previous rounds within the same turn.

**Process:**

1. `CompressHistory(opts.History, keepFullTurns=10)` compresses old state queries
2. Recent turns (last 10) kept full
3. Old action tool results truncated to 200 chars
4. Authentication tools never compressed

**See:** `internal/store/compression.go` for compression logic

## Loop Termination

**Success Exit:**

- LLM returns response without tool calls (line 91)
- Turn completes, user regains control

**Error Exit:**

- LLM call fails (line 70-72)
- Tool execution fails (line 107-109)
- Max rounds exceeded (line 115)

**Max Rounds:** 20 iterations prevents infinite loops if LLM repeatedly calls same tool.

## Message Roles

### User Messages

- **Origin:** User input or autoplay
- **Added:** Before `processTurn()` call
- **Location:** `cli.go:246` (manual), `autoplay.go:190` (autoplay)

### Assistant Messages

- **Origin:** LLM response
- **Added:** During `ProcessTurn` round
- **Variants:**
  - Text only: `Role: "assistant", Content: "..."`
  - With tool calls: `Role: "assistant", Content: "...", ToolCalls: [...]`

### Tool Messages

- **Origin:** Tool execution results
- **Added:** During `executeToolCalls`
- **Format:** `Role: "tool", Content: "result", ToolCallID: "..."`

## Thread Safety

**Mutex Protection:**

- `app.mu` protects `app.history` (cli.go:34)
- Lock acquired for snapshot creation (cli.go:268-271)
- Lock acquired for message append via `addMessage()` (cli.go:286-288)
- `opts.History` is local to `ProcessTurn`, no lock needed

**Concurrency Scenarios:**

- Manual input and autoplay both call `processTurn()` serially
- Each turn operates on its own snapshot
- Callbacks serialize access to `app.history` via mutex

## Error Handling

**LLM Call Failure:**

- Returns error immediately
- Caller displays error to user
- Turn aborted

**Tool Call Failure:**

- Creates error tool result: `Content: "Error: ..."`
- Continues to next tool in list
- LLM sees error in next round, can recover

**Max Rounds Exceeded:**

- Returns error: `"too many tool call rounds (limit: 20)"`
- Indicates LLM stuck in loop (bug or model issue)
- User sees error, can try different prompt

## Performance Characteristics

**History Compression:**

- Reduces token usage for long conversations
- Trade-off: Old details lost vs context window limits
- Compression runs every round (negligible overhead)

**Round Complexity:**

- Each round: 1 LLM call + N tool executions
- Typical turn: 1-5 rounds
- Worst case: 20 rounds (max limit)

**Memory:**

- `opts.History` grows during turn (O(messages))
- Snapshot copied at turn start (O(messages))
- Released at turn end

## Configuration

**MaxToolRounds:** 20

- Prevents infinite loops
- Increase if legitimate use cases need more rounds
- Decrease to fail fast during debugging

**HistoryKeepLast:** 10

- Number of recent turns to keep uncompressed
- Increase for more context (higher token usage)
- Decrease for faster compression (less context)

## Future Improvements

**Streaming:** Support streaming LLM responses for faster perceived performance

**History Window:** Smarter compression based on token count, not just turn count
