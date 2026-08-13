package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRetryableGenerationLockError(t *testing.T) {
	for _, message := range []string{
		"Error 1213 (40001): Deadlock found when trying to get lock",
		"Error 1205 (HY000): Lock wait timeout exceeded",
		"ERROR: could not obtain lock on row in relation generation_tasks",
		"ERROR: serialization failure (SQLSTATE 40001)",
	} {
		if !isRetryableGenerationLockError(errors.New(message)) {
			t.Fatalf("expected retryable lock error: %q", message)
		}
	}
	if isRetryableGenerationLockError(context.DeadlineExceeded) {
		t.Fatal("parent request deadlines must not be retried")
	}
}

func TestCompatAdmissionDeadlineReturnsServiceUnavailable(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCompatGenerationAdmissionError(recorder, context.DeadlineExceeded)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestCompatAuthDeadlineReturnsServiceUnavailable(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCompatAuthError(recorder, context.DeadlineExceeded)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestGenerationAdmissionDeadlineMapsToServiceUnavailable(t *testing.T) {
	err := generationAdmissionError(context.DeadlineExceeded)
	var appErr appError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected appError, got %T", err)
	}
	if appErr.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", appErr.status, http.StatusServiceUnavailable)
	}
	if appErr.message != "请求入队繁忙，请稍后重试" {
		t.Fatalf("message = %q", appErr.message)
	}
}
