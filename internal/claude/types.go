package claude

type DailyActivity struct {
	Date          string `json:"date"`
	MessageCount  int    `json:"messageCount"`
	SessionCount  int    `json:"sessionCount"`
	ToolCallCount int    `json:"toolCallCount"`
}

type DailyModelTokens struct {
	Date          string           `json:"date"`
	TokensByModel map[string]int64 `json:"tokensByModel"`
}

type ModelUsageDetail struct {
	InputTokens              int64 `json:"inputTokens"`
	OutputTokens             int64 `json:"outputTokens"`
	CacheReadInputTokens     int64 `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64 `json:"cacheCreationInputTokens"`
}

type StatsCache struct {
	Version          int                         `json:"version"`
	LastComputedDate string                      `json:"lastComputedDate"`
	DailyActivity    []DailyActivity             `json:"dailyActivity"`
	DailyModelTokens []DailyModelTokens          `json:"dailyModelTokens"`
	ModelUsage       map[string]ModelUsageDetail `json:"modelUsage"`
	TotalSessions    int                         `json:"totalSessions"`
	TotalMessages    int                         `json:"totalMessages"`
	HourCounts       map[string]int              `json:"hourCounts"`
}

type OAuthCredentials struct {
	AccessToken      string `json:"accessToken"`
	SubscriptionType string `json:"subscriptionType"`
	RateLimitTier    string `json:"rateLimitTier"`
	ExpiresAt        int64  `json:"expiresAt"`
}

type Credentials struct {
	ClaudeAiOauth OAuthCredentials `json:"claudeAiOauth"`
}

type SessionFile struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
	UpdatedAt int64  `json:"updatedAt"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
}

type NotificationState struct {
	Triggered bool   `json:"triggered"`
	Timestamp string `json:"timestamp"`
}

type NotificationStates struct {
	ExceedMaxLimit   *NotificationState `json:"exceed_max_limit"`
	TokensWillRunOut *NotificationState `json:"tokens_will_run_out"`
	CostWillExceed   *NotificationState `json:"cost_will_exceed"`
}
