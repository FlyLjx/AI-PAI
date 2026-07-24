package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"aipi-go/internal/database"
)

var (
	ErrAmbiguousModelName        = errors.New("ambiguous active model match")
	ErrDuplicateModelDisplayName = errors.New("duplicate model display name")
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(ctx context.Context) ([]Model, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			ai_models.id,
			ai_models.provider_id,
			api_providers.name AS provider_name,
			api_providers.type AS provider_type,
			api_providers.status AS provider_status,
			ai_models.model_name,
			ai_models.display_name,
			ai_models.capability,
			ai_models.cost_1k,
			ai_models.cost_2k,
			ai_models.cost_4k,
			ai_models.markup_percent,
			ai_models.price_change_percent,
			ai_models.price_1k,
			ai_models.price_2k,
			ai_models.price_4k,
			ai_models.append_size_to_prompt,
			ai_models.enabled_size_tiers,
			ai_models.sort_order,
			ai_models.status,
			ai_models.created_at,
			ai_models.updated_at
		FROM ai_models
		LEFT JOIN api_providers ON api_providers.id = ai_models.provider_id
		WHERE ai_models.deleted_at IS NULL
		ORDER BY
			ai_models.capability ASC,
			ai_models.sort_order ASC,
			api_providers.name ASC,
			ai_models.model_name ASC,
			ai_models.created_at DESC,
			ai_models.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Model{}
	for rows.Next() {
		item, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) FindByID(ctx context.Context, id string) (*Model, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			ai_models.id,
			ai_models.provider_id,
			api_providers.name AS provider_name,
			api_providers.type AS provider_type,
			api_providers.status AS provider_status,
			ai_models.model_name,
			ai_models.display_name,
			ai_models.capability,
			ai_models.cost_1k,
			ai_models.cost_2k,
			ai_models.cost_4k,
			ai_models.markup_percent,
			ai_models.price_change_percent,
			ai_models.price_1k,
			ai_models.price_2k,
			ai_models.price_4k,
			ai_models.append_size_to_prompt,
			ai_models.enabled_size_tiers,
			ai_models.sort_order,
			ai_models.status,
			ai_models.created_at,
			ai_models.updated_at
		FROM ai_models
		LEFT JOIN api_providers ON api_providers.id = ai_models.provider_id
		WHERE ai_models.id = ?
		LIMIT 1
	`, id)
	return scanModel(row)
}

func (r *Repository) FindActiveByNameOrDisplayName(ctx context.Context, name string) (*Model, error) {
	items, err := r.findActiveChatImageModelsByColumn(ctx, "display_name", name)
	if err != nil {
		return nil, err
	}
	if len(items) == 1 {
		return &items[0], nil
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("%w: 模型 %q 匹配到多个公开模型配置，请确保模型 ID 唯一", ErrAmbiguousModelName, name)
	}

	items, err = r.findActiveChatImageModelsByColumn(ctx, "model_name", name)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}
	if len(items) > 1 {
		return nil, fmt.Errorf("%w: 模型 %q 匹配到多个上游渠道，请使用 /v1/models 返回的精确模型 ID", ErrAmbiguousModelName, name)
	}
	return &items[0], nil
}

func (r *Repository) findActiveChatImageModelsByColumn(ctx context.Context, column string, value string) ([]Model, error) {
	var predicate string
	switch column {
	case "display_name":
		predicate = "ai_models.display_name = ?"
	case "model_name":
		predicate = "ai_models.model_name = ?"
	default:
		return nil, fmt.Errorf("unsupported model lookup column %q", column)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			ai_models.id,
			ai_models.provider_id,
			api_providers.name AS provider_name,
			api_providers.type AS provider_type,
			api_providers.status AS provider_status,
			ai_models.model_name,
			ai_models.display_name,
			ai_models.capability,
			ai_models.cost_1k,
			ai_models.cost_2k,
			ai_models.cost_4k,
			ai_models.markup_percent,
			ai_models.price_change_percent,
			ai_models.price_1k,
			ai_models.price_2k,
			ai_models.price_4k,
			ai_models.append_size_to_prompt,
			ai_models.enabled_size_tiers,
			ai_models.sort_order,
			ai_models.status,
			ai_models.created_at,
			ai_models.updated_at
		FROM ai_models
		LEFT JOIN api_providers ON api_providers.id = ai_models.provider_id
		WHERE ai_models.capability = 'chat_image'
			AND ai_models.status = 'active'
			AND api_providers.status = 'active'
			AND ai_models.deleted_at IS NULL
			AND %s
		ORDER BY
			ai_models.sort_order ASC,
			ai_models.created_at DESC,
			ai_models.id ASC
		LIMIT 2
	`, predicate), value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Model{}
	for rows.Next() {
		item, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) FindByProviderNameAndCapability(ctx context.Context, providerID string, modelName string, capability string) (*Model, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			ai_models.id,
			ai_models.provider_id,
			api_providers.name AS provider_name,
			api_providers.type AS provider_type,
			api_providers.status AS provider_status,
			ai_models.model_name,
			ai_models.display_name,
			ai_models.capability,
			ai_models.cost_1k,
			ai_models.cost_2k,
			ai_models.cost_4k,
			ai_models.markup_percent,
			ai_models.price_change_percent,
			ai_models.price_1k,
			ai_models.price_2k,
			ai_models.price_4k,
			ai_models.append_size_to_prompt,
			ai_models.enabled_size_tiers,
			ai_models.sort_order,
			ai_models.status,
			ai_models.created_at,
			ai_models.updated_at
		FROM ai_models
		LEFT JOIN api_providers ON api_providers.id = ai_models.provider_id
		WHERE ai_models.provider_id = ?
			AND ai_models.model_name = ?
			AND ai_models.capability = ?
		LIMIT 1
	`, providerID, modelName, capability)
	return scanModel(row)
}

func (r *Repository) Create(ctx context.Context, model Model) (*Model, error) {
	excludeID := model.ID
	existing, err := r.FindByProviderNameAndCapability(ctx, model.ProviderID, model.ModelName, model.Capability)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		excludeID = existing.ID
	}
	if err := r.ensureUniqueDisplayName(ctx, model.DisplayName, model.Capability, excludeID); err != nil {
		return nil, err
	}

	tiers, _ := json.Marshal(ParseEnabledSizeTiersFromStrings(model.EnabledSizeTiers))
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO ai_models
			(id, provider_id, model_name, display_name, capability,
			 cost_1k, cost_2k, cost_4k, markup_percent,
			 price_change_percent, price_1k, price_2k, price_4k,
			 append_size_to_prompt, enabled_size_tiers, sort_order, status)
		VALUES
			(?, ?, ?, ?, ?,
			 ?, ?, ?, ?,
			 ?, ?, ?, ?,
			 ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			display_name = VALUES(display_name),
			cost_1k = VALUES(cost_1k),
			cost_2k = VALUES(cost_2k),
			cost_4k = VALUES(cost_4k),
			markup_percent = VALUES(markup_percent),
			price_change_percent = VALUES(price_change_percent),
			price_1k = VALUES(price_1k),
			price_2k = VALUES(price_2k),
			price_4k = VALUES(price_4k),
			append_size_to_prompt = VALUES(append_size_to_prompt),
			enabled_size_tiers = VALUES(enabled_size_tiers),
			sort_order = VALUES(sort_order),
			updated_at = CURRENT_TIMESTAMP
	`, model.ID, model.ProviderID, model.ModelName, model.DisplayName, model.Capability,
		model.Cost1K, model.Cost2K, model.Cost4K, model.MarkupPercent,
		model.PriceChangePercent, model.Price1K, model.Price2K, model.Price4K,
		model.AppendSizeToPrompt, string(tiers), model.SortOrder, model.Status)
	if err != nil {
		return nil, err
	}
	return r.FindByProviderNameAndCapability(ctx, model.ProviderID, model.ModelName, model.Capability)
}

func (r *Repository) Update(ctx context.Context, id string, model Model) (*Model, error) {
	if err := r.ensureUniqueDisplayName(ctx, model.DisplayName, model.Capability, id); err != nil {
		return nil, err
	}

	tiers, _ := json.Marshal(ParseEnabledSizeTiersFromStrings(model.EnabledSizeTiers))
	_, err := r.db.ExecContext(ctx, `
		UPDATE ai_models
		SET provider_id = ?,
			model_name = ?,
			display_name = ?,
			capability = ?,
			cost_1k = ?,
			cost_2k = ?,
			cost_4k = ?,
			markup_percent = ?,
			price_change_percent = ?,
			price_1k = ?,
			price_2k = ?,
			price_4k = ?,
			append_size_to_prompt = ?,
			enabled_size_tiers = ?,
			sort_order = ?,
			status = ?
		WHERE id = ?
	`, model.ProviderID, model.ModelName, model.DisplayName, model.Capability,
		model.Cost1K, model.Cost2K, model.Cost4K, model.MarkupPercent,
		model.PriceChangePercent, model.Price1K, model.Price2K, model.Price4K,
		model.AppendSizeToPrompt, string(tiers), model.SortOrder, model.Status, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) ensureUniqueDisplayName(ctx context.Context, displayName string, capability string, excludeID string) error {
	var existingID string
	err := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM ai_models
		WHERE capability = ?
			AND LOWER(display_name) = LOWER(?)
			AND (? = '' OR id <> ?)
		LIMIT 1
	`, capability, displayName, excludeID, excludeID).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: 对外模型名称 %q 已存在，请换一个唯一名称", ErrDuplicateModelDisplayName, displayName)
}

func (r *Repository) Delete(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ai_models
		SET status = 'disabled',
			deleted_at = COALESCE(deleted_at, CURRENT_TIMESTAMP),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (r *Repository) DeleteByProviderID(ctx context.Context, providerID string) (int64, int64, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE ai_models
		SET status = 'disabled',
			deleted_at = COALESCE(deleted_at, CURRENT_TIMESTAMP),
			updated_at = CURRENT_TIMESTAMP
		WHERE provider_id = ?
			AND deleted_at IS NULL
	`, providerID)
	if err != nil {
		return 0, 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	return deleted, 0, nil
}

func (r *Repository) CountTaskReferences(ctx context.Context, id string) (int64, error) {
	row := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_tasks WHERE model_id = ?`, id)
	var total int64
	err := row.Scan(&total)
	return total, err
}

func (r *Repository) Disable(ctx context.Context, id string) (*Model, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE ai_models SET status = 'disabled' WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) UpdateSortOrders(ctx context.Context, items []SortOrderItem) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `UPDATE ai_models SET sort_order = ? WHERE id = ?`, item.SortOrder, item.ID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(items), nil
}

type SortOrderItem struct {
	ID        string
	SortOrder int
}

type modelScanner interface {
	Scan(dest ...any) error
}

func scanModel(row modelScanner) (*Model, error) {
	var item Model
	var providerName, providerType, providerStatus sql.NullString
	var enabledSizeTiers sql.NullString
	var appendSizeToPrompt bool
	if err := row.Scan(
		&item.ID,
		&item.ProviderID,
		&providerName,
		&providerType,
		&providerStatus,
		&item.ModelName,
		&item.DisplayName,
		&item.Capability,
		&item.Cost1K,
		&item.Cost2K,
		&item.Cost4K,
		&item.MarkupPercent,
		&item.PriceChangePercent,
		&item.Price1K,
		&item.Price2K,
		&item.Price4K,
		&appendSizeToPrompt,
		&enabledSizeTiers,
		&item.SortOrder,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if providerName.Valid {
		item.ProviderName = &providerName.String
	}
	if providerType.Valid {
		item.ProviderType = &providerType.String
	}
	if providerStatus.Valid {
		item.ProviderStatus = &providerStatus.String
	}
	if enabledSizeTiers.Valid {
		item.EnabledSizeTiers = ParseEnabledSizeTiers([]byte(enabledSizeTiers.String))
	} else {
		item.EnabledSizeTiers = ParseEnabledSizeTiers(nil)
	}
	item.AppendSizeToPrompt = appendSizeToPrompt
	item.CreatedAt = item.CreatedAt.In(time.Local)
	item.UpdatedAt = item.UpdatedAt.In(time.Local)
	return &item, nil
}
