# wowclaude

Talk to your local Claude Code sessions from inside World of Warcraft Classic.

Two parts:

- `addon/WoWClaude/` - a WoW addon. A movable window with your session list on the
  left and a chat on the right. `/claude` toggles it.
- `wowclaude` - a Go daemon on the same machine. It runs `claude -p` and bridges
  the two files the game can touch.

## How the bridge works

WoW addon Lua has no sockets, no `io`, no `os` and cannot start a process. The
only channels are files, and the game only reads and writes them on `/reload`.

```
in game                          on disk                            daemon
--------                         -------                            ------
Send  ->  SavedVariables/WoWClaude.lua  (game writes on ReloadUI)  ->  poll mtime
                                                                       claude -p ...
Refresh <- Interface/AddOns/WoWClaude/Inbox.lua (game reads on ReloadUI) <- atomic write
```

The `Send` button queues the prompt in `WoWClaudeDB.outbox` and calls
`ReloadUI()`, which flushes SavedVariables. The daemon runs
`claude -p <prompt> [--resume <id>] --output-format stream-json`, streams partial
text into `Inbox.lua` once a second, then rescans `~/.claude/projects` and writes
the full session list with the last 40 turns of each. `Refresh` is a second
`ReloadUI()` that pulls the answer in.

Two reloads per turn is the floor. `C_UI.Reload` needs a hardware event, so the
addon cannot reload on a timer, and there is no live inbound channel.

## Install

```
make build
./bin/wowclaude install --wow-dir "/Applications/World of Warcraft/_classic_era_"
./bin/wowclaude serve   --wow-dir "/Applications/World of Warcraft/_classic_era_" --cwd ~/prg/repos/myproject
```

Windows: `--wow-dir "C:\Program Files (x86)\World of Warcraft\_classic_era_"`.

`serve` flags: `--account` (only when `WTF/Account` has several folders),
`--claude` (binary, default `claude` on PATH), `--cwd` (working directory for new
chats), `--permission-mode` (default `acceptEdits`; print mode cannot answer a
permission prompt, so never use `default`), `--state` (default `~/.wowclaude/state.json`).

## In-game checklist

1. Install the addon with the game closed the first time. `Inbox.lua` must exist
   before the game starts; on macOS `/reload` does not detect new files, only
   changed ones.
2. Start `wowclaude serve` before you log in.
3. Log in. If the addon shows as out of date, run
   `/dump select(4, GetBuildInfo())` and put that number in `## Interface:` in
   `WoWClaude.toc`.
4. `/claude` opens the window. Pick a session or `+ New chat`, type, press Enter
   or `Send`. The UI reloads.
5. Wait for the daemon (its log prints `request N done`), then press `Refresh`.
6. `/claude cwd /path/to/repo` sets the working directory for new chats.

## Tests

```
make check        # go test, go vet, luajit addon tests
```

The Go side is unit tested, including a Lua round trip of `Inbox.lua` through
gopher-lua. The addon's pure logic (`Core.lua`) runs under luajit, which is Lua
5.1 like the game. `UI.lua` is only compile-checked; the frames need the client.
