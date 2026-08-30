# wowclaude

Talk to your local Claude Code sessions from inside World of Warcraft. Works on
vanilla 1.12 clients (TurtleWoW, OctoWoW, Lua 5.0) and on Classic Era (Lua 5.1).

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

## Supported clients

| Client | Folder to pass as `--wow-dir` | `.toc` the game reads |
|---|---|---|
| Vanilla 1.12 (TurtleWoW, OctoWoW) | the folder that holds `WoW.exe` and `WTF\`, e.g. `D:\games\OctoWow` | `WoWClaude.toc` (`## Interface: 11200`) |
| Classic Era / Anniversary | `_classic_era_`, e.g. `/Applications/World of Warcraft/_classic_era_` | `WoWClaude_Vanilla.toc` (`## Interface: 11509`) |

The addon code is one set of files for both. It avoids everything Lua 5.0 lacks
(`#`, `string.match`, varargs at file scope) and reads handler arguments from
`this`/`arg1` when the client passes none.

## Install

Build once, then point `install` and `serve` at the game folder for your client.

```
make build
```

Vanilla 1.12 on Windows, for example OctoWoW:

```
./bin/wowclaude.exe install --wow-dir "D:\games\OctoWow"
./bin/wowclaude.exe serve   --wow-dir "D:\games\OctoWow" --cwd "C:\Users\you\repos\myproject" --claude "C:\Users\you\AppData\Roaming\Claude\claude-code\<version>\claude.exe"
```

Classic Era on macOS:

```
./bin/wowclaude install --wow-dir "/Applications/World of Warcraft/_classic_era_"
./bin/wowclaude serve   --wow-dir "/Applications/World of Warcraft/_classic_era_" --cwd ~/repos/myproject
```

Classic Era on Windows: `--wow-dir "C:\Program Files (x86)\World of Warcraft\_classic_era_"`.

`serve` flags: `--account` (only when `WTF/Account` has several folders),
`--claude` (binary; when omitted the daemon uses `claude` on PATH, then the newest
`Claude\claude-code\<version>\claude.exe` under `%APPDATA%`, so app updates do not break it), `--cwd` (working
directory for new chats; pick a narrow one, prompts auto-accept edits there),
`--permission-mode` (default `acceptEdits`; print mode cannot answer a permission
prompt, so never use `default`), `--state` (default `~/.wowclaude/state.json`).

## In-game checklist

1. Install the addon with the game closed the first time. `Inbox.lua` must exist
   before the game starts; on macOS `/reload` does not detect new files, only
   changed ones.
2. Start `wowclaude serve` before you log in. If it reports 0 account folders,
   log in once, log out, and start it again.
3. Log in. If the addon shows as out of date, tick "Load out of date AddOns".
   On Classic Era you can also put the current build's interface number in
   `WoWClaude_Vanilla.toc`; on 1.12 the number is always `11200`.
4. `/claude` opens the window. Pick a session or `+ New chat`, type, press Enter
   or `Send`. The UI reloads.
5. Wait for the daemon (its log prints `request N done`), then press `Refresh`.
6. `/claude cwd /path/to/repo` sets the working directory for new chats.

If a Lua error appears, run `/console scriptErrors 1` (1.12: `/script SetCVar("scriptErrors", 1)`)
and report the message.

## Tests

```
make check        # go test, go vet, luajit addon tests
```

The Go side is unit tested, including a Lua round trip of `Inbox.lua` through
gopher-lua. The addon's pure logic (`Core.lua`) runs under luajit (Lua 5.1). Lua 5.0
compatibility is enforced by a grep in `make check`, not by a 5.0 interpreter. `UI.lua` is only compile-checked; the frames need the client.
