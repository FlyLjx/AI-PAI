package apierrors

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestParsePreservesStructuredUpstreamError(t *testing.T) {
	details := Parse(http.StatusBadGateway, map[string]any{
		"error": map[string]any{
			"message":    "OAI 生图未完成",
			"title":      "上游生图服务异常",
			"type":       "api_error",
			"code":       "image_generation_failed",
			"category":   "upstream",
			"retryable":  true,
			"action":     "retry",
			"hint":       "请重新提交任务",
			"request_id": "req-1",
		},
	}, nil)
	if details.Code != "image_generation_failed" || details.RequestID != "req-1" || !details.Retryable {
		t.Fatalf("unexpected details: %+v", details)
	}
}

func TestParsePreservesExplicitNonRetryable429(t *testing.T) {
	details := Parse(http.StatusTooManyRequests, map[string]any{
		"error": map[string]any{
			"code":      "client_quota_exhausted",
			"retryable": false,
		},
	}, nil)
	if details.Code != "client_quota_exhausted" || details.Retryable {
		t.Fatalf("explicit retryable=false was overwritten: %+v", details)
	}
}

func TestDetailsJSONRoundTripPreservesRetryableFlag(t *testing.T) {
	original := Details{StatusCode: http.StatusTooManyRequests, Code: "client_quota_exhausted", Retryable: false, RetryableSet: true}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Details
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.RetryableSet || decoded.Retryable {
		t.Fatalf("retryable flag was not preserved: %+v", decoded)
	}
	Normalize(&decoded)
	if decoded.Retryable {
		t.Fatalf("Normalize overwrote explicit false: %+v", decoded)
	}
}

func TestCountedFailureStatusOnlyIncludes429And502(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		if !IsCountedFailureStatus(status) {
			t.Fatalf("status %d should be counted", status)
		}
	}
	for _, status := range []int{400, 401, 403, 428, 499, 503, 504} {
		if IsCountedFailureStatus(status) {
			t.Fatalf("status %d should not be counted", status)
		}
	}
}
