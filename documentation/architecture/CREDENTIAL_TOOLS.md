# Credential Management Tools

Added two MCP tools for session-scoped credential storage:

## Implementation

### Database Schema

Added `session_credentials` table to store game credentials per session:

```sql
CREATE TABLE IF NOT EXISTS session_credentials (
    session_id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);
```

**Key features:**

- One credential pair per session (PRIMARY KEY on session_id)
- Automatic cleanup via CASCADE when session is deleted
- Foreign keys enabled in SQLite connection string

### Store Functions

**`internal/store/store.go`:**

- `SaveCredentials(sessionID, username, password)` - Upsert credentials
- `GetCredentials(sessionID) (username, password, error)` - Retrieve credentials
- Returns empty strings if no credentials saved (not an error)

### MCP Tools

**`internal/mcp/tools.go`:**

#### `save_credentials`

Saves username and password for the current session.

**Input Schema:**

```json
{
  "username": "string (required)",
  "password": "string (required)"
}
```

**Response:**

```
Credentials saved successfully for user 'username'
```

**Validation:**

- Username cannot be empty
- Password cannot be empty

#### `get_credentials`

Retrieves saved credentials for the current session.

**Input Schema:**

```json
{}
```

**Response (credentials exist):**

```json
{
  "username": "player123",
  "password": "secret456"
}
```

**Response (no credentials):**

```
No credentials saved for this session
```

## Usage

Register tools with the proxy:

```go
store, _ := store.Open()
proxy := mcp.NewProxy(upstream)

proxy.RegisterTool(
    mcp.NewSaveCredentialsTool(),
    mcp.MakeSaveCredentialsHandler(store, sessionID),
)

proxy.RegisterTool(
    mcp.NewGetCredentialsTool(),
    mcp.MakeGetCredentialsHandler(store, sessionID),
)
```

## Session Isolation

Credentials are strictly scoped:

- Session A saves "user1/pass1" → only Session A can retrieve it
- Session B saves "user2/pass2" → only Session B can retrieve it
- Deleting a session automatically removes its credentials

## Testing

All tests pass:

- `internal/mcp/tools_test.go` - Unit tests for tool handlers
- `internal/mcp/integration_test.go` - Full workflow with proxy
- `internal/store/credentials_test.go` - Database operations
- Cascade deletion verified
- Session isolation verified

## Integration Status

✅ **Tools are registered in the CLI** (`internal/cli/cli.go:145-157`)

The credential tools are automatically registered when the CLI starts:

1. Session is initialized (gets unique sessionID)
2. Tools are registered with session-scoped handlers
3. Tools are included in the tool list sent to the LLM
4. Agent can call `save_credentials` and `get_credentials` automatically

**Logs:**
- Debug log shows: `Registered local credential tools` with session_id and tool count
- Info log shows total tool count (upstream + local credentials)

**How it works:**

```go
// internal/cli/cli.go (lines 145-157)
proxy.RegisterTool(
    mcp.NewSaveCredentialsTool(),
    mcp.MakeSaveCredentialsHandler(db, sessionID),
)
proxy.RegisterTool(
    mcp.NewGetCredentialsTool(),
    mcp.MakeGetCredentialsHandler(db, sessionID),
)
```

The sessionID is injected into the handlers via closure, never exposed to the agent.

## Files Modified

1. `internal/store/store.go` - Added schema + store functions
2. `internal/mcp/tools.go` - Implemented MCP tools (144 lines)
3. `internal/mcp/tools_test.go` - Unit tests (213 lines)
4. `internal/store/credentials_test.go` - Store tests (153 lines)
5. `internal/cli/cli.go` - Tool registration in CLI
6. `Makefile` - Added `backup-credentials` command

## Backup and Restore

**Backup credentials:**

```bash
make backup-credentials
```

This creates a timestamped SQL file (e.g., `credentials-backup-20260210-180025.sql`) containing INSERT statements for all session credentials.

**Restore credentials:**

```bash
sqlite3 ~/.config/mysis/mysis.db < credentials-backup-20260210-180025.sql
```

**Notes:**

- Backup files contain only INSERT statements (no schema)
- Can be merged into existing database
- Credentials are tied to session IDs (sessions must exist for foreign key constraint)
- Backup filename includes timestamp for safe multiple backups
