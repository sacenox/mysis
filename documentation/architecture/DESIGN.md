# Mysis Design

## CLI - Lightweight, simpler output, lot's of raw (human readable) data

A fully featured agentic interface for Spacemolt, themed like a mysis from [Zoea Nova](//TODO).

**Spacemolt Game docs:**

- https://www.spacemolt.com/skill.md
- https://www.spacemolt.com/api.md

### Features

**Agentic conversation:** Normal agent conversation. User types in stdin, agentic response is printed above. We should show thinking, tool calls (simplified, called, success, failure), and obviously replies. No advanced context management. Use brand coloring output for highlight.

**In-conversation commands:**

- `/autoplay [Repeated response]`
  - Autoplay sends the repeated message every `game tick time * max tool calls * .75` seconds

- `/verbose` ** NOT IMPLEMENTED YET **
  - Verbose truncates some outputs. For now simple truncations

**Config file and CLI arguments:**

- `-p` `--p` for a provider from the config. If param is not there use default from config. If there is no config exit with error
- Session management `-s`

**MCP tools:**

- `save_credentials(username, password)` / `get_credentials` - Simple tools to save username/password to a local sqlite database in the config file folder.
- The session id for the mysis saving the pair, we inject, **DO NOT MAKE IT AN ARGUMENT FOR THE AGENTS**

# TUI - Not a replacement, an augmentation.

Same UI as CLI: simple, with a permanent input line and the conversation log.

- Simple drawing:

```
┌─────────────────────────────────┐
│                                 │
│    Conversation Log             │
│    (scrollable viewport)        │
│                                 │
│ I'm going to mine now!          │
│ [00:00:00] Agent                │
│                                 │
│ Ok! Tell me when you're         │
│ you are done!                   │
│ [00:00:00] You                  │
│                                 │
├─────────────────────────────────┤
│ > Input line___                 │
├─────────────────────────────────┤
│ [A][I][W][E] Status text [L][M] │
└─────────────────────────────────┘
```

Uses our colors from internal/style.
Elegant but retro-futuristic in the same style as `assets/logo.svg`.
One background color: deep purple (ColorBg or ColorBgAlt, to be tested with both).

### Log:

- Same conversation log, no log messages. Pure agent conversation with truncated reasoning shown.
  - Reasoning is truncated from the end, showing the last portion of the text.
- No word wrap for reasoning, user messages, or agent replies.
- The rest (tools, tool calls) is truncated like CLI now.

### Input:

- Permanent input box with same functionality as CLI stdin currently.
- Commands: `/autoplay` `exit` `quit`

- Adds keybinds:
  - `ESC` stop autoplay if autoplay is on

- Up/down arrows: message history
- Left/right: edit the current line

### Status:

- Shows a series of animated icons:
  - [E]: When an error is logged, animate the error icon and show error text (only system errors, not in-game).
  - [W]: When a warning is logged, animate the warning icon and show warning text.
  - [I]: When info is logged, nimate the info icon.
  - [A]: When autoplay is enabled, shows the truncated auto message.

- Shows the text related to the animated icon from above.

- Shows connection status with animated icons for LLM and MCP connections.
  - Animate when a request is fired.

Animation:

- items are always on the "empty" or "thinest" frame.
- When an event happens, the animation starts becomes quick for a few iterations, slowing down back to normal and stop.
- Subsequent events reset the slowdown.

Animation Sequences:

- Autoplay: ◔ ◑ ◕ ● → ends at ◔ (quarter circle)
- Info: ● ◉ ◎ ○ ◌ → ends at ◌ (empty circle)
- Warning: ◆ ◈ ◇ → ends at ◇ (hollow diamond)
- Error: ✖ ✕ → ends at (empty/space)
- LLM: ◉ ◎ ○ ◌ → ends at ◌ (empty circle)
- MCP: ◐ ◓ ◑ ◒ → ends at ○ (circle/dot)

#### Bonus Features (Not in Spec originally)

Implemented beyond specification:

- History compression (saves tokens)
- TUI mode (1,670 lines - complete terminal UI)
- List sessions (--list-sessions)
- Delete sessions (--delete-session)
- System prompt from file (-f/--file)
- Debug logging (-d/--debug)

# TODO:

- blinking cursor?
- Session setup screen
- Game state sidebar, pretty, game like, retro.
