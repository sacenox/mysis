# TODO:

- Session lost not refreshing with login:

```
2:42AM INF OpenCode request started message_count=65 model=gpt-5-nano provider=zen-nano tool_count=121
2:43AM INF OpenCode request successful attempt=1 choice_count=1 message_count=65 model=gpt-5-nano provider=zen-nano status=200
⚙ get_cargo{"session_id":"9f3a930758852cb5ec92b254a36930fb"} ✓
  {
  "available": 36,
  "capacity": 50,
  "cargo": [
    {
      "item_id": "ore_nickel",
      "n...
2:43AM INF OpenCode request started message_count=67 model=gpt-5-nano provider=zen-nano tool_count=121
2:43AM INF OpenCode request successful attempt=1 choice_count=1 message_count=67 model=gpt-5-nano provider=zen-nano status=200
Mining continues.
- Cargo: 14/50 (Nickel 4, Iron 6, Copper 3, Trade Crystal 1)
- Space left: 36
- Location: Commerce Fields (haven_belt)

Next: keep mining Iron Ore (priority) and return to Grand Exchange when near full (or set auto-return threshold).
Would you like me to scan nearby players or deploy drones before finishing?

─── Autoplay Turn ───
> Mine away!
2:43AM INF OpenCode request started message_count=69 model=gpt-5-nano provider=zen-nano tool_count=121
2:44AM INF OpenCode request successful attempt=1 choice_count=1 message_count=69 model=gpt-5-nano provider=zen-nano status=200
⚙ mine{"session_id":"9f3a930758852cb5ec92b254a36930fb"} ✗
  Error: Session not initialized
2:44AM INF OpenCode request started message_count=71 model=gpt-5-nano provider=zen-nano tool_count=121
2:44AM INF OpenCode request successful attempt=1 choice_count=1 message_count=71 model=gpt-5-nano provider=zen-nano status=200
⚙ login{"password":"24ff43c10286496d746dc5934f750e767a3c9518e46c... ✗
  Error: Session not initialized
```

- Print welcome to the TUI on start
- Blinking cursor in TUI
- Mouse selection in TUI
- User messages no visible in TUI
- Remove "All systems operational" with the dotted infinite loader icon when a message is sending.
