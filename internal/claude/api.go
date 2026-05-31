package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// APIUsage holds real-time usage data sourced from rate_limits.json,
// which is captured by our statusline wrapper inside each account dir.
// Schema mirrors the rate_limits block Claude Code passes to statusline scripts.
type APIUsage struct {
	WeeklyPercent float64   // 0.0–1.0; negative if unavailable
	HourlyPercent float64   // 0.0–1.0; negative if unavailable
	ResetAt       time.Time // seven_day reset; zero if unavailable
	HourlyResetAt time.Time // five_hour reset; zero if unavailable
	LimitExceeded bool
	ModelID       string
}

type rateLimitWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type rateLimitModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type rateLimitsFile struct {
	FiveHour   *rateLimitWindow `json:"five_hour"`
	SevenDay   *rateLimitWindow `json:"seven_day"`
	ModelID    string           `json:"model_id"`
	Model      *rateLimitModel  `json:"model"`
	CapturedAt int64            `json:"captured_at"`
}

// usageStaleAfter is how old a captured rate_limits.json may be before we treat
// it as unavailable: past this, Claude Code hasn't run recently and the usage
// percentages / reset times no longer reflect reality.
const usageStaleAfter = 2 * time.Hour

// readUsage loads rate-limit data captured by the statusline wrapper. Returns nil
// if the file is missing, unparseable, or stale — older than usageStaleAfter
// relative to now. captured_at is written by the wrapper as JavaScript Date.now()
// (Unix milliseconds); a zero/missing captured_at is tolerated, since wrappers
// predating that field still produce useful current data.
func readUsage(accountPath string, now time.Time) *APIUsage {
	data, err := os.ReadFile(filepath.Join(accountPath, "rate_limits.json"))
	if err != nil {
		return nil
	}
	var rl rateLimitsFile
	if json.Unmarshal(data, &rl) != nil {
		return nil
	}
	if rl.CapturedAt > 0 && now.Sub(time.UnixMilli(rl.CapturedAt)) > usageStaleAfter {
		return nil
	}

	out := &APIUsage{WeeklyPercent: -1, HourlyPercent: -1}
	if rl.ModelID != "" {
		out.ModelID = rl.ModelID
	} else if rl.Model != nil {
		if rl.Model.ID != "" {
			out.ModelID = rl.Model.ID
		} else if rl.Model.DisplayName != "" {
			out.ModelID = rl.Model.DisplayName
		}
	}

	if rl.SevenDay != nil {
		out.WeeklyPercent = clampPct(rl.SevenDay.UsedPercentage / 100.0)
		if rl.SevenDay.ResetsAt > 0 {
			out.ResetAt = time.Unix(rl.SevenDay.ResetsAt, 0)
		}
	}
	if rl.FiveHour != nil {
		out.HourlyPercent = clampPct(rl.FiveHour.UsedPercentage / 100.0)
		if rl.FiveHour.ResetsAt > 0 {
			out.HourlyResetAt = time.Unix(rl.FiveHour.ResetsAt, 0)
		}
	}
	if out.WeeklyPercent >= 1.0 {
		out.LimitExceeded = true
	}
	return out
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}
