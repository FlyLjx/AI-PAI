package httpserver

import (
	"context"
	"errors"
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
