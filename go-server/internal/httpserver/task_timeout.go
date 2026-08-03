package httpserver

import (
	"context"
	"time"

	"aipi-go/internal/settings"
)

const (
	taskTimeoutCacheTTL       = 15 * time.Second
	compatTaskTimeoutFallback = 290 * time.Second
	compatTaskTimeoutBuffer   = 10 * time.Second
)

func (r *Router) taskProcessingTimeout() time.Duration {
	r.taskTimeoutMu.RLock()
	timeout, cachedAt := r.taskTimeoutCache, r.taskTimeoutCacheAt
	r.taskTimeoutMu.RUnlock()
	if timeout > 0 && time.Since(cachedAt) < taskTimeoutCacheTTL {
		return timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	values, err := settings.NewRepository(r.db).Get(ctx)
	cancel()
	if err == nil {
		timeout = settings.TaskTimeout(values)
		r.cacheTaskProcessingTimeout(timeout)
		return timeout
	}
	if r.logger != nil {
		r.logger.Warn("task timeout settings lookup failed", "error", err)
	}
	if timeout <= 0 {
		timeout = settings.DefaultTaskTimeoutMinutes * time.Minute
	}
	return timeout
}

func (r *Router) cacheTaskProcessingTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = settings.DefaultTaskTimeoutMinutes * time.Minute
	}
	r.taskTimeoutMu.Lock()
	r.taskTimeoutCache = timeout
	r.taskTimeoutCacheAt = time.Now()
	r.taskTimeoutMu.Unlock()
}

func compatTaskWaitTimeout(processingTimeout time.Duration) time.Duration {
	if processingTimeout <= 0 {
		return compatTaskTimeoutFallback
	}
	wait := processingTimeout - compatTaskTimeoutBuffer
	if wait < 5*time.Second {
		return 5 * time.Second
	}
	return wait
}
