package apiaccess

import (
	"testing"
	"time"
)

func TestFillUsageTrendAddsMissingDates(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	start := time.Date(2026, 7, 10, 0, 0, 0, 0, location)
	end := time.Date(2026, 7, 12, 0, 0, 0, 0, location)
	items := []UsageTrendPoint{{Date: "2026-07-11", Total: 5, Success: 3, Failed: 2}}

	result := fillUsageTrend(start, end, items)
	if len(result) != 3 {
		t.Fatalf("len(result) = %d, want 3", len(result))
	}
	if result[0].Date != "2026-07-10" || result[0].Total != 0 {
		t.Fatalf("first point = %#v, want zero point for 2026-07-10", result[0])
	}
	if result[1] != items[0] {
		t.Fatalf("middle point = %#v, want %#v", result[1], items[0])
	}
	if result[2].Date != "2026-07-12" || result[2].Total != 0 {
		t.Fatalf("last point = %#v, want zero point for 2026-07-12", result[2])
	}
}
