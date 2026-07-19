package requestmonitor

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaptureRedactsSensitiveValuesAndRestoresBody(t *testing.T) {
	body := `{"model":"gpt-image-1","prompt":"test prompt","password":"hidden","api_key":"secret","image":"data:image/png;base64,abc"}`
	req := httptest.NewRequest("POST", "http://example.test/v1/images/generations?token=secret&size=1k", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://client.example/path?token=secret")
	req.Header.Set("X-Forwarded-For", "203.0.113.8, 10.0.0.1")

	captured := Capture(req)
	remaining := new(strings.Builder)
	if _, err := io.Copy(remaining, req.Body); err != nil {
		t.Fatal(err)
	}
	if remaining.String() != body {
		t.Fatalf("restored body = %q", remaining.String())
	}
	if strings.Contains(string(captured.BodyParams), "hidden") || strings.Contains(string(captured.BodyParams), "secret") {
		t.Fatalf("sensitive body leaked: %s", captured.BodyParams)
	}
	if strings.Contains(string(captured.QueryParams), "secret") {
		t.Fatalf("sensitive query leaked: %s", captured.QueryParams)
	}
	if !strings.Contains(string(captured.BodyParams), "test prompt") {
		t.Fatalf("ordinary parameter missing: %s", captured.BodyParams)
	}
	if captured.SourceIP != "203.0.113.8" || captured.SourceHost != "client.example" || captured.Origin != "https://client.example/path" {
		t.Fatalf("unexpected source: %+v", captured)
	}
	var decoded map[string]any
	if err := json.Unmarshal(captured.BodyParams, &decoded); err != nil {
		t.Fatal(err)
	}
}

func TestShouldRecordSkipsHealthAndMonitorQueries(t *testing.T) {
	for _, path := range []string{"/api/health", "/api/admin/request-monitor"} {
		if ShouldRecord(httptest.NewRequest("GET", "http://example.test"+path, nil)) {
			t.Fatalf("expected %s to be skipped", path)
		}
	}
	if !ShouldRecord(httptest.NewRequest("POST", "http://example.test/v1/images/generations", nil)) {
		t.Fatal("expected OpenAI-compatible request to be recorded")
	}
}
