package httpserver

import (
	"net/http"
	"net/http/httptest"
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

func TestAdminOperationsSplitRoutesAreRegisteredAndRequireGET(t *testing.T) {
	router := &Router{mux: http.NewServeMux()}
	router.routes()
	paths := []string{
		"/api/admin/api-access/operations/live",
		"/api/admin/api-access/operations/ranking",
		"/api/admin/api-access/operations/trend",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "http://example.test"+path, nil)
			router.mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}
