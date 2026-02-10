# Mysis Project Structure

This document describes the scaffolded project structure and reused components from zoea-nova.

## Directory Structure

```
mysis/
├── cmd/
│   └── mysis/           # Main application entry point
├── internal/
│   ├── config/          # Configuration handling (from zoea-nova)
│   ├── constants/       # Application constants
│   ├── mcp/            # MCP client and proxy (from zoea-nova)
│   ├── provider/       # LLM provider abstraction (from zoea-nova)
│   ├── store/          # SQLite persistence for conversation history
│   └── styles/         # UI styling with brand colors
├── assets/
│   └── logos/          # Branding assets (from zoea-nova)
├── documentation/
│   ├── architecture/   # Architecture documentation
│   └── guides/         # User guides
├── config.toml         # Configuration file (from zoea-nova)
├── go.mod              # Go module definition
├── Makefile            # Build automation
├── LICENSE             # License file
├── .gitignore          # Git ignore patterns
└── README.md           # Project documentation
```

## Reused Components from zoea-nova

### 1. MCP Package (`internal/mcp/`)
Complete MCP client implementation with:
- HTTP transport for Streamable HTTP MCP
- Session management
- Tool discovery and invocation
- Proxy architecture for local tools
- Retry logic and error handling

**Files:**
- `client.go` - MCP HTTP client
- `proxy.go` - MCP proxy with local tool support
- `stub.go` - Stub server for offline testing
- `tools.go` - Local tool implementations (credential management)
- `tools_test.go` - Unit tests for tool handlers
- `integration_test.go` - Full workflow tests with proxy
- Additional tests for all components

### 2. Provider Package (`internal/provider/`)
LLM provider abstraction with:
- OpenAI-compatible API client
- Ollama support
- Rate limiting
- Retry logic
- Tool calling support

**Files:**
- `provider.go` - Provider interface and registry
- `openai_common.go` - OpenAI-compatible base implementation
- `opencode.go` - OpenCode Zen provider
- `ollama.go` - Ollama provider
- `factory.go` - Provider factory pattern
- Tests for all providers

### 3. Config Package (`internal/config/`)
Configuration management with:
- TOML config file parsing
- Credentials handling
- Data directory management (`~/.config/mysis`)
- Provider configuration

**Files:**
- `config.go` - Config structures and loading
- `credentials.go` - Secure credential handling
- Tests

### 4. Store Package (`internal/store/`)
SQLite persistence layer for conversation history and credentials:
- Session management (create, resume, list)
- Message storage with tool calls and reasoning
- Multi-instance support (no locking conflicts)
- WAL mode for concurrent access
- Session-scoped credential storage

**Database Schema:**
- `sessions` - Conversation sessions with provider info
- `messages` - Message history per session
- `session_credentials` - Game credentials per session (foreign key to sessions)
- Stored at `~/.config/mysis/mysis.db`

**Files:**
- `store.go` - Database operations and schema
- `credentials_test.go` - Credential storage tests

### 5. Brand Assets
- Logo files (SVG format)
- Brand colors: #9D00FF (electric purple), #00FFCC (bright teal)
- Style definitions in `internal/styles/styles.go`

### 6. Configuration
- `config.toml` with provider definitions
- Support for Ollama (local) and OpenCode Zen (cloud)
- MCP upstream configuration

## Technology Stack

Core dependencies:
- **Go 1.25.6** - Programming language
- **SQLite 3** - Conversation persistence (`github.com/mattn/go-sqlite3`)
- **UUID** - Session IDs (`github.com/google/uuid`)
- **Lipgloss** - Terminal styling
- **go-openai** - OpenAI-compatible API client (`github.com/sashabaranov/go-openai`)
- **zerolog** - Structured logging
- **TOML** - Configuration format (`github.com/BurntSushi/toml`)

## Brand Colors

From zoea-nova design system:
- **Primary (Brand Purple):** #9D00FF
- **Secondary (Teal):** #00FFCC
- **Background:** #08080F (deep space black)
- **Error:** #FF3366
- **Success:** #00FF66
- **Muted:** #5555AA

## Next Steps

This is a scaffold with reusable infrastructure. Implementation of application-specific features should:

1. Define the application's purpose and requirements
2. Design data models and state management
3. Implement core business logic
4. Build TUI components using the established brand styling
5. Add tests following the patterns from zoea-nova

## Data Storage

**Config Directory:** `~/.config/mysis/`
- `config.toml` - Configuration file (optional, falls back to `./config.toml`)
- `credentials.json` - API keys (mode 0600)
- `mysis.db` - SQLite database for conversation history and game credentials
- `account-backup.md` - Backup of SpaceMolt test accounts
- `credentials-backup-*.sql` - Timestamped credential backups

**Session Management:**
- Each CLI instance creates a unique session (UUID)
- Sessions can be named for easy resumption: `--session my-session`
- Anonymous sessions get auto-generated IDs
- Multiple instances can run concurrently without conflicts

**Database Features:**
- WAL mode for concurrent read/write
- Automatic schema initialization
- Full conversation history (user, assistant, tool messages)
- Tool calls stored as JSON
- Session metadata (provider, model, timestamps)
- Foreign key constraints enabled (CASCADE delete for credentials)

**Credential Management:**
- Session-scoped credential storage via MCP tools
- Agent can save/retrieve game credentials without user re-entry
- Credentials isolated by session (Session A cannot access Session B's credentials)
- Automatic cleanup when session is deleted (CASCADE)
- Tools: `save_credentials`, `get_credentials`
- Registered automatically in CLI on startup

## Local MCP Tools

The project includes local MCP tools registered with the proxy and available to the agent:

### Credential Management Tools

**`save_credentials`** - Saves username and password for the current session.
- **Input:** `{ "username": "string", "password": "string" }`
- **Response:** Success message with username confirmation
- **Validation:** Both fields required (non-empty)

**`get_credentials`** - Retrieves saved credentials for the current session.
- **Input:** Empty object `{}`
- **Response:** JSON with username/password, or message if none saved
- **Session isolation:** Each session can only access its own credentials

**Implementation:**
- Defined in `internal/mcp/tools.go`
- Registered automatically in `internal/cli/cli.go` on startup
- Session ID injected via closure (never exposed to agent)
- Full test coverage in `tools_test.go` and `integration_test.go`

**Backup/Restore:**
- `make backup-credentials` - Create timestamped SQL backup
- Restore: `sqlite3 ~/.config/mysis/mysis.db < backup-file.sql`

See `documentation/architecture/CREDENTIAL_TOOLS.md` for detailed implementation.

## Notes

- All import paths updated from `github.com/xonecas/zoea-nova` to `github.com/xonecas/mysis`
- The MCP client is configured to connect to `https://game.spacemolt.com/mcp`
- Provider configurations support both local (Ollama) and cloud (OpenCode Zen) models
- The project maintains the retro-futuristic aesthetic from zoea-nova
- Config directory changed from `~/.mysis` to `~/.config/mysis` (XDG Base Directory standard)
- Foreign key constraints enabled in SQLite for referential integrity
