package pricing

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"aipi-go/internal/database"
)

const (
	MinUserModelUnitPrice = 0.001
	MaxUserModelUnitPrice = 99999999.9999
)

var ErrInvalidUserModelUnitPrice = errors.New("用户模型单价必须在 0.001 到 99999999.9999 之间")

type UserModelPriceOverride struct {
	ID               string    `json:"id"`
	UserID           string    `json:"userId"`
	UserEmail        string    `json:"userEmail"`
	ModelID          string    `json:"modelId"`
	ModelName        string    `json:"modelName"`
	ModelDisplayName string    `json:"modelDisplayName"`
	UnitPrice        float64   `json:"unitPrice"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type UserModelPriceRepository struct {
	db *database.DB
}

func NewUserModelPriceRepository(db *database.DB) *UserModelPriceRepository {
	return &UserModelPriceRepository{db: db}
}

func NormalizeUserModelUnitPrice(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < MinUserModelUnitPrice || value > MaxUserModelUnitPrice {
		return 0, false
	}
	return math.Round(value*10000) / 10000, true
}

func (r *UserModelPriceRepository) FindAll(ctx context.Context) ([]UserModelPriceOverride, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_model_price_overrides.id,
			user_model_price_overrides.user_id,
			users.email,
			user_model_price_overrides.model_id,
			ai_models.model_name,
			ai_models.display_name,
			user_model_price_overrides.unit_price,
			user_model_price_overrides.created_at,
			user_model_price_overrides.updated_at
		FROM user_model_price_overrides
		INNER JOIN users ON users.id = user_model_price_overrides.user_id
		INNER JOIN ai_models ON ai_models.id = user_model_price_overrides.model_id
		WHERE users.role <> 'admin'
			AND ai_models.capability = 'chat_image'
		ORDER BY user_model_price_overrides.created_at DESC, user_model_price_overrides.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserModelPriceOverride, 0)
	for rows.Next() {
		var item UserModelPriceOverride
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.UserEmail,
			&item.ModelID,
			&item.ModelName,
			&item.ModelDisplayName,
			&item.UnitPrice,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *UserModelPriceRepository) FindForUserAndModel(ctx context.Context, userID string, modelID string) (float64, bool, error) {
	return findUserModelUnitPrice(ctx, r.db, userID, modelID)
}

func (r *UserModelPriceRepository) FindForUserAndModelTx(ctx context.Context, tx *database.Tx, userID string, modelID string) (float64, bool, error) {
	return findUserModelUnitPrice(ctx, tx, userID, modelID)
}

func findUserModelUnitPrice(ctx context.Context, source interface {
	QueryRowContext(context.Context, string, ...any) *database.Row
}, userID string, modelID string) (float64, bool, error) {
	var unitPrice float64
	err := source.QueryRowContext(ctx, `
		SELECT unit_price
		FROM user_model_price_overrides
		WHERE user_id = ? AND model_id = ?
		LIMIT 1
	`, userID, modelID).Scan(&unitPrice)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return unitPrice, true, nil
}

func (r *UserModelPriceRepository) Save(ctx context.Context, item UserModelPriceOverride) (*UserModelPriceOverride, error) {
	unitPrice, ok := NormalizeUserModelUnitPrice(item.UnitPrice)
	if !ok {
		return nil, ErrInvalidUserModelUnitPrice
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_model_price_overrides (id, user_id, model_id, unit_price)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			unit_price = VALUES(unit_price),
			updated_at = CURRENT_TIMESTAMP
	`, item.ID, item.UserID, item.ModelID, unitPrice)
	if err != nil {
		return nil, err
	}
	return r.findByUserAndModel(ctx, item.UserID, item.ModelID)
}

func (r *UserModelPriceRepository) findByUserAndModel(ctx context.Context, userID string, modelID string) (*UserModelPriceOverride, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			user_model_price_overrides.id,
			user_model_price_overrides.user_id,
			users.email,
			user_model_price_overrides.model_id,
			ai_models.model_name,
			ai_models.display_name,
			user_model_price_overrides.unit_price,
			user_model_price_overrides.created_at,
			user_model_price_overrides.updated_at
		FROM user_model_price_overrides
		INNER JOIN users ON users.id = user_model_price_overrides.user_id
		INNER JOIN ai_models ON ai_models.id = user_model_price_overrides.model_id
		WHERE user_model_price_overrides.user_id = ?
			AND user_model_price_overrides.model_id = ?
		LIMIT 1
	`, userID, modelID)
	var item UserModelPriceOverride
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.UserEmail,
		&item.ModelID,
		&item.ModelName,
		&item.ModelDisplayName,
		&item.UnitPrice,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *UserModelPriceRepository) Delete(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM user_model_price_overrides WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}
