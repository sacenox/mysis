# Project info for agents:

Design is at `documentation/architecture/DESIGN.md`. This is the USER's vision and is **only to be editted when explicitly asked**.

If the code/comment/other document is contradictory to the design document stop and warn the USER.

Use YAGNI **always**. When coding, when writting documentation.

Keep documentation concise and factual. No documentation file should be more than 500 lines, most should be at around ~200.

## Quick reference:

- The config, credentials, and database are in the app's data folder: `~/.config/mysis`.
- The app config is in `./config.toml`.
- `cmd/main.go` is the orchestrator and entrypoint. It can then run in interactive mode with TUI or non-interactive mode with cli.
- Project has a strict rule of separation of concerns. Shared functionality and data access is shared no matter what display mode the app is using. Ex: `llm/loop.go` is one core functionaly shared by both displays.
