package apikeys

import (
	"context"
	"database/sql"
	"time"

	"aipi-go/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindActiveByPrefix(ctx context.Context, prefix string) ([]APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_api_keys.id,
			user_api_keys.user_id,
			users.email AS user_email,
			user_api_keys.name,
			user_api_keys.key_prefix,
			user_api_keys.key_hash,
			user_api_keys.key_plain,
			user_api_keys.status,
			user_api_keys.last_used_at,
			user_api_keys.deleted_at,
			user_api_keys.created_at,
			user_api_keys.updated_at
		FROM user_api_keys
		LEFT JOIN users ON users.id = user_api_keys.user_id
		WHERE user_api_keys.key_prefix = ?
			AND user_api_keys.status = 'active'
			AND user_api_keys.deleted_at IS NULL
		LIMIT 20
	`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []APIKey{}
	for rows.Next() {
		item, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) FindByUserID(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_api_keys.id,
			user_api_keys.user_id,
			users.email AS user_email,
			user_api_keys.name,
			user_api_keys.key_prefix,
			user_api_keys.key_hash,
			user_api_keys.key_plain,
			user_api_keys.status,
			user_api_keys.last_used_at,
			user_api_keys.deleted_at,
			user_api_keys.created_at,
			user_api_keys.updated_at
		FROM user_api_keys
		LEFT JOIN users ON users.id = user_api_keys.user_id
		WHERE user_api_keys.user_id = ?
			AND user_api_keys.deleted_at IS NULL
		ORDER BY user_api_keys.created_at DESC, user_api_keys.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []APIKey{}
	for rows.Next() {
		item, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) FindAll(ctx context.Context) ([]APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_api_keys.id,
			user_api_keys.user_id,
			users.email AS user_email,
			user_api_keys.name,
			user_api_keys.key_prefix,
			user_api_keys.key_hash,
			user_api_keys.key_plain,
			user_api_keys.status,
			user_api_keys.last_used_at,
			user_api_keys.deleted_at,
			user_api_keys.created_at,
			user_api_keys.updated_at
		FROM user_api_keys
		LEFT JOIN users ON users.id = user_api_keys.user_id
		WHERE user_api_keys.deleted_at IS NULL
		ORDER BY user_api_keys.created_at DESC, user_api_keys.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []APIKey{}
	for rows.Next() {
		item, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) FindByID(ctx context.Context, id string) (*APIKey, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			user_api_keys.id,
			user_api_keys.user_id,
			users.email AS user_email,
			user_api_keys.name,
			user_api_keys.key_prefix,
			user_api_keys.key_hash,
			user_api_keys.key_plain,
			user_api_keys.status,
			user_api_keys.last_used_at,
			user_api_keys.deleted_at,
			user_api_keys.created_at,
			user_api_keys.updated_at
		FROM user_api_keys
		LEFT JOIN users ON users.id = user_api_keys.user_id
		WHERE user_api_keys.id = ?
		LIMIT 1
	`, id)
	return scanAPIKey(row)
}

func (r *Repository) Create(ctx context.Context, key APIKey) (*APIKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if key.Status == "active" {
		if _, err := tx.ExecContext(ctx, `UPDATE user_api_keys SET status = 'disabled' WHERE user_id = ? AND status = 'active'`, key.UserID); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_api_keys (id, user_id, name, key_prefix, key_hash, key_plain, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, key.ID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash, key.KeyPlain, key.Status)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, key.ID)
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status string, userID string) (*APIKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if status == "active" {
		if _, err := tx.ExecContext(ctx, `UPDATE user_api_keys SET status = 'disabled' WHERE user_id = ? AND id <> ? AND status = 'active'`, userID, id); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_api_keys SET status = ? WHERE id = ? AND deleted_at IS NULL`, status, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) DeleteByUserID(ctx context.Context, id string, userID string) (bool, error) {
	where := `id = ?`
	args := []any{id}
	if userID != "" {
		where += ` AND user_id = ?`
		args = append(args, userID)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE user_api_keys
		SET status = 'disabled', deleted_at = COALESCE(deleted_at, CURRENT_TIMESTAMP)
		WHERE `+where, args...)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (r *Repository) MarkUsed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE user_api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAPIKey(row scanner) (*APIKey, error) {
	var item APIKey
	var userEmail, keyPlain sql.NullString
	var lastUsedAt, deletedAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&userEmail,
		&item.Name,
		&item.KeyPrefix,
		&item.KeyHash,
		&keyPlain,
		&item.Status,
		&lastUsedAt,
		&deletedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if userEmail.Valid {
		item.UserEmail = &userEmail.String
	}
	if keyPlain.Valid {
		item.KeyPlain = &keyPlain.String
	}
	if lastUsedAt.Valid {
		value := lastUsedAt.Time.In(time.Local)
		item.LastUsedAt = &value
	}
	if deletedAt.Valid {
		value := deletedAt.Time.In(time.Local)
		item.DeletedAt = &value
	}
	item.CreatedAt = item.CreatedAt.In(time.Local)
	item.UpdatedAt = item.UpdatedAt.In(time.Local)
	return &item, nil
}
