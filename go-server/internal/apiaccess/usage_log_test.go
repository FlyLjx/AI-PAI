package apiaccess

import (
	"testing"
	"time"
)

func TestToPublicLogIncludesDurationAndRequestParameters(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	publicLog := ToPublicLog(UsageLog{
		ID:              "log-1",
		DurationSeconds: 2.375,
		RequestParams: map[string]any{
			"model": "image-model",
			"n":     2,
		},
		CreatedAt: createdAt,
	})

	if publicLog.DurationSeconds != 2.375 {
		t.Fatalf("expected durationSeconds 2.375, got %v", publicLog.DurationSeconds)
	}
	if publicLog.RequestParams["model"] != "image-model" || publicLog.RequestParams["n"] != 2 {
		t.Fatalf("unexpected request parameters: %#v", publicLog.RequestParams)
	}
}
