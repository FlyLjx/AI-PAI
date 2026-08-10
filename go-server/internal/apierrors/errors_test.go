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

func TestRetryableStatusDefaultsCoverTransientFailures(t *testing.T) {
	for _, status := range []int{408, 429, 500, 502, 503, 504} {
		if !IsRetryableStatus(status) {
			t.Fatalf("status %d should be retryable", status)
		}
	}
	for _, status := range []int{400, 401, 403, 404, 413, 428, 499} {
		if IsRetryableStatus(status) {
			t.Fatalf("status %d should not be retryable by default", status)
		}
	}
}

func TestParsePreservesAllStandardUpstreamCodes(t *testing.T) {
	standardCodes := map[int][]string{
		400: {"invalid_request", "request_body_incomplete", "reference_image_required", "content_policy_violation", "invalid_output_format", "prompt_required", "reference_image_invalid", "request_failed"},
		401: {"api_key_invalid"},
		403: {"request_not_allowed", "endpoint_not_allowed", "model_not_allowed"},
		404: {"resource_not_found"},
		413: {"request_body_too_large"},
		428: {"interactive_challenge_required"},
		429: {"image_generation_stalled", "image_generation_timeout", "image_quota_exhausted", "account_pool_unavailable", "task_queue_full", "service_rate_limited", "request_rate_limited", "client_quota_exhausted"},
		499: {"request_canceled"},
		500: {"internal_error"},
		502: {"image_generation_failed", "image_session_failed", "image_result_unavailable", "image_upload_failed", "service_error"},
		503: {"service_restarted", "service_unavailable", "service_busy"},
		504: {"image_upload_timeout", "image_preparation_timeout", "service_timeout"},
	}

	count := 0
	for status, codes := range standardCodes {
		for _, code := range codes {
			count++
			if mappedStatus, ok := StatusForCode(code); !ok || mappedStatus != status {
				t.Fatalf("StatusForCode(%q) = %d, %v; want %d, true", code, mappedStatus, ok, status)
			}
			details := Parse(status, map[string]any{"error": map[string]any{
				"message":   "upstream message",
				"code":      code,
				"retryable": false,
			}}, nil)
			if details.Code != code || details.StatusCode != status {
				t.Fatalf("status %d code %q was not preserved: %+v", status, code, details)
			}
		}
	}
	if count != 36 {
		t.Fatalf("standard code count = %d, want 36", count)
	}
}

func TestParseAcceptsStringErrorPayload(t *testing.T) {
	details := Parse(http.StatusNotFound, map[string]any{"error": "task not found"}, nil)
	if details.Message != "task not found" || details.Code != "resource_not_found" {
		t.Fatalf("unexpected string error details: %+v", details)
	}
}

func TestParseFindsNestedTaskError(t *testing.T) {
	details := Parse(http.StatusOK, map[string]any{
		"data": map[string]any{
			"status": "failed",
			"error": map[string]any{
				"code":    "image_generation_failed",
				"message": "任务生成失败",
			},
		},
	}, nil)
	if details.Message != "任务生成失败" || details.Code != "image_generation_failed" {
		t.Fatalf("unexpected nested task error details: %+v", details)
	}
}

func TestDefaultCodeUsesCurrentGatewayVocabulary(t *testing.T) {
	tests := map[int]string{
		400: "invalid_request",
		401: "api_key_invalid",
		403: "request_not_allowed",
		404: "resource_not_found",
		413: "request_body_too_large",
		428: "interactive_challenge_required",
		429: "service_rate_limited",
		499: "request_canceled",
		500: "internal_error",
		502: "service_error",
		503: "service_unavailable",
		504: "service_timeout",
	}
	for status, want := range tests {
		if got := DefaultCode(status); got != want {
			t.Fatalf("DefaultCode(%d) = %q, want %q", status, got, want)
		}
	}
}
