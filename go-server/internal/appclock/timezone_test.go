package appclock

import (
	"testing"
	"time"
)

func TestDatabaseTimeInterpretsUTCTimestampAsLocalWallTime(t *testing.T) {
	got := DatabaseTime(time.Date(2026, 7, 5, 17, 12, 0, 0, time.UTC))
	if got.Format(time.RFC3339) != "2026-07-05T17:12:00+08:00" {
		t.Fatalf("unexpected database time: %s", got.Format(time.RFC3339))
	}
}

func TestDatabaseTimeKeepsZonedTimeInstant(t *testing.T) {
	shanghai := ConfigureDefault()
	got := DatabaseTime(time.Date(2026, 7, 5, 17, 12, 0, 0, shanghai))
	if got.Format(time.RFC3339) != "2026-07-05T17:12:00+08:00" {
		t.Fatalf("unexpected zoned time: %s", got.Format(time.RFC3339))
	}
}
