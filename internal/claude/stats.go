package claude

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var modelVersionRe = regexp.MustCompile(`-(\d+)-(\d+)`)

// BarData is the computed display payload sent to the frontend on each poll.
type BarData struct {
	AccountName      string  `json:"accountName"`
	SubscriptionType string  `json:"subscriptionType"`
	// Message-based usage (more intuitive than raw tokens)
	PeriodMessages   int64   `json:"periodMessages"`
	PeriodPercent    float64 `json:"periodPercent"`  // 0.0–1.0 when msgLimit > 0
	PeriodMsgLimit   int64   `json:"periodMsgLimit"` // 0 = no limit configured
	// Most recent day with data (stats-cache may lag several days)
	LastDataLabel    string  `json:"lastDataLabel"` // "TODAY" or "5-21"
	LastDataMsgs     int     `json:"lastDataMsgs"`
	// Other metrics
	ResetIn          string  `json:"resetIn"`
	PrimaryModel     string  `json:"primaryModel"`
	Status           string  `json:"status"` // BUSY / IDLE / OFFLINE
	LimitExceeded    bool    `json:"limitExceeded"`
	LastUpdated      int64   `json:"lastUpdated"` // unix ms
}

// ComputeBarData derives all display metrics from raw file data.
func ComputeBarData(
	accountName string,
	sc *StatsCache,
	creds *Credentials,
	sessions []SessionFile,
	notifs *NotificationStates,
	msgLimit int64,
	billingResetDay int,
) *BarData {
	now := time.Now()
	periodStart := billingPeriodStart(now, billingResetDay)
	periodStartStr := periodStart.Format("2006-01-02")
	todayStr := now.Format("2006-01-02")

	// Sum messages for the billing period
	var periodMsgs int64
	var lastDate string
	var lastMsgs int
	if sc != nil {
		for _, day := range sc.DailyActivity {
			if day.Date >= periodStartStr {
				periodMsgs += int64(day.MessageCount)
			}
			// Track most recent day that has data
			if day.Date > lastDate && day.MessageCount > 0 {
				lastDate = day.Date
				lastMsgs = day.MessageCount
			}
		}
	}

	// Progress percent — only when a limit is configured
	var pct float64
	if msgLimit > 0 {
		pct = float64(periodMsgs) / float64(msgLimit)
		if pct > 1.0 {
			pct = 1.0
		}
	}

	// Human-readable label for last-data date
	lastDataLabel := "---"
	if lastDate != "" {
		if lastDate == todayStr {
			lastDataLabel = "TODAY"
		} else {
			// Parse and format as "5-21"
			if t, err := time.Parse("2006-01-02", lastDate); err == nil {
				lastDataLabel = fmt.Sprintf("%d-%d", int(t.Month()), t.Day())
			} else {
				lastDataLabel = lastDate[5:] // "MM-DD"
			}
		}
	}

	nextReset := billingPeriodStart(now, billingResetDay).AddDate(0, 1, 0)
	resetIn := formatDuration(nextReset.Sub(now))

	primaryModel := computePrimaryModel(sc, periodStartStr)
	status := computeStatus(sessions)

	limitExceeded := false
	if notifs != nil && notifs.ExceedMaxLimit != nil {
		limitExceeded = notifs.ExceedMaxLimit.Triggered
	}

	subType := ""
	if creds != nil {
		subType = strings.ToUpper(creds.ClaudeAiOauth.SubscriptionType)
	}

	return &BarData{
		AccountName:      accountName,
		SubscriptionType: subType,
		PeriodMessages:   periodMsgs,
		PeriodPercent:    pct,
		PeriodMsgLimit:   msgLimit,
		LastDataLabel:    lastDataLabel,
		LastDataMsgs:     lastMsgs,
		ResetIn:          resetIn,
		PrimaryModel:     primaryModel,
		Status:           status,
		LimitExceeded:    limitExceeded,
		LastUpdated:      now.UnixMilli(),
	}
}

func billingPeriodStart(now time.Time, resetDay int) time.Time {
	if resetDay < 1 {
		resetDay = 1
	}
	if resetDay > 28 {
		resetDay = 28
	}
	candidate := time.Date(now.Year(), now.Month(), resetDay, 0, 0, 0, 0, now.Location())
	if candidate.After(now) {
		candidate = candidate.AddDate(0, -1, 0)
	}
	return candidate
}

func computePrimaryModel(sc *StatsCache, periodStartStr string) string {
	if sc == nil {
		return "---"
	}
	totals := make(map[string]int64)
	for _, day := range sc.DailyModelTokens {
		if day.Date >= periodStartStr {
			for model, tokens := range day.TokensByModel {
				totals[model] += tokens
			}
		}
	}
	var topModel string
	var topTokens int64
	for model, tokens := range totals {
		if tokens > topTokens {
			topTokens = tokens
			topModel = model
		}
	}
	if topModel == "" {
		for model := range sc.ModelUsage {
			topModel = model
			break
		}
	}
	return shortModelName(topModel)
}

// shortModelName converts a full model ID to a compact display name with version.
// e.g. "claude-opus-4-7" → "OPUS 4.7"
func shortModelName(full string) string {
	if full == "" {
		return "---"
	}
	lower := strings.ToLower(full)

	var family string
	switch {
	case strings.Contains(lower, "opus"):
		family = "OPUS"
	case strings.Contains(lower, "sonnet"):
		family = "SONNET"
	case strings.Contains(lower, "haiku"):
		family = "HAIKU"
	default:
		if len(full) > 8 {
			return strings.ToUpper(full[:8])
		}
		return strings.ToUpper(full)
	}

	// Extract major.minor version from pattern like "-4-7"
	if m := modelVersionRe.FindStringSubmatch(lower); m != nil {
		return family + " " + m[1] + "." + m[2]
	}
	return family
}

// computeStatus derives BUSY/IDLE/OFFLINE from session file freshness.
// Real session statuses seen: "busy" (processing), "idle" (open but waiting).
func computeStatus(sessions []SessionFile) string {
	nowMs := time.Now().UnixMilli()
	for _, s := range sessions {
		age := nowMs - s.UpdatedAt
		// Any non-idle status updated in the last 5 minutes = actively working
		if s.Status != "idle" && age < 5*60*1000 {
			return "BUSY"
		}
	}
	// Any session updated in the last 60 minutes = someone has Claude open
	for _, s := range sessions {
		if nowMs-s.UpdatedAt < 60*60*1000 {
			return "IDLE"
		}
	}
	return "OFFLINE"
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "NOW"
	}
	totalHours := int(d.Hours())
	days := totalHours / 24
	hours := totalHours % 24
	if days > 0 {
		return fmt.Sprintf("%dD %dH", days, hours)
	}
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dH %dM", totalHours, minutes)
}
