package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUsageTrendRangeDefaultsToSevenDays(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/api-access/logs/trend", nil)

	start, end, err := usageTrendRange(req, now)
	if err != nil {
		t.Fatal(err)
	}
	if start.Format("2006-01-02") != "2026-07-10" || end.Format("2006-01-02") != "2026-07-16" {
		t.Fatalf("range = %s to %s, want 2026-07-10 to 2026-07-16", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

func TestUsageTrendRangeAcceptsCustomDates(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/api-access/logs/trend?startDate=2026-07-01&endDate=2026-07-15", nil)

	start, end, err := usageTrendRange(req, now)
	if err != nil {
		t.Fatal(err)
	}
	if start.Format("2006-01-02") != "2026-07-01" || end.Format("2006-01-02") != "2026-07-15" {
		t.Fatalf("range = %s to %s, want 2026-07-01 to 2026-07-15", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

func TestUsageTrendRangeRejectsInvalidRanges(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	tests := []string{
		"?startDate=invalid&endDate=2026-07-15",
		"?startDate=2026-07-15&endDate=invalid",
		"?startDate=2026-07-16&endDate=2026-07-17",
		"?startDate=2026-07-16&endDate=2026-07-15",
		"?startDate=2025-07-15&endDate=2026-07-16",
	}
	for _, query := range tests {
		req := httptest.NewRequest(http.MethodGet, "http://example.test/api/api-access/logs/trend"+query, nil)
		if _, _, err := usageTrendRange(req, now); err == nil {
			t.Fatalf("query %q was accepted, want error", query)
		}
	}
}
