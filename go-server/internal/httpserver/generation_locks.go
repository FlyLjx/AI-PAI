package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"aipi-go/internal/database"
)

func (r *Router) withUserGenerationLock(ctx context.Context, userID string, fn func(tx *database.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var lockedUserID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE id = ? AND status = 'active'
		FOR UPDATE
	`, userID).Scan(&lockedUserID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newAppError(http.StatusForbidden, "用户不存在或已被禁用")
		}
		return err
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
