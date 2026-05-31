package claude

import (
	"testing"
)

// These tests pin the fallback and edge behaviour of the read-then-compute
// pipeline that previously had no test home: what happens when the captured
// rate-limit file is missing, stale, or disagrees with the sticky notification
// state, and how the primary-model / last-data / reset fields degrade.
//
// All cases run through loadBarDataAt with the shared fixedNow clock (defined in
// bardata_test.go) so calendar-dependent fields are deterministic.

// Missing rate_limits.json: live usage is unavailable, so the percent-based
// fields report "no data" rather than a wrong value, and the bar falls back to
// stats-cache for messages + primary model and to the month boundary for reset.
func TestLoadBarData_MissingUsage(t *testing.T) {
	got, err := loadBarDataAt("testdata/no_usage", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"SubscriptionType", got.SubscriptionType, "PRO"},
		{"PeriodMessages", got.PeriodMessages, int64(80)},
		{"PeriodPercent", got.PeriodPercent, 0.0},
		{"PeriodMsgLimit", got.PeriodMsgLimit, int64(0)}, // unavailable, not shown
		{"HourlyPercent", got.HourlyPercent, -1.0},
		{"HourlyResetIn", got.HourlyResetIn, "---"},
		{"ResetIn", got.ResetIn, "12H 0M"},               // month-boundary fallback
		{"PrimaryModel", got.PrimaryModel, "SONNET 4.6"}, // stats-cache fallback
		{"LastDataLabel", got.LastDataLabel, "5-10"},
		{"LastDataMsgs", got.LastDataMsgs, 80},
		{"Status", got.Status, "OFFLINE"},
		{"LimitExceeded", got.LimitExceeded, false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// Stale rate_limits.json (captured_at older than the 2h cutoff) is treated as
// unavailable, so the percent fields fall back to "no data" rather than showing
// outdated usage.
func TestLoadBarData_StaleUsage(t *testing.T) {
	got, err := loadBarDataAt("testdata/stale_usage", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.PeriodMsgLimit != 0 {
		t.Errorf("PeriodMsgLimit = %d, want 0 (stale weekly percent should be unavailable)", got.PeriodMsgLimit)
	}
	if got.HourlyPercent != -1.0 {
		t.Errorf("HourlyPercent = %v, want -1 (stale hourly percent should be unavailable)", got.HourlyPercent)
	}
	if got.LimitExceeded {
		t.Errorf("LimitExceeded = true, want false (stale data should not drive the limit flag)")
	}
}

// rate_limits.json with no captured_at (older statusline wrappers) is tolerated
// rather than treated as stale, so its live data still drives the bar.
func TestLoadBarData_MissingCapturedAt(t *testing.T) {
	got, err := loadBarDataAt("testdata/no_captured_at", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.PeriodMsgLimit != 1 {
		t.Errorf("PeriodMsgLimit = %d, want 1 (live data without captured_at should still load)", got.PeriodMsgLimit)
	}
	if got.PeriodPercent != 40.0/100.0 {
		t.Errorf("PeriodPercent = %v, want 0.4", got.PeriodPercent)
	}
}

// captured_at in Unix SECONDS (the real statusline wrapper's format, matching
// resets_at in the same file) is interpreted correctly: a fresh seconds
// timestamp loads its live data. Regression guard for the units bug that dropped
// all live usage when captured_at was misread as milliseconds.
func TestLoadBarData_FreshSecondsCapturedAt(t *testing.T) {
	got, err := loadBarDataAt("testdata/fresh_seconds", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.PeriodMsgLimit != 1 {
		t.Errorf("PeriodMsgLimit = %d, want 1 (fresh seconds-based captured_at should load)", got.PeriodMsgLimit)
	}
	if got.PeriodPercent != 37.0/100.0 {
		t.Errorf("PeriodPercent = %v, want 0.37", got.PeriodPercent)
	}
	if got.HourlyPercent != 35.0/100.0 {
		t.Errorf("HourlyPercent = %v, want 0.35", got.HourlyPercent)
	}
	if got.PrimaryModel != "OPUS 4.8" {
		t.Errorf("PrimaryModel = %q, want OPUS 4.8 (from rate_limits model_id)", got.PrimaryModel)
	}
}

// A seconds-based captured_at older than the cutoff is still detected as stale.
func TestLoadBarData_StaleSecondsCapturedAt(t *testing.T) {
	got, err := loadBarDataAt("testdata/stale_seconds", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.PeriodMsgLimit != 0 {
		t.Errorf("PeriodMsgLimit = %d, want 0 (stale seconds-based captured_at should be unavailable)", got.PeriodMsgLimit)
	}
}

// Source-of-truth, reset case: live weekly usage is below 100% but the sticky
// notification file still says the limit was breached. The live value wins, so
// the bar does NOT stay red after a reset.
func TestLoadBarData_LimitExceeded_LiveWinsAfterReset(t *testing.T) {
	got, err := loadBarDataAt("testdata/exceeded_reset", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.LimitExceeded {
		t.Error("LimitExceeded = true, want false (live 50% must override sticky notification)")
	}
	if got.PeriodPercent != 50.0/100.0 {
		t.Errorf("PeriodPercent = %v, want 0.5 (live data should be used)", got.PeriodPercent)
	}
}

// Source-of-truth, sticky fallback: no live rate-limit file, so the sticky
// notification flag is the only signal and must be honoured.
func TestLoadBarData_LimitExceeded_StickyFallback(t *testing.T) {
	got, err := loadBarDataAt("testdata/exceeded_sticky", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !got.LimitExceeded {
		t.Error("LimitExceeded = false, want true (sticky notification is the only signal)")
	}
	if got.PeriodMsgLimit != 0 {
		t.Errorf("PeriodMsgLimit = %d, want 0 (no live percent available)", got.PeriodMsgLimit)
	}
}

// Last-data label: the most recent day with data is today's date, so the label
// reads TODAY rather than M-D.
func TestLoadBarData_LastDataLabel_Today(t *testing.T) {
	got, err := loadBarDataAt("testdata/today", "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.LastDataLabel != "TODAY" {
		t.Errorf("LastDataLabel = %q, want TODAY", got.LastDataLabel)
	}
	if got.LastDataMsgs != 42 {
		t.Errorf("LastDataMsgs = %d, want 42", got.LastDataMsgs)
	}
}

// Empty account directory: every reader returns nil, so the bar shows the
// no-data sentinels rather than crashing or fabricating values.
func TestLoadBarData_EmptyAccount(t *testing.T) {
	got, err := loadBarDataAt(t.TempDir(), "Acct", fixedNow)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"SubscriptionType", got.SubscriptionType, ""},
		{"PeriodMessages", got.PeriodMessages, int64(0)},
		{"PeriodMsgLimit", got.PeriodMsgLimit, int64(0)},
		{"HourlyPercent", got.HourlyPercent, -1.0},
		{"LastDataLabel", got.LastDataLabel, "---"},
		{"LastDataMsgs", got.LastDataMsgs, 0},
		{"PrimaryModel", got.PrimaryModel, "---"},
		{"Status", got.Status, "OFFLINE"},
		{"ResetIn", got.ResetIn, "12H 0M"}, // month-boundary fallback
		{"LimitExceeded", got.LimitExceeded, false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}
