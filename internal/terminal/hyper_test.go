package terminal

import (
	"strings"
	"testing"
)

// A minimal .hyper.js stand-in: the single uncommented shell:/shellArgs: lines
// plus decoy comment lines that mention "shell:" and "shellArgs:" (which the
// rewrite must NOT touch).
const hyperFixturePristine = `module.exports = {
  config: {
        // PowerShell on Windows
        // - Example shell: 'C:\\path\\powershell.exe',
        shell: '',
        // for setting shell arguments (i.e. for using interactive shellArgs: ['-i'])
        shellArgs: [],
        env: {},
  },
};
`

func TestIsInjectedHyperConfig(t *testing.T) {
	if isInjectedHyperConfig(hyperFixturePristine) {
		t.Fatal("pristine config reported as injected")
	}
	injected := rewriteHyperShell(hyperFixturePristine,
		"shell: 'powershell.exe',",
		`shellArgs: ['-NoExit', '-Command', "$env:CLAUDE_CODE_DISABLE_TERMINAL_TITLE = '1'; claude"],`)
	if !isInjectedHyperConfig(injected) {
		t.Fatal("injected config not detected by marker")
	}
}

// TestRewriteHyperShellIdempotent is the regression test for the freeze bug:
// re-injecting an already-injected config must replace the previous
// project/account, not silently no-op (which left every launch stuck on the
// first one ever launched).
func TestRewriteHyperShellIdempotent(t *testing.T) {
	// Note the "[main]"/"[alt]" tags: the ] inside the value must not confuse the
	// shellArgs regex, which anchors on the trailing "]," at end of line.
	argsA := `shellArgs: ['-NoExit', '-Command', "$env:CLAUDE_CODE_DISABLE_TERMINAL_TITLE = '1'; Set-Location -LiteralPath 'C:\\projA'; $host.UI.RawUI.WindowTitle = '🔴 ProjA [main]'; claude"],`
	argsB := `shellArgs: ['-NoExit', '-Command', "$env:CLAUDE_CODE_DISABLE_TERMINAL_TITLE = '1'; Set-Location -LiteralPath 'C:\\projB'; $host.UI.RawUI.WindowTitle = '🟢 ProjB [alt]'; claude"],`

	// Pristine → inject project A.
	out := rewriteHyperShell(hyperFixturePristine, "shell: 'powershell.exe',", argsA)
	if !strings.Contains(out, "shell: 'powershell.exe',") {
		t.Error("shell line not rewritten")
	}
	if strings.Contains(out, "shellArgs: [],") {
		t.Error("pristine shellArgs placeholder still present after inject")
	}
	if !strings.Contains(out, "projA") || !strings.Contains(out, "ProjA [main]") {
		t.Error("project A args not injected")
	}
	// Decoy comment lines must be untouched.
	if !strings.Contains(out, "// - Example shell: 'C:\\\\path\\\\powershell.exe',") {
		t.Error("commented shell example line was modified")
	}
	if !strings.Contains(out, "interactive shellArgs: ['-i']") {
		t.Error("commented shellArgs example line was modified")
	}

	// Already-injected (A) → inject project B: B must REPLACE A, not coexist.
	out2 := rewriteHyperShell(out, "shell: 'powershell.exe',", argsB)
	if strings.Contains(out2, "projA") || strings.Contains(out2, "ProjA [main]") {
		t.Error("stale project A args survived re-injection (the freeze bug)")
	}
	if !strings.Contains(out2, "projB") || !strings.Contains(out2, "ProjB [alt]") {
		t.Error("project B args not injected on rewrite")
	}
	// Exactly one injected shellArgs line (the comment decoy is not a match).
	if n := strings.Count(out2, "shellArgs: ['-NoExit'"); n != 1 {
		t.Errorf("want exactly 1 injected shellArgs line, got %d", n)
	}
}

// TestRewriteHyperShellCRLF is the regression test for the CRLF bug: Hyper writes
// .hyper.js with \r\n line endings on Windows, and a bare `$` end-anchor on the
// shellArgs regex matched before the \n (after the \r), so the shellArgs line was
// never rewritten — Hyper then launched a plain shell with no `claude`.
func TestRewriteHyperShellCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(hyperFixturePristine, "\n", "\r\n")
	args := `shellArgs: ['-NoExit', '-Command', "$env:CLAUDE_CONFIG_DIR = 'C:\\Users\\Admin\\.claude-alt'; claude"],`
	out := rewriteHyperShell(crlf, "shell: 'powershell.exe',", args)

	if !strings.Contains(out, "shell: 'powershell.exe',") {
		t.Error("shell line not rewritten on CRLF input")
	}
	if strings.Contains(out, "shellArgs: [],") {
		t.Error("shellArgs placeholder still present on CRLF input — the bug")
	}
	if !strings.Contains(out, "shellArgs: ['-NoExit'") || !strings.Contains(out, "claude-alt") {
		t.Error("shellArgs not injected on CRLF input")
	}
	// CRLF line endings preserved (no stray lone-LF introduced on the rewritten line).
	if strings.Contains(out, "],\n") && !strings.Contains(out, "],\r\n") {
		t.Error("CRLF line ending not preserved on the injected shellArgs line")
	}
}
