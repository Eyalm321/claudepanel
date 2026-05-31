package claude

import (
	"testing"
	"time"
)

// fixedNow matches the timestamps baked into testdata/populated:
//   - the busy session was updated one minute earlier
//   - seven_day resets three days four hours later  → "3D 4H"
//   - five_hour resets two hours thirty minutes later → "2H 30M"
//
// Keeping the clock fixed makes the period sum, the last-data label and the
// reset countdowns assertable regardless of when the suite runs.
var fixedNow = time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)

func TestLoadBarData_PopulatedAccount(t *testing.T) {
	got, err := loadBarDataAt("testdata/populated", "Test Account", fixedNow)
	if err != nil {
		t.Fatalf("loadBarDataAt returned error: %v", err)
	}
	if got == nil {
		t.Fatal("loadBarDataAt returned nil BarData")
	}

	want := &BarData{
		AccountName:      "Test Account",
		SubscriptionType: "MAX",
		PeriodMessages:   350, // 100 (5-15) + 250 (5-20), both within May
		PeriodPercent:    68.0 / 100.0,
		PeriodMsgLimit:   1, // sentinel: API-sourced percent is present
		LastDataLabel:    "5-20",
		LastDataMsgs:     250,
		HourlyPercent:    32.5 / 100.0,
		HourlyResetIn:    "2H 30M",
		ResetIn:          "3D 4H",
		PrimaryModel:     "OPUS 4.7", // from rate_limits model_id, not stats fallback
		Status:           "BUSY",
		LimitExceeded:    false, // weekly 0.68 < 1.0
		LastUpdated:      fixedNow.UnixMilli(),
	}

	if *got != *want {
		t.Errorf("BarData mismatch:\n got: %+v\nwant: %+v", *got, *want)
	}
}
