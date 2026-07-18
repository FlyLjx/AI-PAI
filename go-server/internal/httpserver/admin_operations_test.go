package httpserver

import (
	"testing"
	"time"
)

func TestAdminOperationsRange(t *testing.T) {
	now := time.Date(2026, 7, 18, 15, 30, 0, 0, time.Local)
	rangeKey, startAt := adminOperationsRange("15d", now)
	if rangeKey != "15d" || startAt.Format("2006-01-02 15:04:05") != "2026-07-04 00:00:00" {
		t.Fatalf("range = %q, startAt = %s", rangeKey, startAt)
	}
	rangeKey, startAt = adminOperationsRange("invalid", now)
	if rangeKey != "today" || startAt.Format("2006-01-02 15:04:05") != "2026-07-18 00:00:00" {
		t.Fatalf("fallback range = %q, startAt = %s", rangeKey, startAt)
	}
}
