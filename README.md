# 👾 ClaudePanel

A highly customized, lightweight desktop utility bar for **Windows, macOS, and Linux** that displays Claude Code usage across multiple accounts with a retro terminal aesthetic. Fully integrated with custom terminal themes, cross-platform monospace typography, and an interactive headless YouTube audio stream (Claude FM).

```
👾 │ ACCT: MAIN [PRO] │ WEEKLY: 71% ░░░░░░··· │ RESET: 4D 18H │ OPUS │ IDLE │ CLAUDE FM [ON] · VOL 120%
```

Always-on-top · Frameless · Multi-monitor Dock · Click-through mode · System tray · Multi-account · ~30 MB RAM

---

## 🖥️ Core Features

### Multi-account
Switch between any number of Claude accounts — each configured with a separate `~/.claude` path — via the system tray or Settings panel. Account names and the active selection persist across restarts.

### Multi-monitor docking
Pick which monitor the bar docks to at any time from the tray menu. On Windows, **AppBar mode** uses the `SHAppBarMessage` API to reserve screen space so maximized windows automatically tile below the bar. On Linux X11 it sets `_NET_WM_STRUT_PARTIAL` for compatible compositors. On macOS and Linux Wayland the bar floats at the topmost window level without reserving space.

### System tray
A resident tray icon gives one-click access to all controls without the bar needing to be in focus: switch account, switch monitor, toggle click-through, toggle start-on-login, and quit.

### Click-through mode
When enabled, mouse events pass straight through the bar to whatever is behind it — ideal for full-screen workflows. Toggle via the tray or Settings. (Linux Wayland: click-through is not yet implemented at v1.)

### Start on login
Registers ClaudePanel to launch at login via the OS-native mechanism: **Windows** Registry (`HKCU\...\Run`), **macOS** LaunchAgent plist, **Linux** XDG autostart `.desktop` file.

### Live token usage
Reads `~/.claude/rate_limits.json` (populated via `statuslineCommand`) every N seconds and shows weekly consumption as both a percentage and a shaded progress bar, with a reset countdown sourced from the Claude API.

---

## 🎨 Premium Visual Features

### 1. Monospace Typography & TUI Glyphs
- **Developer-first font stack**: Prefers **Cascadia Mono / Cascadia Code** (Windows Terminal), **SF Mono** (macOS), **Menlo**, **Fira Code**, **JetBrains Mono**, **DejaVu Sans Mono**, and **Inconsolata** — picks whatever monospace font your system already has installed.
- **Dynamic Shaded Progress Blocks**: Displays weekly and hourly usage via retro character tiles (`░▒▓█`) representing your current warning tier. Unused cells are faint terminal middle dots (`·`).

### 2. Retro Theme Engine
Cycle between five distinct CRT-scanline-filtered presets on-the-fly:
- 🔸 **CLAUDE (Default)**: Flat CLI style featuring signature terracotta orange accents (`#d77757`), lavender badges (`#b1b9f9`), and clean white headers.
- 🟢 **FALLOUT**: Iconic Pip-Boy green HUD with outline progress bar brackets (`____]`), a solid 1px-higher flush green fill, glowing CRT raster scanlines, and high-readability dimmed green labels (`#2db32d`).
- 🟡 **AMBER**: DEC VT100 / Fallout NV terminal with beautiful glowing amber values and high-readability dimmed amber labels (`#b37b00`).
- 📟 **MATRIX**: Digital rain theme with sharp green characters, custom dividers, and a blinking green/yellow/red terminal block caret (`█`) synced to warning status.
- 😈 **DRACULA**: Sleek modern dark mode with cyan labels and pastel pink progress bars.

### 3. Headless Radio Player (Claude FM)
Listen to the iconic **Claude FM** Lo-Fi stream directly inside the bar without opening any browser windows:
- 📻 **Masked Marquee**: When playing, the label transforms into a smooth horizontal scrolling text: `NOW PLAYING CLAUDE FM · NOW PLAYING CLAUDE FM · `. This text is hidden beyond a fixed-width mask (`75px`), scrolling seamlessly without affecting surrounding elements. Reverts to `CLAUDE FM` instantly when paused.
- 🔊 **0% - 200% Volume Range**: Custom extended volume headroom. Linearly maps the UI's `0% - 200%` range to the YouTube player's default `0 - 100` volume range.
- 🎛️ **Dual Adjustments**:
  - **Scroll Wheel**: Hover and scroll up/down anywhere over the segment to adjust volume by precise `5%` increments.
  - **Click-Cycling**: Click on `VOL` or the volume number to cycle downwards in granular `10%` steps (e.g. `200% → 190% → 180% → ... → 0% → 200%`).
- 💾 **State Persistence**: Saves your preferred volume level and theme choice to `localStorage` to restore them automatically on boot.

### 4. Smart Status Overrides
- Dynamically translates dynamic and static `OFFLINE` indicators globally into a sleek active lavender **`IDLE`** status badge (`#b1b9f9`) to preserve your active CLI context.

---

## 🚀 Quick Start

Download the installer for your platform from the [Releases](../../releases/latest) page:

| Platform | File | Notes |
|---|---|---|
| Windows 10/11 x64 | `ClaudePanel-*-windows-amd64-setup.exe` | NSIS installer. Requires [WebView2 Runtime](https://go.microsoft.com/fwlink/p/?LinkId=2124703) (pre-installed on Win11). |
| macOS 10.13+ (Intel + Apple Silicon) | `ClaudePanel-*-macos-universal.pkg` | Double-click to install to `/Applications`. |
| Debian / Ubuntu | `claudepanel_*_amd64.deb` | `sudo apt install ./claudepanel_*_amd64.deb` |
| Fedora / RHEL | `claudepanel-*.x86_64.rpm` | `sudo dnf install ./claudepanel-*.x86_64.rpm` |
| Any Linux (portable) | `ClaudePanel-x86_64.AppImage` | `chmod +x ClaudePanel-x86_64.AppImage && ./ClaudePanel-x86_64.AppImage` |

The installers configure Claude Code's `statuslineCommand` automatically on install, and remove it on uninstall — no terminal commands needed. AppImage users see a one-time first-run prompt instead (no install hooks available).

### First-launch security warnings (unsigned v1)

Until code-signing certificates are in place, expect:
- **Windows** → SmartScreen "Windows protected your PC" → click *More info* → *Run anyway*
- **macOS** → "ClaudePanel cannot be opened because it is from an unidentified developer" → System Settings → Privacy & Security → *Open Anyway*, or right-click the .app → *Open*
- **Linux .deb/.rpm** → no warnings (root install)
- **AppImage** → no warnings (user-mode)

### Build from source

Requires Go 1.23+, Node.js 18+, Wails v2 CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`). On Linux also: `libgtk-3-dev`, `libwebkit2gtk-4.0-dev`, `wmctrl`, `x11-utils`, `xdotool`.

```bash
wails dev                                                       # hot-reload dev mode
wails build -platform windows/amd64 -nsis                        # Windows installer
wails build -platform darwin/universal                           # macOS .app (then see build/darwin/scripts for .pkg)
wails build -platform linux/amd64                                # Linux binary (then nfpm/AppImage via build/linux/*)
```

---

## 🔌 Live Usage Capturing (configured automatically by installers)

ClaudePanel reads live usage from `~/.claude/rate_limits.json`, populated by Claude Code via its `statuslineCommand` hook. The installers set this hook for you on install and clear it on uninstall by editing `~/.claude/settings.json` and adding (only) the `statuslineCommand` key — other keys are preserved.

If you built from source or are using the AppImage and want to configure the hook manually:

```bash
claude config set statuslineCommand "node -e \"const fs=require('fs');const p=require('path');const os=require('os');const d=fs.readFileSync(0,'utf-8');if(d){const parsed=JSON.parse(d);fs.writeFileSync(p.join(os.homedir(),'.claude','rate_limits.json'),JSON.stringify({...parsed,captured_at:Date.now()}))}\""
```

Every Claude prompt then writes a tiny JSON payload to `rate_limits.json`, which ClaudePanel picks up instantly.

---

## ⚙️ Configuration

Config file (auto-created on first run):

| Platform | Path |
|---|---|
| Windows | `%APPDATA%\ClaudePanel\config.json` |
| macOS | `~/Library/Application Support/ClaudePanel/config.json` |
| Linux | `$XDG_CONFIG_HOME/ClaudePanel/config.json` (fallback `~/.config/ClaudePanel/config.json`) |


```json
{
  "monitor": 0,
  "theme": "claude",
  "opacity": 0.92,
  "refreshSeconds": 15,
  "barHeight": 28,
  "appBarMode": true,
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
| `barHeight` | Pixel height of the bar |
| `refreshSeconds` | Re-read interval for Claude data files |
| `theme` | Visual theme: `claude`, `fallout`, `amber`, `matrix`, `dracula` |
| `appBarMode` | Reserve screen space so maximized windows tile below the bar (Windows/Linux X11 only) |

---

## 📁 Project Structure

```
claudepanel/
├── main.go                    # Wails bootstrap + embed directives
├── app.go                     # App struct + Wails-exported bindings
├── platform_info.go           # runtime.GOOS helper exposed to frontend
├── icon_{windows,darwin,linux}.go  # Per-OS tray icon embedding
├── internal/
│   ├── config/
│   │   ├── config.go              # Config struct, Load/Save, cross-platform AppDataDir
│   │   └── startup_{windows,darwin,linux}.go  # Per-OS autostart (registry / LaunchAgent / .desktop)
│   ├── claude/                    # Read Claude JSON files, compute BarData
│   ├── platform/                  # Per-OS window + monitor APIs
│   │   ├── window_{windows,darwin,linux}.go
│   │   └── monitor_{windows,darwin,linux}.go
│   └── tray/                      # System tray via getlantern/systray
├── frontend/                       # Wails webview UI
└── build/
    ├── windows/installer/          # NSIS template + statusline PowerShell script
    ├── darwin/scripts/             # pkgbuild postinstall/preuninstall bash scripts
    └── linux/                      # nfpm.yaml, .desktop, AppDir, AppRun, postinstall.sh
```

---

## ⚠️ Known limitations (v1 cross-platform)

- **Linux Wayland**: there is no portable Wayland protocol for "stay above other windows" or "reserve screen space". The bar appears but may not float above fullscreen apps. KWin honors the `_NET_WM_WINDOW_TYPE_DOCK` hint; GNOME/Mutter mostly ignores it; wlroots compositors (Hyprland, Sway) vary. X11 sessions work correctly.
- **Linux click-through**: not implemented at v1 (requires XShape extension bindings). The tray menu option is preserved but is a no-op on Linux for now.
- **macOS docking**: NSWindow at `NSStatusWindowLevel` floats above other windows but cannot reserve screen space the way the Windows AppBar API does. Maximized apps will draw beneath the bar — accepted as macOS-native behavior.
- **macOS Gatekeeper** (unsigned v1): see *First-launch security warnings* in Quick Start.
- **Settings merge safety**: if `~/.claude/settings.json` already exists but contains invalid JSON, the installer logs a warning and skips the modification rather than overwriting the file.

---

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.
