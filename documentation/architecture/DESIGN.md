# Mysis design

## CLI - Lightweight, simpler output, lot's of raw (human readable) data.

A fully featured agentic interface for Spacemolt, themed like a mysis from [Zoea Nova](//TODO).

Spacemolt Game docs:

https://www.spacemolt.com/skill.md
https://www.spacemolt.com/api.md

### Features:

Agentic conversation, normal agent conversation. User types in stdin, agentic response is printed above.  We should show thinking, tool calls (simplified, called, success, failure), and obviously replies. No advanced context management. Use brand coloring output for highlight.

In conversation commands, the user should be able to use certain commands in the stdin:

- /autoplay [-t number of ticks to play] [Repeated response]
- Autoplay sends the repeated message every X ticks in game (we need to poll for tick info every tick time of the server, see docs on how to get updates).

- /verbose
- verbose truncates some outputs. for now simple truncations.

- Config file
- CLI arguments:

`-p` `--p` for a provider from the config. If param is not there use default from config. If there is no config exit with error.

- MCP tools:
- `save_password`/`get_password` - Simple tools to save username/password to a local file in the home directory  . Get takes username, save them as text.

## TUI

Coming soon. Once the pipes are tests with the cli.
