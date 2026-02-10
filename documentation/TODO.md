# TODO: Critical Bugs

## CRITICAL: Agent Repeating Tool Calls

Multiple critical bugs cause agents to repeat the same tool calls indefinitely. Root cause identified through comprehensive audit (2026-02-10).

---

### ~~BUG #1: History Snapshot Never Updated in Multi-Round Loop~~ [FIXED]

**Status:** ✅ FIXED (2026-02-10)

**Priority:** P0 - PRIMARY CAUSE of repeated tool calls

**Location:**

- `internal/cli/cli.go:266-282`
- `internal/llm/loop.go:40-107`

**Problem:**
`processTurn()` creates a snapshot of conversation history and passes it to `ProcessTurn()`. Inside the loop, `ProcessTurn()` iterates up to 20 rounds for tool calls. Each round uses the **same original snapshot** for compression.

When tool results are added via `OnMessage` callback, they update `app.history` but `opts.History` (the snapshot passed to ProcessTurn) never sees these new messages. The LLM repeatedly receives history without its own tool results.

**Example Flow:**

1. Round 0: LLM gets initial history, calls tool `get_status`
2. Tool result added to `app.history` via OnMessage
3. Round 1: `compressedHistory` regenerated from original snapshot (doesn't include tool result from round 0)
4. LLM receives history WITHOUT the tool result from round 0
5. LLM doesn't know it already called `get_status`, calls it again
6. Infinite loop until MaxToolRounds limit (20 rounds)

**Evidence:**

- `loop.go:43`: `compressedHistory := store.CompressHistory(opts.History, opts.HistoryKeepLast)` - always uses same snapshot
- `cli.go:269-271`: History snapshot made once, never updated during turn
- `loop.go:86-99, 142-146, 159-179`: Tool results added to `app.history`, but `opts.History` unchanged

**Fix Applied:** Option A - Update `opts.History` after each round

**Changes Made:**

1. Modified `internal/llm/loop.go:86-91` - Update `opts.History` after adding assistant response (no tool calls)
2. Modified `internal/llm/loop.go:96-103` - Update `opts.History` after adding assistant message with tool calls
3. Modified `internal/llm/loop.go:106-110` - Collect tool results and append to `opts.History`
4. Modified `executeToolCalls()` signature to return `([]provider.Message, error)` so tool results can be collected
5. Modified `executeToolCalls()` implementation to collect and return all tool result messages

**How It Works Now:**

- After each `OnMessage()` callback, the corresponding message is also appended to `opts.History`
- On the next loop iteration, `CompressHistory()` sees the complete history including all tool calls and results from previous rounds
- LLM receives compressed history that includes its own previous tool calls and results
- LLM no longer repeats tool calls because it can see the results in history

---

### BUG #2: Race Condition in History Append [CRITICAL]

**Priority:** P0 - Race condition with autoplay

**Location:**

- `internal/cli/cli.go:246` (manual input - NO LOCK)
- `internal/cli/autoplay.go:190` (autoplay - HAS LOCK)

**Problem:**
Manual user input appends to `app.history` WITHOUT mutex lock:

```go
// cli.go:246 - NO LOCK
app.history = append(app.history, userMsg)
```

But autoplay DOES lock:

```go
// autoplay.go:190 - HAS LOCK
app.mu.Lock()
app.history = append(app.history, userMsg)
app.mu.Unlock()
```

**Race Scenario:**

```
Time  Main Loop (manual input)         Autoplay Goroutine
----  ---------------------------      --------------------
T1                                     mu.Lock()
T2    app.history = append(...)
T3                                     app.history = append(...)
T4                                     mu.Unlock()
```

At T2, main loop appends without lock while autoplay holds lock. Violates mutex semantics.

**Impact:**

- Slice corruption: append may resize backing array while another goroutine accesses it
- Lost messages: concurrent appends may overwrite each other
- Panic: possible slice index out of range

**Fix:**

```go
// cli.go:246 - Add mutex protection
app.mu.Lock()
app.history = append(app.history, userMsg)
app.mu.Unlock()
```

---

### BUG #3: Shallow Copy in Compression Mutates Backing Arrays [CRITICAL]

**Priority:** P0 - Silent data corruption

**Location:**

- `internal/store/compression.go:132-134`
- `internal/store/compression.go:140-142`

**Problem:**

```go
// Line 132-134
compressedMsg := msg  // SHALLOW COPY - shares backing arrays!
compressedMsg.Content = compressedToolResult
compressed = append(compressed, compressedMsg)

// Line 140-142
compressedMsg := msg  // SHALLOW COPY - shares backing arrays!
compressedMsg.Content = msg.Content[:200] + "... [truncated]"
compressed = append(compressed, compressedMsg)
```

`compressedMsg := msg` creates shallow copy. The `ToolCalls` slice in both `msg` and `compressedMsg` point to the **same backing array**. Later append operations can mutate the backing array visible to both original and compressed history.

**Impact:**

- Messages in compressed history can have their `ToolCalls` mutated
- Database saves may contain corrupted ToolCalls data
- LLM may receive inconsistent history across rounds

**Fix:**

```go
// Deep copy the message struct
compressedMsg := provider.Message{
    Role:       msg.Role,
    Content:    compressedToolResult,
    ToolCallID: msg.ToolCallID,
    ToolCalls:  append([]provider.ToolCall(nil), msg.ToolCalls...),
}
```

---

### BUG #4: Naive Truncation Loses Critical Information [HIGH]

**Priority:** P1 - Can cause LLM to miss important game events

**Location:** `internal/store/compression.go:139-142`

**Problem:**
Truncation simply takes first 200 characters of action tool results. If critical information appears later in the response (common in verbose API responses), it gets lost.

```go
if len(msg.Content) > 500 {
    compressedMsg := msg
    compressedMsg.Content = msg.Content[:200] + "... [truncated]"
    compressed = append(compressed, compressedMsg)
}
```

**Real-world scenario:**

- SpaceMolt travel response: 1198 characters
- First 200 chars: Headers and departure info
- Lost content: "CRITICAL: Base commander requests immediate meeting for urgent mission!"
- Result: LLM misses quest hooks and important notifications

**Why this causes repeated tool calls:**
If truncation removes success confirmation or critical state changes, LLM may think action didn't complete and try again.

**Fix Options:**

Option A: Smart truncation (keep beginning and end)

```go
if len(msg.Content) > 500 {
    compressedMsg := msg
    start := msg.Content[:150]
    end := msg.Content[len(msg.Content)-50:]
    compressedMsg.Content = start + "... [middle truncated] ..." + end
    compressed = append(compressed, compressedMsg)
}
```

Option B: Configurable truncation

```go
const truncateKeepFirst = 150
const truncateKeepLast = 50

if len(msg.Content) > 500 {
    compressedMsg := msg
    start := msg.Content[:truncateKeepFirst]
    end := msg.Content[len(msg.Content)-truncateKeepLast:]
    compressedMsg.Content = start + "..." + end + " [truncated]"
    compressed = append(compressed, compressedMsg)
}
```

---

## Additional Issues Found During Audit

### BUG #5: Missing ToolCalls Field in No-Tool-Call Response [MEDIUM]

**Priority:** P2 - Data consistency issue

**Location:** `internal/llm/loop.go:86-89`

**Problem:**
When LLM responds with text only (no tool calls), assistant message is added without preserving the ToolCalls field:

```go
opts.OnMessage(provider.Message{
    Role:    "assistant",
    Content: resp.Content,
    // ToolCalls field missing - defaults to nil
})
```

**Fix:**

```go
opts.OnMessage(provider.Message{
    Role:      "assistant",
    Content:   resp.Content,
    ToolCalls: resp.ToolCalls,  // Include even if empty
})
```

---

### BUG #6: ProcessTurn Non-Blocking Allows Concurrent Autoplay [HIGH]

**Priority:** P1 - Autoplay timing bug

**Location:** `internal/cli/autoplay.go:198-206`

**Problem:**
After calling `processTurn()`, the function returns immediately without waiting for turn completion. The ticker continues and can fire again.

```go
if err := app.processTurn(ctx); err != nil {
    fmt.Fprintln(os.Stderr, styles.Error.Render("Error: "+err.Error()))
    log.Warn().Err(err).Msg("Autoplay turn failed, continuing...")
}

fmt.Println()
return nil  // Returns immediately
```

Autoplay interval: 100 seconds (10 tools × 10s/tick). But `processTurn()` can take longer if there are delays, retries, or >10 tool calls. Timer fires at fixed intervals regardless of whether previous turn finished.

**Impact:**
If turn takes >100 seconds, ticker fires again before previous turn completes, causing concurrent `processTurn()` executions.

**Fix:**
Add synchronization to block next autoplay message until current turn completes.
