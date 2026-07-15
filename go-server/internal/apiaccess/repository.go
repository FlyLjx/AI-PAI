package apiaccess

import (
	"context"
	"database/sql"
	"strings"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

type accessStore interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *database.Row
}

func (r *Repository) CreateKey(ctx context.Context, key AccessKey) (*AccessKey, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO api_access_keys (id, user_id, name, key_prefix, key_hash, key_plain, status, concurrency_limit)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, key.ID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash, key.KeyPlain, key.Status, normalizedConcurrencyLimit(key.ConcurrencyLimit))
	if err != nil {
		return nil, err
	}
	return r.FindKeyByID(ctx, key.ID)
}

func (r *Repository) FindActiveByPrefix(ctx context.Context, prefix string) ([]AccessKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			api_access_keys.id,
			api_access_keys.user_id,
			users.email AS user_email,
			api_access_keys.name,
			api_access_keys.key_prefix,
			api_access_keys.key_hash,
			api_access_keys.key_plain,
			api_access_keys.status,
			api_access_keys.concurrency_limit,
			api_access_keys.last_used_at,
			api_access_keys.deleted_at,
			api_access_keys.created_at,
			api_access_keys.updated_at,
			0 AS request_count,
			0 AS success_count,
			0 AS failed_count,
			0 AS image_count,
			NULL AS last_error
		FROM api_access_keys
		LEFT JOIN users ON users.id = api_access_keys.user_id
		WHERE api_access_keys.key_prefix = ?
			AND api_access_keys.status = 'active'
			AND api_access_keys.deleted_at IS NULL
		LIMIT 50
	`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccessKeys(rows)
}

func (r *Repository) FindKeyByID(ctx context.Context, id string) (*AccessKey, error) {
	row := r.db.QueryRowContext(ctx, keyListSelect()+`
		WHERE api_access_keys.id = ?
		GROUP BY api_access_keys.id, api_access_keys.user_id, users.email, api_access_keys.name,
			api_access_keys.key_prefix, api_access_keys.key_hash, api_access_keys.key_plain, api_access_keys.status,
			api_access_keys.concurrency_limit, api_access_keys.last_used_at, api_access_keys.deleted_at,
			api_access_keys.created_at, api_access_keys.updated_at
		LIMIT 1
	`, id)
	return scanAccessKey(row)
}

func (r *Repository) ListKeys(ctx context.Context, userID string) ([]AccessKey, error) {
	where := `WHERE api_access_keys.deleted_at IS NULL`
	args := []any{}
	if strings.TrimSpace(userID) != "" {
		where += ` AND api_access_keys.user_id = ?`
		args = append(args, strings.TrimSpace(userID))
	}
	rows, err := r.db.QueryContext(ctx, keyListSelect()+where+`
		GROUP BY api_access_keys.id, api_access_keys.user_id, users.email, api_access_keys.name,
			api_access_keys.key_prefix, api_access_keys.key_hash, api_access_keys.key_plain, api_access_keys.status,
			api_access_keys.concurrency_limit, api_access_keys.last_used_at, api_access_keys.deleted_at,
			api_access_keys.created_at, api_access_keys.updated_at
		ORDER BY api_access_keys.created_at DESC, api_access_keys.id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccessKeys(rows)
}

func keyListSelect() string {
	return `
		SELECT
			api_access_keys.id,
			api_access_keys.user_id,
			users.email AS user_email,
			api_access_keys.name,
			api_access_keys.key_prefix,
			api_access_keys.key_hash,
			api_access_keys.key_plain,
			api_access_keys.status,
			api_access_keys.concurrency_limit,
			api_access_keys.last_used_at,
			api_access_keys.deleted_at,
			api_access_keys.created_at,
			api_access_keys.updated_at,
			COUNT(api_access_logs.id) AS request_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status = 'success' THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status = 'success' THEN api_access_logs.image_count ELSE 0 END), 0) AS image_count,
			MAX(CASE WHEN api_access_logs.status = 'failed' THEN api_access_logs.error_message ELSE NULL END) AS last_error
		FROM api_access_keys
		LEFT JOIN users ON users.id = api_access_keys.user_id
		LEFT JOIN api_access_logs ON api_access_logs.api_key_id = api_access_keys.id
	`
}

func (r *Repository) UpdateKeyStatus(ctx context.Context, id string, userID string, status string) (*AccessKey, error) {
	return r.UpdateKeySettings(ctx, id, userID, status, nil)
}

func (r *Repository) UpdateKeySettings(ctx context.Context, id string, userID string, status string, concurrencyLimit *int) (*AccessKey, error) {
	assignments := []string{}
	args := []any{}
	if strings.TrimSpace(status) != "" {
		assignments = append(assignments, "status = ?")
		args = append(args, strings.TrimSpace(status))
	}
	if concurrencyLimit != nil {
		assignments = append(assignments, "concurrency_limit = ?")
		args = append(args, normalizedConcurrencyLimit(*concurrencyLimit))
	}
	if len(assignments) == 0 {
		return r.FindKeyByID(ctx, id)
	}
	assignments = append(assignments, "updated_at = CURRENT_TIMESTAMP")
	where := `id = ? AND deleted_at IS NULL`
	args = append(args, id)
	if strings.TrimSpace(userID) != "" {
		where += ` AND user_id = ?`
		args = append(args, strings.TrimSpace(userID))
	}
	_, err := r.db.ExecContext(ctx, `UPDATE api_access_keys SET `+strings.Join(assignments, ", ")+` WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	return r.FindKeyByID(ctx, id)
}

func (r *Repository) DeleteKey(ctx context.Context, id string, userID string) (bool, error) {
	where := `id = ? AND deleted_at IS NULL`
	args := []any{id}
	if strings.TrimSpace(userID) != "" {
		where += ` AND user_id = ?`
		args = append(args, strings.TrimSpace(userID))
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE api_access_keys
		SET status = 'disabled', deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE `+where, args...)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (r *Repository) MarkUsed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_access_keys SET last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (r *Repository) CreateLog(ctx context.Context, log UsageLog) (*UsageLog, error) {
	return r.createLog(ctx, r.db, log)
}

func (r *Repository) CreateLogWithTx(ctx context.Context, tx *database.Tx, log UsageLog) (*UsageLog, error) {
	if tx == nil {
		return r.CreateLog(ctx, log)
	}
	return r.createLog(ctx, tx, log)
}

func (r *Repository) createLog(ctx context.Context, store accessStore, log UsageLog) (*UsageLog, error) {
	_, err := store.ExecContext(ctx, `
		INSERT INTO api_access_logs
			(id, user_id, api_key_id, task_id, endpoint, model, prompt, size, quality, quantity, image_count, response_format, status, error_message, finished_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.ID, log.UserID, log.APIKeyID, log.TaskID, log.Endpoint, log.Model, log.Prompt, log.Size, log.Quality, log.Quantity, log.ImageCount, log.ResponseFormat, log.Status, log.ErrorMessage, log.FinishedAt)
	if err != nil {
		return nil, err
	}
	row := store.QueryRowContext(ctx, usageLogSelect()+` WHERE api_access_logs.id = ? LIMIT 1`, log.ID)
	return scanUsageLog(row)
}

func (r *Repository) MarkLogsProcessingForTask(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE api_access_logs
		SET status = 'processing'
		WHERE task_id = ? AND status = 'queued'
	`, taskID)
	return err
}

func (r *Repository) FinishLog(ctx context.Context, id string, status string, imageCount int, message string) error {
	var errorMessage any
	if strings.TrimSpace(message) != "" {
		errorMessage = strings.TrimSpace(message)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE api_access_logs
		SET status = ?, image_count = ?, error_message = ?, finished_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, imageCount, errorMessage, id)
	return err
}

func (r *Repository) FinishLogsForTask(ctx context.Context, taskID string, status string, imageCount int, message string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	var errorMessage any
	if strings.TrimSpace(message) != "" {
		errorMessage = strings.TrimSpace(message)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE api_access_logs
		SET status = ?, image_count = ?, error_message = ?, finished_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
			AND (
				status IN ('queued', 'processing')
				OR (status = 'failed' AND error_message = 'context canceled')
			)
	`, status, imageCount, errorMessage, taskID)
	return err
}

func (r *Repository) SyncTerminalTaskLogs(ctx context.Context, limit int) error {
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			api_access_logs.id,
			generation_tasks.status,
			generation_tasks.quantity,
			generation_tasks.error_message
		FROM api_access_logs
		INNER JOIN generation_tasks ON generation_tasks.id = api_access_logs.task_id
		WHERE (
				api_access_logs.status IN ('queued', 'processing')
				OR (api_access_logs.status = 'failed' AND api_access_logs.error_message = 'context canceled')
			)
			AND generation_tasks.status IN ('success', 'failed', 'canceled')
		ORDER BY api_access_logs.created_at DESC, api_access_logs.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	type terminalUpdate struct {
		id           string
		status       string
		quantity     int
		errorMessage sql.NullString
	}
	updates := []terminalUpdate{}
	for rows.Next() {
		var item terminalUpdate
		if err := rows.Scan(&item.id, &item.status, &item.quantity, &item.errorMessage); err != nil {
			return err
		}
		updates = append(updates, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range updates {
		status := "failed"
		imageCount := 0
		message := strings.TrimSpace(item.errorMessage.String)
		if item.status == "success" {
			status = "success"
			imageCount = item.quantity
			if imageCount < 1 {
				imageCount = 1
			}
			message = ""
		} else if message == "" {
			if item.status == "canceled" {
				message = "任务已取消"
			} else {
				message = "图片生成失败"
			}
		}
		if err := r.FinishLog(ctx, item.id, status, imageCount, message); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) FindLogByID(ctx context.Context, id string) (*UsageLog, error) {
	row := r.db.QueryRowContext(ctx, usageLogSelect()+` WHERE api_access_logs.id = ? LIMIT 1`, id)
	return scanUsageLog(row)
}

func (r *Repository) ListLogs(ctx context.Context, input ListLogsInput) ([]UsageLog, int, error) {
	page, pageSize, offset := normalizePage(input.Page, input.PageSize)
	_ = page
	where, args := buildLogWhere(input)
	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM api_access_logs
		LEFT JOIN users ON users.id = api_access_logs.user_id
		LEFT JOIN api_access_keys ON api_access_keys.id = api_access_logs.api_key_id
		`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append(args, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, usageLogSelect()+` `+where+`
		ORDER BY api_access_logs.created_at DESC, api_access_logs.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []UsageLog{}
	for rows.Next() {
		item, err := scanUsageLog(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func usageLogSelect() string {
	return `
		SELECT
			api_access_logs.id,
			api_access_logs.user_id,
			users.email AS user_email,
			api_access_logs.api_key_id,
			api_access_keys.name AS key_name,
			api_access_keys.key_prefix,
			api_access_logs.task_id,
			api_access_logs.endpoint,
			api_access_logs.model,
			api_access_logs.prompt,
			api_access_logs.size,
			api_access_logs.quality,
			api_access_logs.quantity,
			api_access_logs.image_count,
			api_access_logs.response_format,
			api_access_logs.status,
			api_access_logs.error_message,
			api_access_logs.created_at,
			api_access_logs.finished_at
		FROM api_access_logs
		LEFT JOIN users ON users.id = api_access_logs.user_id
		LEFT JOIN api_access_keys ON api_access_keys.id = api_access_logs.api_key_id
	`
}

func buildLogWhere(input ListLogsInput) (string, []any) {
	conditions := []string{}
	args := []any{}
	if strings.TrimSpace(input.UserID) != "" {
		conditions = append(conditions, "api_access_logs.user_id = ?")
		args = append(args, strings.TrimSpace(input.UserID))
	}
	if strings.TrimSpace(input.APIKeyID) != "" {
		conditions = append(conditions, "api_access_logs.api_key_id = ?")
		args = append(args, strings.TrimSpace(input.APIKeyID))
	}
	if strings.TrimSpace(input.Status) != "" && strings.TrimSpace(input.Status) != "all" {
		conditions = append(conditions, "api_access_logs.status = ?")
		args = append(args, strings.TrimSpace(input.Status))
	}
	keyword := strings.TrimSpace(input.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		conditions = append(conditions, "(api_access_logs.model LIKE ? OR api_access_logs.prompt LIKE ? OR api_access_logs.endpoint LIKE ? OR users.email LIKE ? OR api_access_keys.key_prefix LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *Repository) AdminStats(ctx context.Context) (AdminStats, error) {
	var stats AdminStats
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total_keys,
			COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0) AS active_keys
		FROM api_access_keys
		WHERE deleted_at IS NULL
	`).Scan(&stats.TotalKeys, &stats.ActiveKeys); err != nil {
		return stats, err
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS today_requests,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) AS today_success,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS today_failed,
			COALESCE(SUM(CASE WHEN status = 'success' THEN image_count ELSE 0 END), 0) AS today_image_count
		FROM api_access_logs
		WHERE DATE(created_at) = CURRENT_DATE
	`).Scan(&stats.TodayRequests, &stats.TodaySuccess, &stats.TodayFailed, &stats.TodayImageCount); err != nil {
		return stats, err
	}
	return stats, nil
}

type accessKeyScanner interface {
	Scan(dest ...any) error
}

func scanAccessKeys(rows *sql.Rows) ([]AccessKey, error) {
	items := []AccessKey{}
	for rows.Next() {
		item, err := scanAccessKey(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanAccessKey(row accessKeyScanner) (*AccessKey, error) {
	var item AccessKey
	var userEmail, keyPlain, lastError sql.NullString
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
		&item.ConcurrencyLimit,
		&lastUsedAt,
		&deletedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.RequestCount,
		&item.SuccessCount,
		&item.FailedCount,
		&item.ImageCount,
		&lastError,
	); err != nil {
		return nil, err
	}
	if userEmail.Valid {
		item.UserEmail = &userEmail.String
	}
	if keyPlain.Valid && strings.TrimSpace(keyPlain.String) != "" {
		item.KeyPlain = &keyPlain.String
	}
	if lastUsedAt.Valid {
		value := appclock.DatabaseTime(lastUsedAt.Time)
		item.LastUsedAt = &value
	}
	if deletedAt.Valid {
		value := appclock.DatabaseTime(deletedAt.Time)
		item.DeletedAt = &value
	}
	if lastError.Valid && strings.TrimSpace(lastError.String) != "" {
		item.LastError = &lastError.String
	}
	item.CreatedAt = appclock.DatabaseTime(item.CreatedAt)
	item.UpdatedAt = appclock.DatabaseTime(item.UpdatedAt)
	return &item, nil
}

type usageLogScanner interface {
	Scan(dest ...any) error
}

func scanUsageLog(row usageLogScanner) (*UsageLog, error) {
	var item UsageLog
	var userEmail, keyName, keyPrefix, taskID, errorMessage sql.NullString
	var finishedAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&userEmail,
		&item.APIKeyID,
		&keyName,
		&keyPrefix,
		&taskID,
		&item.Endpoint,
		&item.Model,
		&item.Prompt,
		&item.Size,
		&item.Quality,
		&item.Quantity,
		&item.ImageCount,
		&item.ResponseFormat,
		&item.Status,
		&errorMessage,
		&item.CreatedAt,
		&finishedAt,
	); err != nil {
		return nil, err
	}
	if userEmail.Valid {
		item.UserEmail = &userEmail.String
	}
	if keyName.Valid {
		item.KeyName = &keyName.String
	}
	if keyPrefix.Valid {
		item.KeyPrefix = &keyPrefix.String
	}
	if taskID.Valid {
		item.TaskID = &taskID.String
	}
	if errorMessage.Valid && strings.TrimSpace(errorMessage.String) != "" {
		item.ErrorMessage = &errorMessage.String
	}
	if finishedAt.Valid {
		value := appclock.DatabaseTime(finishedAt.Time)
		item.FinishedAt = &value
	}
	item.CreatedAt = appclock.DatabaseTime(item.CreatedAt)
	return &item, nil
}

func normalizePage(page int, pageSize int) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, (page - 1) * pageSize
}
