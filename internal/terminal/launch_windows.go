//go:build windows

package terminal

import (
	"os/exec"
	"syscall"

	"claudepanel/internal/config"
)

// builtinPresets is ordered by detection preference: Windows Terminal first,
// then PowerShell, then Command Prompt.
func builtinPresets() []Preset {
	return []Preset{
		{
			Key: "windows-terminal",
			Exe: "wt.exe",
			// `wt -w new new-tab --suppressApplicationTitle --title "🔵 NAME" -d DIR pwsh -NoExit -Command CMD`.
			// The colored-dot emoji in the title carries the color (no --tabColor),
			// so name + color read consistently in the tab strip on every OS.
			// --suppressApplicationTitle pins the tab to our --title so the running
			// program (e.g. `claude`, which sets its own OSC title) can't rename it.
			PreColor:   []string{"-w", "new", "new-tab", "--suppressApplicationTitle", "--title", "{title}"},
			PostColor:  []string{"-d", "{dir}", "pwsh", "-NoExit", "-Command", "{cmd}"},
			DotInTitle: true,
			// {cmd} runs in pwsh (-Command); marks the shell for env-var syntax.
			// composeShellCmd only appends `exec` for bash/sh, so this is a no-op
			// for keep-open (WT stays open via -NoExit).
			Shell: "pwsh",
			Quote: quoteNone,
		},
		{
			Key: "powershell",
			Exe: "powershell",
			// Launched directly (no cmd.exe): a console subsystem child of our
			// GUI process gets its own visible console window. The title is set
			// from inside the session; -NoExit keeps it open.
			PreColor: []string{"-NoExit", "-Command",
				"$host.UI.RawUI.WindowTitle = {title}; Set-Location -LiteralPath {dir}; {cmd}"},
			DotInTitle: true,
			Shell:      "pwsh", // keep-open via -NoExit, not by appending exec
			Quote:      quotePwsh,
		},
		{
			Key: "cmd",
			Exe: "cmd.exe",
			// `cmd /k "title 🔵 NAME&CMD"`. The working dir is set via cmd.Dir
			// in Launch, which sidesteps cmd.exe's brittle quoting of `cd /d`.
			PreColor:   []string{"/k", "title {title}&{cmd}"},
			DotInTitle: true,
			Shell:      "cmd", // keep-open via /k
			Quote:      quoteNone,
		},
	}
}

// DetectDefault probes for an installed terminal in preference order.
func DetectDefault() config.LauncherConfig {
	for _, p := range builtinPresets() {
		if _, err := exec.LookPath(p.Exe); err == nil {
			return config.LauncherConfig{Preset: p.Key}
		}
	}
	// powershell.exe ships with every Windows install — a safe last resort.
	return config.LauncherConfig{Preset: "powershell"}
}

// detachAttrs is the deliberate inverse of internal/audio's hidden helper: NO
// HideWindow / CREATE_NO_WINDOW (that would hide the very console the user
// wants). CREATE_NEW_PROCESS_GROUP detaches Ctrl-C handling so closing
// ClaudePanel doesn't signal the terminal.
func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x00000200} // CREATE_NEW_PROCESS_GROUP
}
