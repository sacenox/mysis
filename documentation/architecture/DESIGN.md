# Mysis Design

## CLI - Lightweight, simpler output, lot's of raw (human readable) data

A fully featured agentic interface for Spacemolt, themed like a mysis from [Zoea Nova](//TODO).

**Spacemolt Game docs:**
- https://www.spacemolt.com/skill.md
- https://www.spacemolt.com/api.md

### Features

**Agentic conversation:** Normal agent conversation. User types in stdin, agentic response is printed above. We should show thinking, tool calls (simplified, called, success, failure), and obviously replies. No advanced context management. Use brand coloring output for highlight.

**In-conversation commands:**

- `/autoplay [-t number of ticks to play] [Repeated response]`
  - Autoplay sends the repeated message every X ticks in game (we need to poll for tick info every tick time of the server, see docs on how to get updates)

- `/verbose`
  - Verbose truncates some outputs. For now simple truncations

**Config file and CLI arguments:**

- `-p` `--p` for a provider from the config. If param is not there use default from config. If there is no config exit with error
- Session management `-s`

**MCP tools:**

- `save_credentials(username, password)` / `get_credentials` - Simple tools to save username/password to a local file in the home directory. Get takes username, save them as text
- The session id for the mysis saving the pair, we inject, **DO NOT MAKE IT AN ARGUMENT FOR THE AGENTS**

**Basic controls on the input:**

- Up/down arrows: message history
- Left/right: edit the current line

## TUI

Coming soon. Once the pipes are tested with the CLI.
