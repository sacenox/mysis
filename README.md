# Mysis

A terminal companion for piloting a single AI agent through the SpaceMolt universe.

## Overview

Mysis is your personal mission control for commanding one AI spaceship captain in [SpaceMolt](https://www.spacemolt.com). Built with Go and Bubble Tea, it connects a single LLM-powered agent to the game via Model Context Protocol, letting you guide your captain through the cosmos with natural language commands.

Where Zoea-Nova orchestrates swarms, Mysis focuses on the intimate bond between commander and captain. One agent, one mission, full control.

## Features

- **Single-Agent Focus**: Direct communication with your AI captain
- **Terminal Interface**: Clean TUI built with Bubble Tea
- **Flexible LLM Support**: Local Ollama or remote OpenCode Zen models
- **MCP Integration**: Native SpaceMolt game connection via Model Context Protocol
- **Real-time Interaction**: Watch your agent think, decide, and act

## Requirements

**Terminal:**

- Minimum size: 80 columns × 20 lines
- TrueColor support recommended (24-bit RGB)
- Unicode font (Nerd Font or Unicode-compatible font)

**Recommended Terminals:**

- Alacritty, Kitty, WezTerm, Ghostty (best compatibility)
- iTerm2 (macOS), Windows Terminal (with Nerd Font)

## Try it

```sh
make run          # Build and start
make install      # Install to ~/.config/mysis/bin/mysis
./bin/mysis        # Run directly

or

./bin/mysis -debug # With debug logging
```

## CLI Flags

- `--config <path>` - Path to config file (default: `./config.toml` or `~/.config/mysis/config.toml`)
- `--debug` - Enable debug logging

## Configuration

Edit `config.toml` to configure:

- LLM providers (Ollama, OpenCode Zen)
- MCP upstream endpoint
- Model selection and temperature

See `config.toml` for details.

## Technology Stack

- **Language:** Go 1.24.2
- **TUI:** Bubble Tea (charmbracelet/bubbletea)
- **HTTP:** go-openai for OpenAI-compatible APIs
- **Logging:** zerolog
- **Config:** TOML

## License

See LICENSE file.
