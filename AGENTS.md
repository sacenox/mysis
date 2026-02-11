# Mysis

Single-agent CLI for playing SpaceMolt via MCP. Go 1.24.2, OpenAI-compatible LLM providers.

## Architecture Overview

**Simple CLI loop**: User types commands → LLM processes → Tools execute → Response displayed.

The agent maintains conversation history and calls SpaceMolt game tools (login, navigate, mine, trade, etc.) via MCP proxy.

## Core Components

**Config** (`internal/config/`): TOML file loading, provider configuration, credential management. Data directory: `~/.config/mysis`.

**Provider** (`internal/provider/`): OpenAI-compatible LLM client interface. Supports Ollama (local) and OpenCode Zen (cloud). Factory pattern for creating providers with model/temperature params.

**MCP** (`internal/mcp/`): Proxy between agent and SpaceMolt game server. Combines local tool handlers with upstream MCP tools. Automatic retry with rate limit respect (2s, 5s, 10s delays). Parses `Retry-After` headers. Includes credential management tools (`save_credentials`, `get_credentials`) with session-scoped storage.

**Styles** (`internal/styles/`): Brand colors for terminal output (electric purple, bright teal, deep space backgrounds).

**LLM Loop** (`internal/llm/`): Multi-round tool calling with reasoning display, streaming support, and history compression.

**Session** (`internal/session/`): Session creation/resumption, provider selection, history loading.

**Store** (`internal/store/`): SQLite-based message history with WAL mode, foreign key constraints, and tool call serialization.

**CLI** (`internal/cli/`): Interactive terminal interface with stdin input, autoplay support, and branded output formatting.

**TUI** (`internal/tui/`): Bubbletea-based terminal UI with scrollable conversation log, animated status bar, and concurrent message processing.

## Implementation Status

**Fully implemented**:

- Config loading from TOML with default provider support
- Multiple LLM provider support (Ollama, OpenCode Zen)
- MCP client with upstream game server connection
- Tool call retry with rate limiting
- Credential management (session-scoped, SQLite storage)
- CLI conversation loop with stdin input
- Message history storage (SQLite with compression)
- Agent state management
- Terminal UI (TUI mode with bubbletea)
- Autoplay command (`/autoplay [message]`)
- Session management (create, resume, list, delete)
- Branded output with reasoning and tool call display

## Design Decisions

**MCP proxy simplicity**: Pass-through to upstream tools with retry logic. Local tool registration available but unused initially. ** DO NOT CHANGE UPSTREAM RESPONSES **

**Config-driven**: Provider selection via config file. No runtime provider switching.

**Data directory**: `~/.config/mysis` for config, credentials, database (sessions, messages, game credentials).

## Where to Find Things

- Config: `config.toml` and `~/.config/mysis/`
- Database: `~/.config/mysis/mysis.db` (SQLite with sessions, messages, credentials)
- Credentials (LLM providers): `~/.config/mysis/credentials.json` (auto-created with 0600)
- Logs: stderr (zerolog)
- Main entry: `cmd/mysis/main.go`
- Provider registry: `internal/provider/registry.go`
- MCP proxy: `internal/mcp/proxy.go`
- Game client: `internal/mcp/client.go`
- LLM loop: `internal/llm/loop.go`
- CLI: `internal/cli/cli.go`
- TUI: `internal/tui/app.go`

## Agent Rules

**Do not edit `documentation/architecture/DESIGN.md` unless explicitly instructed by the user.** This file contains the user's design vision and should only be modified when directly requested.
