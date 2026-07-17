package operations

import (
	"context"
	"log/slog"
	"time"

	"aipi-go/internal/database"
)

const (
	PendingOrderLifetime    = 30 * time.Minute
	orderExpiryScanInterval = 15 * time.Second
)

func (r *Repository) CloseExpiredPendingOrders(ctx context.Context, now time.Time) (int, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE recharge_orders
		SET status='closed', pay_url=NULL, qr_code=NULL, updated_at=CURRENT_TIMESTAMP
		WHERE status='pending' AND created_at <= ?
	`, now.Add(-PendingOrderLifetime))
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	return int(rows), err
}

func StartOrderExpiryWorker(ctx context.Context, db *database.DB, logger *slog.Logger) {
	repo := NewRepository(db)
	closeExpired := func() {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		closed, err := repo.CloseExpiredPendingOrders(checkCtx, time.Now())
		if err != nil {
			if ctx.Err() == nil && logger != nil {
				logger.Warn("expired order cleanup failed", "error", err)
			}
			return
		}
		if closed > 0 && logger != nil {
			logger.Info("expired pending orders closed", "count", closed)
		}
	}
	go func() {
		closeExpired()
		ticker := time.NewTicker(orderExpiryScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				closeExpired()
			}
		}
	}()
}
