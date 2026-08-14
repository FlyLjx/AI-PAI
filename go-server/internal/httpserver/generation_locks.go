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
	// Admission must stay database-only. Quote and entitlement reads happen
	// before this transaction, and callers reserve the balance as their final
	// statement so the per-user row lock is released by the following commit.
	generationLockRetryCount = 3
	generationEnqueueTimeout = 2 * time.Minute
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

	// Validate the account without SELECT ... FOR UPDATE. A generation task
	// must not hold an account lock while doing quota/price reads or writing
	// access logs; the balance reservation UPDATE is the only admission point
	// that needs a short database row lock.
	var activeUserID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE id = ? AND status = 'active'
	`, userID).Scan(&activeUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newAppError(http.StatusForbidden, "用户不存在或已被禁用"), false
		}
		return err, false
	}

	if err := fn(tx); err != nil {
		return err, false
	}
	return tx.Commit(), false
}

// reserveGenerationBalance atomically adds a pending balance reservation.
// The conditional UPDATE is the concurrency gate: concurrent requests may
// calculate the same quote, but only requests that still have unreserved
// balance can commit a reservation.
func (r *Router) reserveGenerationBalance(ctx context.Context, tx *database.Tx, userID string, amount float64, billingMode string) error {
	amount = normalizedCreditAmount(amount)
	if amount <= 0 {
		return nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET generation_reserved_credits = COALESCE(generation_reserved_credits, 0) + ?
		WHERE id = ?
			AND status = 'active'
			AND credits - COALESCE(generation_reserved_credits, 0) >= ?
	`, amount, userID, amount)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		r.notifyBalanceInsufficient(userID)
		return newAppError(http.StatusPaymentRequired, generationBalanceInsufficientMessage(billingMode))
	}
	return nil
}

func (r *Router) reserveSubscriptionQuota(ctx context.Context, tx *database.Tx, userID string, quantity int, unitsPerImage int) error {
	if quantity < 1 || unitsPerImage < 1 {
		return nil
	}
	amount := quantity * unitsPerImage
	result, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET quota_remaining = quota_remaining - ?
		WHERE user_id = ?
			AND status = 'active'
			AND expires_at > NOW()
			AND quota_remaining >= ?
	`, amount, userID, amount)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return newAppError(http.StatusPaymentRequired, "本周期生成额度不足，请续费或升级订阅")
	}
	return nil
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

func generationAdmissionError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return newAppError(http.StatusServiceUnavailable, "请求入队繁忙，请稍后重试")
	}
	return err
}

func writeCompatGenerationAdmissionError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		writeOpenAIError(w, http.StatusServiceUnavailable, "请求入队繁忙，请稍后重试", "server_error")
		return
	}
	writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "api_error")
}

func (r *Router) logGenerationAdmissionFailure(stage string, started time.Time, userID string, err error) {
	if r == nil || r.logger == nil {
		return
	}
	stats := r.db.Raw().Stats()
	r.logger.Error("generation admission failed",
		"stage", stage,
		"elapsed", time.Since(started),
		"userId", userID,
		"error", err,
		"dbOpenConnections", stats.OpenConnections,
		"dbInUse", stats.InUse,
		"dbIdle", stats.Idle,
		"dbWaitCount", stats.WaitCount,
		"dbWaitDuration", stats.WaitDuration,
	)
}
