package settings

import (
	"context"
	"strconv"

	"aipi-go/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Get(ctx context.Context) (Settings, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT setting_key, setting_value FROM system_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := Settings{}
	for key, value := range Defaults {
		result[key] = value
	}
	for rows.Next() {
		var key string
		var value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if _, ok := Defaults[key]; !ok {
			continue
		}
		result[key] = parseValue(key, value)
	}
	return result, rows.Err()
}

func (r *Repository) Update(ctx context.Context, input Settings) (Settings, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for key, value := range input {
		if _, ok := Defaults[key]; !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_settings (setting_key, setting_value)
			VALUES (?, ?)
			ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)
		`, key, serializeValue(value)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.Get(ctx)
}

func parseValue(key string, value string) any {
	if _, ok := Defaults[key].(bool); ok {
		return value == "true" || value == "1"
	}
	if _, ok := Defaults[key].(float64); ok {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return Defaults[key]
		}
		return number
	}
	return value
}

func serializeValue(value any) string {
	switch item := value.(type) {
	case string:
		return item
	case bool:
		if item {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(item, 'f', -1, 64)
	case int:
		return strconv.Itoa(item)
	default:
		return ""
	}
}
