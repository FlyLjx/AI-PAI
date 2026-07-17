package apiaccess

import (
	"testing"
	"time"
)

func TestToPublicLogIncludesDurationSeconds(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	publicLog := ToPublicLog(UsageLog{
		ID:              "log-1",
		DurationSeconds: 2.375,
		CreatedAt:       createdAt,
	})

	if publicLog.DurationSeconds != 2.375 {
		t.Fatalf("expected durationSeconds 2.375, got %v", publicLog.DurationSeconds)
	}
}
