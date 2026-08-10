package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/database"
)

const (
	// A generation request should not wait for an account row indefinitely. A
	// short, explicit lock budget prevents a busy account from consuming the
	// whole request deadline; transient database deadlocks are retried below.
	generationLockWaitTimeout = 5 * time.Second
	generationLockRetryCount  = 3
	generationEnqueueTimeout  = 30 * time.Second
)

func (r *Router) withUserGenerationLock(ctx context.Context, userID string, fn func(tx *database.Tx) error) error {
	var lastErr error
	for attempt := 0; attempt < generationLockRetryCount; attempt++ {
		err, busy := r.withUserGenerationLockOnce(ctx, userID, fn)
		if err == nil {
			return nil
		}
		lastErr = err
		if busy {
			return err
		}
		if !isRetryableGenerationLockError(err) || ctx.Err() != nil || attempt == generationLockRetryCount-1 {
			break
		}
		backoff := time.Duration(attempt+1) * 50 * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return lastErr
}

func (r *Router) withUserGenerationLockOnce(ctx context.Context, userID string, fn func(tx *database.Tx) error) (error, bool) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err, false
	}
	defer tx.Rollback()

	lockCtx, cancel := context.WithTimeout(ctx, generationLockWaitTimeout)
	defer cancel()
	var lockedUserID string
	if err := tx.QueryRowContext(lockCtx, `
		SELECT id
		FROM users
		WHERE id = ? AND status = 'active'
		FOR UPDATE
	`, userID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newAppError(http.StatusForbidden, "用户不存在或已被禁用"), false
		}
		if errors.Is(lockCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return newAppError(http.StatusTooManyRequests, "当前账户请求较多，请稍后重试"), true
		}
		return err, false
	}

	if err := fn(tx); err != nil {
		return err, false
	}
	return tx.Commit(), false
}

func isRetryableGenerationLockError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"deadlock",
		"lock wait timeout",
		"could not obtain lock",
		"could not serialize access",
		"serialization failure",
		"sqlstate 40001",
		"sqlstate 40p01",
		"error 1213",
		"error 1205",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
