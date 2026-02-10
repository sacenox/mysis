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
- `tools.go` - Tool definitions and helpers
- Tests for all components

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
- Data directory management
- Provider configuration

**Files:**
- `config.go` - Config structures and loading
- `credentials.go` - Secure credential handling
- Tests

### 4. Brand Assets
- Logo files (SVG format)
- Brand colors: #9D00FF (electric purple), #00FFCC (bright teal)
- Style definitions in `internal/styles/styles.go`

### 5. Configuration
- `config.toml` with provider definitions
- Support for Ollama (local) and OpenCode Zen (cloud)
- MCP upstream configuration

## Technology Stack

Inherited from zoea-nova:
- **Go 1.24.2** - Programming language
- **Bubble Tea** - TUI framework (dependency ready)
- **Lipgloss** - Terminal styling
- **go-openai** - OpenAI-compatible API client
- **zerolog** - Structured logging
- **TOML** - Configuration format

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

## Notes

- All import paths updated from `github.com/xonecas/zoea-nova` to `github.com/xonecas/mysis`
- The MCP client is configured to connect to `https://game.spacemolt.com/mcp`
- Provider configurations support both local (Ollama) and cloud (OpenCode Zen) models
- The project maintains the retro-futuristic aesthetic from zoea-nova
