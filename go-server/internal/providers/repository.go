package providers

import (
	"context"
	"time"

	"aipi-go/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(ctx context.Context) ([]Provider, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type, capability, base_url, api_key, status, created_at, updated_at
		FROM api_providers
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Provider{}
	for rows.Next() {
		var item Provider
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Type,
			&item.Capability,
			&item.BaseURL,
			&item.APIKey,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.CreatedAt = item.CreatedAt.In(time.Local)
		item.UpdatedAt = item.UpdatedAt.In(time.Local)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) FindByID(ctx context.Context, id string) (*Provider, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, type, capability, base_url, api_key, status, created_at, updated_at
		FROM api_providers
		WHERE id = ?
		LIMIT 1
	`, id)
	var item Provider
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Type,
		&item.Capability,
		&item.BaseURL,
		&item.APIKey,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.CreatedAt = item.CreatedAt.In(time.Local)
	item.UpdatedAt = item.UpdatedAt.In(time.Local)
	return &item, nil
}

func (r *Repository) Create(ctx context.Context, provider Provider) (*Provider, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO api_providers
			(id, name, type, capability, base_url, api_key, status)
		VALUES
			(?, ?, ?, ?, ?, ?, ?)
	`, provider.ID, provider.Name, provider.Type, provider.Capability, provider.BaseURL, provider.APIKey, provider.Status)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, provider.ID)
}

func (r *Repository) Update(ctx context.Context, id string, provider Provider) (*Provider, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE api_providers
		SET name = ?, type = ?, capability = ?, base_url = ?, api_key = ?, status = ?
		WHERE id = ?
	`, provider.Name, provider.Type, provider.Capability, provider.BaseURL, provider.APIKey, provider.Status, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM api_providers WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}
