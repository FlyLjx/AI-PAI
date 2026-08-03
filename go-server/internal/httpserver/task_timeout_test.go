package httpserver

import (
	"testing"
	"time"
)

func TestCompatTaskWaitTimeoutLeavesProxyBuffer(t *testing.T) {
	if got := compatTaskWaitTimeout(10 * time.Minute); got != 590*time.Second {
		t.Fatalf("compat wait timeout = %s, want 590s", got)
	}
	if got := compatTaskWaitTimeout(5 * time.Second); got != 5*time.Second {
		t.Fatalf("short compat wait timeout = %s, want 5s", got)
	}
	if got := compatTaskWaitTimeout(0); got != compatTaskTimeoutFallback {
		t.Fatalf("fallback compat wait timeout = %s, want %s", got, compatTaskTimeoutFallback)
	}
}
