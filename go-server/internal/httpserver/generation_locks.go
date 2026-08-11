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
	// Admission must stay database-only. The old implementation held the user
	// row lock while calculating the quote and inserting the task, which made
	// every request for one account wait behind the previous request and turn
	// normal concurrency into a 5-second 429. Balance reservation below uses a
	// single conditional UPDATE, so the row is locked only for that statement.
	generationLockRetryCount = 3
	generationEnqueueTimeout = 30 * time.Second
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
// read the same quote, but only requests that still have unreserved balance
// can commit a reservation. It holds the user row lock for one UPDATE instead
// of the full admission transaction.
func reserveGenerationBalance(ctx context.Context, tx *database.Tx, userID string, amount float64, billingMode string) error {
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
		return newAppError(http.StatusPaymentRequired, generationBalanceInsufficientMessage(billingMode))
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
