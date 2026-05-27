# Claude Bar

A lightweight Windows desktop utility bar that displays Claude Code usage across multiple accounts with a retro terminal aesthetic.

```
[ CLAUDE ] │ ACCT: MAIN │ USAGE: 71% ███████░░░ │ RESET: 4D 18H │ MODEL: OPUS │ TODAY: 42 │ STATUS: ONLINE ▋   ◀ MAX ▶
```

Always-on-top · Frameless · Multi-monitor · System tray · Click-through mode · ~30 MB RAM

---

## Features

- **Billing period usage** — token count vs configurable limit, shown as % with progress bar
- **Reset countdown** — days/hours until next billing reset (configurable reset day)
- **Primary model** — OPUS / SONNET / HAIKU (most used this period)
- **Session status** — ONLINE (active < 5 min), IDLE (< 30 min), OFFLINE
- **Today's messages** — message count from today's activity
- **Multi-account** — cycle with `◀ ▶` buttons or tray menu; each account isolated
- **Multi-monitor** — move bar between screens via tray menu or `Ctrl+Alt+M`
- **Click-through mode** — toggle via tray or `Ctrl+Alt+T`
- **Retro terminal aesthetic** — black background, green phosphor glow, CRT scanlines, blinking cursor
- **Orange warning flash** at ≥90% usage

---

## Quick Start

### Prerequisites

- Windows 10/11 x64
- [Microsoft Edge WebView2 Runtime](https://go.microsoft.com/fwlink/p/?LinkId=2124703) (pre-installed on Win11)
- Go 1.21+ — https://go.dev/dl/
- Wails v2 CLI — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Node.js 18+ — https://nodejs.org/

### Dev (hot reload)

```powershell
cd claudebar
wails dev
```

### Build production EXE

```powershell
wails build -platform windows/amd64 -ldflags "-X main.Version=1.0.0"
# Output: build\bin\ClaudeBar.exe
```

Double-click `ClaudeBar.exe`. Bar appears at the top of the primary monitor.
Right-click the system tray icon for the menu.

---

## Configuration

Config file: `%APPDATA%\ClaudeBar\config.json` (auto-created with defaults on first run)

```json
{
  "monitor": 0,
  "opacity": 0.92,
  "refreshSeconds": 15,
  "weeklyTokenLimit": 20000000,
  "billingResetDay": 1,
  "barHeight": 28,
  "activeAccount": 0,
  "accounts": [
    { "name": "main", "path": "C:\\Users\\USER\\.claude" },
    { "name": "alt",  "path": "C:\\Users\\USER\\.claude-alt" }
  ],
  "startWithWindows": false,
  "clickThrough": false
}
```

| Field | Description |
|---|---|
| `weeklyTokenLimit` | Denominator for usage % (tokens per billing period). Default 20M. |
| `billingResetDay` | Day-of-month your billing resets (1 = 1st of month) |
| `barHeight` | Pixel height of the bar |
| `refreshSeconds` | Re-read interval for Claude data files |

---

## Data Sources (read-only)

| File | Used for |
|---|---|
| `{account}/stats-cache.json` | Daily token counts, message counts, model usage |
| `{account}/.credentials.json` | Subscription type (max/pro) |
| `{account}/sessions/*.json` | Session status, last activity time |
| `{account}/config/notification_states.json` | Limit-exceeded flag |

**Usage % =** tokens since `billingResetDay` ÷ `weeklyTokenLimit`

---

## System Tray Menu

```
Claude Bar v1.0.0
──────────────────
● Account: main       ← active account (clickable)
  Account: alt
──────────────────
● Monitor 1           ← active monitor (clickable)
  Monitor 2
──────────────────
  Click-through: OFF
  Start with Windows: OFF
──────────────────
  Quit
```

---

## Project Structure

```
claudebar/
├── main.go                    # Wails bootstrap + embed directives
├── app.go                     # App struct + all Wails-exported bindings
├── internal/
│   ├── config/
│   │   ├── config.go          # Config struct, Load/Save (atomic), Defaults
│   │   └── startup.go         # HKCU registry for start-with-Windows
│   ├── claude/
│   │   ├── types.go           # StatsCache, Credentials, SessionFile structs
│   │   ├── reader.go          # Read Claude JSON files (read-only)
│   │   └── stats.go           # ComputeBarData → BarData display payload
│   ├── syswin/
│   │   ├── monitor.go         # EnumDisplayMonitors → []MonitorInfo
│   │   └── window.go          # Win32: always-on-top, dock, click-through, opacity
│   └── tray/
│       └── tray.go            # System tray via getlantern/systray
└── frontend/
    ├── index.html
    └── src/
        ├── style.css          # Black bg, green glow, CRT scanlines, blink
        └── main.js            # Poll GetBarData(), update DOM
```

---

## Architecture Notes

**Go + Wails v2** wraps Windows' built-in WebView2 (Edge-based) rather than bundling Chromium — 11 MB EXE vs ~150 MB for Electron.

Windows integration (always-on-top, frameless, click-through, multi-monitor dock) uses direct Win32 syscalls via `golang.org/x/sys/windows`. No CGo, no C bindings.

The retro aesthetic (scanlines, text glow, blink) is pure CSS — no canvas or WebGL.

All file access is read-only. Auth tokens in `.credentials.json` are never used beyond reading `subscriptionType`.

---

## Future Ideas

- GPU / CPU widgets via `pdh.dll`
- Spotify now-playing via Web API
- GitHub unread notification count
- 14-day token sparkline (Canvas API)
- Configurable widget order (drag-to-reorder)
- Amber / cyan / white theme variants
