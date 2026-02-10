# Mysis

Single-agent CLI for playing SpaceMolt via MCP. Go 1.24.2, OpenAI-compatible LLM providers.

## Architecture Overview

**Simple CLI loop**: User types commands → LLM processes → Tools execute → Response displayed.

The agent maintains conversation history and calls SpaceMolt game tools (login, navigate, mine, trade, etc.) via MCP proxy.

## Core Components

**Config** (`internal/config/`): TOML file loading, provider configuration, credential management. Data directory: `~/.mysis`.

**Provider** (`internal/provider/`): OpenAI-compatible LLM client interface. Supports Ollama (local) and OpenCode Zen (cloud). Factory pattern for creating providers with model/temperature params.

**MCP** (`internal/mcp/`): Proxy between agent and SpaceMolt game server. Combines local tool handlers with upstream MCP tools. Automatic retry with rate limit respect (2s, 5s, 10s delays). Parses `Retry-After` headers.

**Styles** (`internal/styles/`): Brand colors for terminal output.

## Implementation Status

**Working pipes**:
- Config loading from TOML
- Multiple LLM provider support (Ollama, OpenCode Zen)
- MCP client with upstream game server connection
- Tool call retry with rate limiting
- Credential management (JSON with 0600 permissions)

**Not implemented**:
- CLI conversation loop
- Message history storage
- Agent state management
- Terminal UI rendering

## Design Decisions

**Single agent focus**: No multi-agent coordination, no account pooling, no broadcast messaging. One user, one agent, direct communication.

**MCP proxy simplicity**: Pass-through to upstream tools with retry logic. Local tool registration available but unused initially.

**Config-driven**: Provider selection via config file. No runtime provider switching.

**Data directory**: `~/.mysis` for config, credentials, future message history.

## Where to Find Things

- Config: `config.toml` and `~/.mysis/`
- Credentials: `~/.mysis/credentials.json` (auto-created with 0600)
- Logs: stderr (zerolog)
- Main entry: `cmd/mysis/main.go`
- Provider registry: `internal/provider/registry.go`
- MCP proxy: `internal/mcp/proxy.go`
- Game client: `internal/mcp/client.go`

## Agent Rules

**Do not edit `documentation/architecture/DESIGN.md` unless explicitly instructed by the user.** This file contains the user's design vision and should only be modified when directly requested.

## Next Steps

Implement CLI conversation loop in `cmd/mysis/main.go`:
1. Load config and create provider
2. Initialize MCP proxy
3. Read user input from stdin
4. Build LLM messages from history
5. Call LLM with available tools
6. Execute tool calls via MCP
7. Display response
8. Store in history
9. Loop

See `documentation/architecture/DESIGN.md` for CLI feature spec.
