// Package terminal opens a new, visible terminal window running a command
// (default `claude`) in a chosen directory, with a name as the window/tab
// title and a color applied. It is the deliberate inverse of internal/audio,
// which spawns a HIDDEN helper: here the spawned process must be VISIBLE and
// must outlive ClaudePanel (detached, never Wait()ed).
//
// Color is conveyed uniformly across every OS by prepending the nearest
// colored-circle emoji to the title (e.g. "🔵 CRM"), so it reads in the tab
// strip, taskbar and Alt-Tab switcher. No terminal uses a native tab-color flag.
//
// This file is shared across OSes (no build tag) and is pure/unit-testable
// except for Launch. The per-OS preset tables, default detection, and
// process-detach attributes live in launch_{windows,darwin,linux}.go.
package terminal

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"claudepanel/internal/config"
)

// Preset describes how to invoke one terminal program. Args are split around an
// optional color-args slice so a preset that DOES take native color flags can
// have them cleanly dropped when the entry has no color. No builtin currently
// uses this (color rides in the title as an emoji dot), but the seam is kept.
//
// Placeholders substituted per-argv-element (never re-split after substitution):
//
//	{title} – the entry name, with the emoji dot prepended when DotInTitle
//	{color} – "#RRGGBB"
//	{dot}   – nearest colored-circle emoji ("" when no color)
//	{dir}   – working directory (~ expanded)
//	{cmd}   – the command to run, after OSC-title / keep-open composition
type Preset struct {
	Key        string
	Exe        string
	PreColor   []string // template args before the color args
	ColorArgs  []string // spliced in only when entry.Color != ""
	PostColor  []string // template args after the color args
	DotInTitle bool     // prepend the nearest emoji dot to {title}
	NeedsOSC   bool     // emit an OSC title escape from inside the shell command
	DirInShell bool     // `cd <dir>` inside the shell command (no working-dir flag)
	Shell      string   // keep-open hint: "bash"/"sh" append `; exec bash`/`; exec sh`
	Quote      quoteMode
}

// PresetInfo is the lightweight view handed to the frontend dropdown.
type PresetInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	OS    string `json:"os"`
}

type quoteMode int

const (
	quoteNone quoteMode = iota // discrete argv element, no shell — passed literally
	quoteSh                    // POSIX shell single-quote
	quotePwsh                  // PowerShell single-quote
	quoteOsa                   // AppleScript double-quote
)

// presetLabels maps a preset key to its human label for the UI dropdown. It
// covers keys from every OS so the table is stable regardless of build target.
var presetLabels = map[string]string{
	"windows-terminal":    "Windows Terminal",
	"powershell":          "PowerShell",
	"cmd":                 "Command Prompt",
	"terminal-app":        "Terminal.app",
	"iterm2":              "iTerm2",
	"ghostty":             "Ghostty",
	"gnome-terminal":      "GNOME Terminal",
	"konsole":             "Konsole",
	"alacritty":           "Alacritty",
	"wezterm":             "WezTerm",
	"kitty":               "kitty",
	"xterm":               "xterm",
	"x-terminal-emulator": "Default (x-terminal-emulator)",
	"custom":              "Custom…",
}

func labelFor(key string) string {
	if l, ok := presetLabels[key]; ok {
		return l
	}
	return key
}

// Presets lists the builtin terminals for the current OS plus the custom
// escape hatch, for the bar's terminal-program dropdown.
func Presets() []PresetInfo {
	bp := builtinPresets()
	out := make([]PresetInfo, 0, len(bp)+1)
	for _, p := range bp {
		out = append(out, PresetInfo{Key: p.Key, Label: labelFor(p.Key), OS: runtime.GOOS})
	}
	out = append(out, PresetInfo{Key: "custom", Label: labelFor("custom"), OS: runtime.GOOS})
	return out
}

func resolvePreset(key string) (Preset, error) {
	for _, p := range builtinPresets() {
		if p.Key == key {
			return p, nil
		}
	}
	return Preset{}, fmt.Errorf("unknown terminal preset %q", key)
}

// subVals holds the already-computed placeholder values for one launch.
type subVals struct {
	title string // display title (emoji dot already prepended if applicable)
	dir   string
	color string
	dot   string
	cmd   string // fully composed command string
}

func quoteData(s string, q quoteMode) string {
	switch q {
	case quoteSh:
		return shquote(s)
	case quotePwsh:
		return pwshquote(s)
	case quoteOsa:
		return osaquote(s)
	default:
		return s // quoteNone — literal discrete argv element
	}
}

// subOne substitutes placeholders in a single template element. {title} and
// {dir} are data and are quoted per the preset's quote mode; {cmd} is executed
// verbatim (the user intentionally typed it); {color}/{dot} are a fixed-charset
// hex / emoji.
func subOne(e string, v subVals, q quoteMode) string {
	e = strings.ReplaceAll(e, "{title}", quoteData(v.title, q))
	e = strings.ReplaceAll(e, "{dir}", quoteData(v.dir, q))
	e = strings.ReplaceAll(e, "{color}", v.color)
	e = strings.ReplaceAll(e, "{dot}", v.dot)
	e = strings.ReplaceAll(e, "{cmd}", v.cmd)
	return e
}

func substitute(tmpl []string, v subVals, q quoteMode) []string {
	if len(tmpl) == 0 {
		return nil
	}
	out := make([]string, len(tmpl))
	for i, e := range tmpl {
		out[i] = subOne(e, v, q)
	}
	return out
}

// composeShellCmd builds the shell command string for shell-based presets:
// optional `cd`, optional OSC title escape, the user command, and an optional
// keep-open `exec` so the window stays after the command exits. It is pure so
// the OSC-injection / quoting behaviour can be tested on any OS.
func composeShellCmd(cmd, shell string, needsOSC, dirInShell bool, dir, title string) string {
	out := cmd
	switch shell {
	case "bash":
		out += "; exec bash"
	case "sh":
		out += "; exec sh"
	}
	if needsOSC {
		out = "printf '\\033]0;%s\\007' " + shquote(title) + "; " + out
	}
	if dirInShell && dir != "" {
		out = "cd " + shquote(dir) + "; " + out
	}
	return out
}

// build resolves the launcher to an exe + argv. It is pure (no process is
// started) so argv/quoting can be asserted in tests. sublabel is an optional
// per-launch suffix appended to the title (e.g. "CRM · backend") so several
// terminals opened from one entry can be told apart; it never touches config.
func build(entry config.TerminalConfig, launcher config.LauncherConfig, sublabel string) (string, []string, error) {
	color := strings.TrimSpace(entry.Color)
	dot := nearestDot(color)
	dir := resolveDir(entry.Dir)
	label := entry.Name
	if sub := strings.TrimSpace(sublabel); sub != "" {
		label = entry.Name + " · " + sub
	}
	cmd := strings.TrimSpace(entry.Command)
	if cmd == "" {
		cmd = "claude"
	}

	// Flat-template mode: the custom escape hatch, or any builtin whose Args
	// were overridden in config. Substituted verbatim as discrete argv.
	if launcher.Preset == "custom" || len(launcher.Args) > 0 {
		exe := launcher.Exe
		if exe == "" && launcher.Preset != "" && launcher.Preset != "custom" {
			if p, err := resolvePreset(launcher.Preset); err == nil {
				exe = p.Exe
			}
		}
		if exe == "" {
			return "", nil, fmt.Errorf("custom terminal: no executable configured")
		}
		title := label
		if dot != "" {
			title = dot + " " + label
		}
		v := subVals{title: title, dir: dir, color: color, dot: dot, cmd: cmd}
		return subOne(exe, v, quoteNone), substitute(launcher.Args, v, quoteNone), nil
	}

	p, err := resolvePreset(launcher.Preset)
	if err != nil {
		return "", nil, err
	}
	if launcher.Exe != "" { // edited builtin exe path
		p.Exe = launcher.Exe
	}

	title := label
	if p.DotInTitle && dot != "" {
		title = dot + " " + label
	}
	shellCmd := composeShellCmd(cmd, p.Shell, p.NeedsOSC, p.DirInShell, dir, title)

	v := subVals{title: title, dir: dir, color: color, dot: dot, cmd: shellCmd}

	var args []string
	args = append(args, substitute(p.PreColor, v, p.Quote)...)
	if color != "" {
		args = append(args, substitute(p.ColorArgs, v, p.Quote)...)
	}
	args = append(args, substitute(p.PostColor, v, p.Quote)...)
	return p.Exe, args, nil
}

// Launch resolves the entry to a command and starts it detached so the new
// terminal outlives ClaudePanel. It never Wait()s. The child's working
// directory is set when the configured dir exists, which also gives the `cmd`
// preset its working directory without fragile cmd.exe quoting.
func Launch(entry config.TerminalConfig, launcher config.LauncherConfig, sublabel string) error {
	exe, args, err := build(entry, launcher, sublabel)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	if dir := resolveDir(entry.Dir); dir != "" {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			cmd.Dir = dir
		}
	}
	cmd.SysProcAttr = detachAttrs()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", exe, err)
	}
	// Detach: the terminal is the user's; we don't reap it.
	return cmd.Process.Release()
}

// resolveDir expands a leading ~ and falls back to the home directory when the
// configured dir is empty (so presets that hard-require a dir flag don't get a
// blank argument).
func resolveDir(dir string) string {
	dir = expandHome(strings.TrimSpace(dir))
	if dir == "" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
	}
	return dir
}

// expandHome expands a leading "~" / "~/" / "~\" to the user's home directory.
func expandHome(s string) string {
	if s == "" {
		return ""
	}
	if s == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return s
	}
	if strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, s[2:])
		}
	}
	return s
}

// ── Quoters ──────────────────────────────────────────────────────────────────

// shquote single-quotes a string for POSIX shells, ending the quote, inserting
// an escaped quote, and reopening: foo'bar -> 'foo'\”bar'.
func shquote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// pwshquote single-quotes a string for PowerShell, where an embedded single
// quote is doubled: foo'bar -> 'foo”bar'.
func pwshquote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// osaquote double-quotes a string for AppleScript, escaping backslash then
// double-quote.
func osaquote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// ── Color → emoji dot ──────────────────────────────────────────────────────────

type emojiDot struct {
	emoji   string
	r, g, b float64
}

// The 9 colored-circle emoji and a representative RGB for each.
var emojiDots = []emojiDot{
	{"🔴", 0xFF, 0x00, 0x00}, // red
	{"🟠", 0xFF, 0xA5, 0x00}, // orange
	{"🟡", 0xFF, 0xFF, 0x00}, // yellow
	{"🟢", 0x00, 0x80, 0x00}, // green
	{"🔵", 0x00, 0x00, 0xFF}, // blue
	{"🟣", 0x80, 0x00, 0x80}, // purple
	{"🟤", 0x8B, 0x45, 0x13}, // brown
	{"⚫", 0x00, 0x00, 0x00}, // black
	{"⚪", 0xFF, 0xFF, 0xFF}, // white
}

// nearestDot returns the colored-circle emoji nearest to a "#RRGGBB" hex by
// Euclidean RGB distance. Empty / unparseable input returns "".
func nearestDot(hex string) string {
	r, g, b, ok := parseHex(hex)
	if !ok {
		return ""
	}
	best := ""
	bestDist := math.MaxFloat64
	for _, d := range emojiDots {
		dr, dg, db := float64(r)-d.r, float64(g)-d.g, float64(b)-d.b
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			best = d.emoji
		}
	}
	return best
}

func parseHex(hex string) (r, g, b int, ok bool) {
	hex = strings.TrimSpace(hex)
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	var vals [3]int
	for i := 0; i < 3; i++ {
		v, err := parseByte(hex[i*2 : i*2+2])
		if err {
			return 0, 0, 0, false
		}
		vals[i] = v
	}
	return vals[0], vals[1], vals[2], true
}

func parseByte(s string) (int, bool) {
	v := 0
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= int(c - '0')
		case c >= 'a' && c <= 'f':
			v |= int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v |= int(c-'A') + 10
		default:
			return 0, true
		}
	}
	return v, false
}
