package claude

import (
	"fmt"
	"strings"
	"time"
)

// BarData is the computed display payload sent to the frontend on each poll.
type BarData struct {
	AccountName      string  `json:"accountName"`
	SubscriptionType string  `json:"subscriptionType"`
	WeeklyTokens     int64   `json:"weeklyTokens"`
	WeeklyTokenLimit int64   `json:"weeklyTokenLimit"`
	WeeklyPercent    float64 `json:"weeklyPercent"` // 0.0–1.0
	ResetIn          string  `json:"resetIn"`
	PrimaryModel     string  `json:"primaryModel"`
	Status           string  `json:"status"` // ONLINE / IDLE / OFFLINE
	TodayMessages    int     `json:"todayMessages"`
	TodayToolCalls   int     `json:"todayToolCalls"`
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
	tokenLimit int64,
	billingResetDay int,
) *BarData {
	now := time.Now()
	periodStart := billingPeriodStart(now, billingResetDay)
	periodStartStr := periodStart.Format("2006-01-02")

	var periodTokens int64
	if sc != nil {
		for _, day := range sc.DailyModelTokens {
			if day.Date >= periodStartStr {
				for _, v := range day.TokensByModel {
					periodTokens += v
				}
			}
		}
	}

	var pct float64
	if tokenLimit > 0 {
		pct = float64(periodTokens) / float64(tokenLimit)
		if pct > 1.0 {
			pct = 1.0
		}
	}

	nextReset := billingPeriodStart(now, billingResetDay).AddDate(0, 1, 0)
	resetIn := formatDuration(nextReset.Sub(now))

	primaryModel := computePrimaryModel(sc, periodStartStr)
	status, _ := computeStatus(sessions)

	todayStr := now.Format("2006-01-02")
	var todayMsgs, todayTools int
	if sc != nil {
		for _, day := range sc.DailyActivity {
			if day.Date == todayStr {
				todayMsgs = day.MessageCount
				todayTools = day.ToolCallCount
				break
			}
		}
	}

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
		WeeklyTokens:     periodTokens,
		WeeklyTokenLimit: tokenLimit,
		WeeklyPercent:    pct,
		ResetIn:          resetIn,
		PrimaryModel:     primaryModel,
		Status:           status,
		TodayMessages:    todayMsgs,
		TodayToolCalls:   todayTools,
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

func shortModelName(full string) string {
	lower := strings.ToLower(full)
	switch {
	case strings.Contains(lower, "opus"):
		return "OPUS"
	case strings.Contains(lower, "sonnet"):
		return "SONNET"
	case strings.Contains(lower, "haiku"):
		return "HAIKU"
	case full == "":
		return "---"
	default:
		if len(full) > 8 {
			return strings.ToUpper(full[:8])
		}
		return strings.ToUpper(full)
	}
}

func computeStatus(sessions []SessionFile) (string, int) {
	nowMs := time.Now().UnixMilli()
	active := 0
	anyRecentIdle := false
	for _, s := range sessions {
		age := nowMs - s.UpdatedAt
		if s.Status == "active" && age < 5*60*1000 {
			active++
		} else if age < 30*60*1000 {
			anyRecentIdle = true
		}
	}
	if active > 0 {
		return "ONLINE", active
	}
	if anyRecentIdle {
		return "IDLE", 0
	}
	return "OFFLINE", 0
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
