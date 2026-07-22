package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

type taskStore interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *database.Row
}

func (r *Repository) Create(ctx context.Context, task Task) (*Task, error) {
	return r.create(ctx, r.db, task)
}

func (r *Repository) CreateWithTx(ctx context.Context, tx *database.Tx, task Task) (*Task, error) {
	if tx == nil {
		return r.Create(ctx, task)
	}
	return r.create(ctx, tx, task)
}

func (r *Repository) create(ctx context.Context, store taskStore, task Task) (*Task, error) {
	resultJSON := any(nil)
	if task.ResultJSON != nil {
		bytes, _ := json.Marshal(task.ResultJSON)
		resultJSON = string(bytes)
	}
	_, err := store.ExecContext(ctx, `
		INSERT INTO generation_tasks
			(id, user_id, model_id, provider_id, capability, prompt, reference_image_url, size_tier, size, output_format, transparent_background, quantity, user_ip,
			 cost_credits, model_cost_credits, remaining_credits, duration_seconds, status, error_message, result_json,
			 favorite_enabled, public_status, display_enabled, display_note)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			 ?, ?, ?, ?, ?, ?, ?,
			 ?, ?, ?, ?)
	`, task.ID, task.UserID, task.ModelID, task.ProviderID, task.Capability, task.Prompt, task.ReferenceImageURL, task.SizeTier, task.Size, task.OutputFormat, task.TransparentBackground, task.Quantity, task.UserIP,
		task.CostCredits, task.ModelCostCredits, task.RemainingCredits, task.DurationSeconds, task.Status, task.ErrorMessage, resultJSON,
		task.FavoriteEnabled, task.PublicStatus, task.DisplayEnabled, task.DisplayNote)
	if err != nil {
		return nil, err
	}
	row := store.QueryRowContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		WHERE generation_tasks.id = ?
		LIMIT 1
	`, task.ID)
	return scanTask(row)
}

func (r *Repository) FindByID(ctx context.Context, id string) (*Task, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		WHERE generation_tasks.id = ?
		LIMIT 1
	`, id)
	return scanTask(row)
}

func (r *Repository) FindAll(ctx context.Context, input ListInput) ([]Task, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildTaskWhere(input.Keyword, input.Status, input.Display)
	total, err := r.count(ctx, where, args)
	if err != nil {
		return nil, 0, err
	}
	queryArgs := append(args, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		`+where+`
		ORDER BY generation_tasks.created_at DESC, generation_tasks.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanTasks(rows)
	return items, total, err
}

func (r *Repository) FindAdminList(ctx context.Context, input ListInput) ([]AdminTaskListItem, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildTaskWhere(input.Keyword, input.Status, input.Display)
	total, err := r.count(ctx, where, args)
	if err != nil {
		return nil, 0, err
	}
	queryArgs := append(args, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			generation_tasks.id,
			generation_tasks.user_id,
			users.email AS user_email,
			generation_tasks.model_id,
			ai_models.model_name,
			ai_models.display_name AS model_display_name,
			generation_tasks.size_tier,
			generation_tasks.size,
			generation_tasks.quantity,
			generation_tasks.user_ip,
			generation_tasks.cost_credits,
			generation_tasks.duration_seconds,
			generation_tasks.status,
			generation_tasks.error_message,
			generation_tasks.created_at,
			user_subscriptions.plan_name AS user_subscription_plan_name
		FROM generation_tasks
		LEFT JOIN users ON users.id = generation_tasks.user_id
		LEFT JOIN ai_models ON ai_models.id = generation_tasks.model_id
		LEFT JOIN (
			SELECT user_subscriptions.user_id, MAX(subscription_plans.name) AS plan_name
			FROM user_subscriptions
			LEFT JOIN subscription_plans ON subscription_plans.id = user_subscriptions.plan_id
			WHERE user_subscriptions.status = 'active' AND user_subscriptions.expires_at > NOW()
			GROUP BY user_subscriptions.user_id
		) user_subscriptions ON user_subscriptions.user_id = generation_tasks.user_id
		`+where+`
		ORDER BY generation_tasks.created_at DESC, generation_tasks.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []AdminTaskListItem{}
	for rows.Next() {
		item, err := scanAdminTaskListItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *Repository) FindAllForExport(ctx context.Context) ([]Task, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		ORDER BY generation_tasks.created_at DESC, generation_tasks.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *Repository) FindImages(ctx context.Context, input ListInput) ([]Task, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildTaskWhere(input.Keyword, "", input.Display)
	if where == "" {
		where = "WHERE generation_tasks.status = 'success' AND generation_tasks.result_json IS NOT NULL"
	} else {
		where += " AND generation_tasks.status = 'success' AND generation_tasks.result_json IS NOT NULL"
	}
	total, err := r.count(ctx, where, args)
	if err != nil {
		return nil, 0, err
	}
	queryArgs := append(args, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		`+where+`
		ORDER BY generation_tasks.created_at DESC, generation_tasks.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanTasks(rows)
	return items, total, err
}

func (r *Repository) FindByUserID(ctx context.Context, userID string, page int, pageSize int) ([]Task, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM generation_tasks WHERE user_id = ?`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		WHERE user_id = ?
		ORDER BY generation_tasks.created_at DESC, generation_tasks.id DESC
		LIMIT ? OFFSET ?
	`, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Task{}
	items, err = scanTasks(rows)
	return items, total, err
}

func (r *Repository) FindFavoritesByUserID(ctx context.Context, userID string, input ListInput) ([]Task, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	conditions := []string{
		"generation_tasks.user_id = ?",
		"generation_tasks.favorite_enabled = TRUE",
		"generation_tasks.status = 'success'",
		"generation_tasks.result_json IS NOT NULL",
	}
	args := []any{userID}
	appendKeywordWhere(&conditions, &args, input.Keyword)
	where := "WHERE " + strings.Join(conditions, " AND ")
	total, err := r.count(ctx, where, args)
	if err != nil {
		return nil, 0, err
	}
	queryArgs := append(args, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		`+where+`
		ORDER BY generation_tasks.updated_at DESC, generation_tasks.created_at DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanTasks(rows)
	return items, total, err
}

func (r *Repository) FindPublicDisplay(ctx context.Context) ([]Task, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		WHERE generation_tasks.public_status = 'approved'
			AND generation_tasks.display_enabled = TRUE
			AND generation_tasks.status = 'success'
			AND generation_tasks.result_json IS NOT NULL
		ORDER BY generation_tasks.updated_at DESC, generation_tasks.created_at DESC
		LIMIT 60
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status Status) (*Task, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE generation_tasks SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) ClaimForProcessing(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'processing',
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status IN ('queued', 'pending')
	`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (r *Repository) FinishSuccess(ctx context.Context, id string, result any, durationSeconds float64) (*Task, error) {
	bytes, _ := json.Marshal(result)
	_, err := r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'success',
			result_json = ?,
			error_message = NULL,
			duration_seconds = ?
		WHERE id = ?
	`, string(bytes), durationSeconds, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) FinishFailed(ctx context.Context, id string, message string, durationSeconds float64) (*Task, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'failed',
			error_message = ?,
			duration_seconds = ?
		WHERE id = ? AND status = 'processing'
	`, message, durationSeconds, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) FailTimedOut(ctx context.Context, cutoff time.Time, now time.Time, message string, limit int) ([]string, error) {
	return r.failTimedOut(ctx, cutoff, now, message, limit, false)
}

func (r *Repository) FailTimedOutProcessing(ctx context.Context, cutoff time.Time, now time.Time, message string, limit int) ([]string, error) {
	return r.failTimedOut(ctx, cutoff, now, message, limit, true)
}

func (r *Repository) failTimedOut(ctx context.Context, cutoff time.Time, now time.Time, message string, limit int, processingOnly bool) ([]string, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	cutoff = appclock.DatabaseTime(cutoff)
	now = appclock.DatabaseTime(now)
	where := `status IN ('queued', 'pending', 'processing')`
	if processingOnly {
		where = `status = 'processing'`
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, created_at
		FROM generation_tasks
		WHERE `+where+`
			AND updated_at <= ?
		ORDER BY created_at ASC, id ASC
		LIMIT ?
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		id        string
		createdAt time.Time
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.createdAt); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	failedIDs := make([]string, 0, len(candidates))
	for _, item := range candidates {
		createdAt := appclock.DatabaseTime(item.createdAt)
		durationSeconds := now.Sub(createdAt).Seconds()
		if durationSeconds < 0 {
			durationSeconds = 0
		}
		result, err := r.db.ExecContext(ctx, `
			UPDATE generation_tasks
			SET status = 'failed',
				error_message = ?,
				duration_seconds = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
				AND `+where+`
				AND updated_at <= ?
		`, message, durationSeconds, item.id, cutoff)
		if err != nil {
			return failedIDs, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return failedIDs, err
		}
		if affected == 1 {
			failedIDs = append(failedIDs, item.id)
		}
	}
	return failedIDs, nil
}

func (r *Repository) TouchWaiting(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET updated_at = CURRENT_TIMESTAMP
		WHERE status IN ('queued', 'pending')
	`)
	return err
}

func (r *Repository) Cancel(ctx context.Context, id string) (*Task, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'canceled',
			error_message = '任务已取消'
		WHERE id = ?
			AND status IN ('queued', 'processing', 'pending')
	`, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Stats(ctx context.Context) (Stats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			status,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = 'success' THEN quantity ELSE 0 END), 0) AS total_images
		FROM generation_tasks
		GROUP BY status
	`)
	if err != nil {
		return Stats{}, err
	}
	defer rows.Close()
	stats := Stats{}
	for rows.Next() {
		var status string
		var total int
		var images int
		if err := rows.Scan(&status, &total, &images); err != nil {
			return Stats{}, err
		}
		stats.Total += total
		stats.TotalImages += images
		switch Status(status) {
		case StatusQueued:
			stats.Queued = total
		case StatusPending:
			stats.Pending = total
		case StatusProcessing:
			stats.Processing = total
		case StatusSuccess:
			stats.Success = total
		case StatusFailed:
			stats.Failed = total
		case StatusCanceled:
			stats.Canceled = total
		}
	}
	return stats, rows.Err()
}

func (r *Repository) UpdateDisplay(ctx context.Context, id string, displayEnabled bool, displayNote string) (*Task, error) {
	task, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.Status != StatusSuccess || len(ResultURLs(task.ResultJSON)) == 0 {
		return nil, ErrNoResultImage
	}
	publicStatus := "private"
	if displayEnabled {
		publicStatus = "approved"
	}
	var note any
	if strings.TrimSpace(displayNote) != "" {
		note = strings.TrimSpace(displayNote)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET display_enabled = ?,
			public_status = ?,
			public_reviewed_at = NOW(),
			display_note = ?
		WHERE id = ?
	`, displayEnabled, publicStatus, note, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) ReviewPublic(ctx context.Context, id string, status string, displayNote string) (*Task, error) {
	task, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.Status != StatusSuccess || len(ResultURLs(task.ResultJSON)) == 0 {
		return nil, ErrNoResultImage
	}
	if status != "approved" && status != "rejected" {
		return nil, ErrInvalidPublicStatus
	}
	displayEnabled := status == "approved"
	note := strings.TrimSpace(displayNote)
	if note == "" {
		if task.DisplayNote != nil {
			note = *task.DisplayNote
		} else {
			note = task.Prompt
		}
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET public_status = ?,
			public_reviewed_at = NOW(),
			display_enabled = ?,
			display_note = ?
		WHERE id = ?
	`, status, displayEnabled, note, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) UpdateFavorite(ctx context.Context, id string, userID string, favoriteEnabled bool) (*Task, error) {
	task, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if userID != "" && task.UserID != userID {
		return nil, ErrForbiddenTask
	}
	if task.Status != StatusSuccess || len(ResultURLs(task.ResultJSON)) == 0 {
		return nil, ErrNoResultImage
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET favorite_enabled = ?
		WHERE id = ?
	`, favoriteEnabled, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) RequestPublic(ctx context.Context, id string, userID string, displayNote string) (*Task, error) {
	task, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if userID != "" && task.UserID != userID {
		return nil, ErrForbiddenTask
	}
	if task.Status != StatusSuccess || len(ResultURLs(task.ResultJSON)) == 0 {
		return nil, ErrNoResultImage
	}
	note := strings.TrimSpace(displayNote)
	if note == "" {
		if task.DisplayNote != nil {
			note = *task.DisplayNote
		} else {
			note = task.Prompt
		}
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET public_status = 'pending',
			public_requested_at = NOW(),
			public_reviewed_at = NULL,
			display_enabled = FALSE,
			display_note = ?
		WHERE id = ?
	`, note, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) ImageURLByIndex(ctx context.Context, id string, index int) (string, error) {
	task, err := r.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	urls := ResultURLs(task.ResultJSON)
	if task.Status != StatusSuccess || index < 0 || index >= len(urls) {
		return "", sql.ErrNoRows
	}
	return RewriteImageURL(task.ProviderBaseURL, urls[index]), nil
}

type ListInput struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
	Display  string
}

const taskSelectColumns = `
	generation_tasks.id,
	generation_tasks.user_id,
	generation_tasks.model_id,
	generation_tasks.provider_id,
	generation_tasks.capability,
	generation_tasks.prompt,
	generation_tasks.reference_image_url,
	generation_tasks.size_tier,
	generation_tasks.size,
	generation_tasks.output_format,
	generation_tasks.transparent_background,
	generation_tasks.quantity,
	generation_tasks.user_ip,
	generation_tasks.cost_credits,
	generation_tasks.model_cost_credits,
	generation_tasks.remaining_credits,
	generation_tasks.duration_seconds,
	generation_tasks.status,
	generation_tasks.error_message,
	generation_tasks.result_json,
	generation_tasks.favorite_enabled,
	generation_tasks.public_status,
	generation_tasks.public_requested_at,
	generation_tasks.public_reviewed_at,
	generation_tasks.display_enabled,
	generation_tasks.display_note,
	generation_tasks.created_at,
	generation_tasks.updated_at,
	users.email AS user_email,
	ai_models.model_name,
	ai_models.display_name AS model_display_name,
	api_providers.name AS provider_name,
	api_providers.base_url AS provider_base_url
`

const taskJoins = `
	LEFT JOIN users ON users.id = generation_tasks.user_id
	LEFT JOIN ai_models ON ai_models.id = generation_tasks.model_id
	LEFT JOIN api_providers ON api_providers.id = generation_tasks.provider_id
`

func (r *Repository) count(ctx context.Context, where string, args []any) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM generation_tasks
		`+taskJoins+`
		`+where+`
	`, args...).Scan(&total)
	return total, err
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

func buildTaskWhere(keyword string, status string, display string) (string, []any) {
	conditions := []string{}
	args := []any{}
	if status != "" && status != "all" {
		conditions = append(conditions, "generation_tasks.status = ?")
		args = append(args, status)
	}
	switch display {
	case "public":
		conditions = append(conditions, "generation_tasks.public_status = 'approved'")
	case "private":
		conditions = append(conditions, "generation_tasks.public_status = 'private'")
	case "pending":
		conditions = append(conditions, "generation_tasks.public_status = 'pending'")
	case "rejected":
		conditions = append(conditions, "generation_tasks.public_status = 'rejected'")
	}
	if strings.TrimSpace(keyword) != "" {
		conditions = append(conditions, "(generation_tasks.prompt LIKE ? OR users.email LIKE ? OR ai_models.model_name LIKE ? OR ai_models.display_name LIKE ?)")
		like := "%" + strings.TrimSpace(keyword) + "%"
		args = append(args, like, like, like, like)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func appendKeywordWhere(conditions *[]string, args *[]any, keyword string) {
	if strings.TrimSpace(keyword) == "" {
		return
	}
	*conditions = append(*conditions, "(generation_tasks.prompt LIKE ? OR generation_tasks.display_note LIKE ? OR users.email LIKE ? OR ai_models.model_name LIKE ? OR ai_models.display_name LIKE ?)")
	like := "%" + strings.TrimSpace(keyword) + "%"
	*args = append(*args, like, like, like, like, like)
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner) (*Task, error) {
	var task Task
	var referenceURL, size, outputFormat, errorMessage, resultJSON, displayNote sql.NullString
	var publicRequestedAt, publicReviewedAt sql.NullTime
	var userEmail, modelName, modelDisplayName, providerName, providerBaseURL sql.NullString
	var status string
	if err := row.Scan(
		&task.ID,
		&task.UserID,
		&task.ModelID,
		&task.ProviderID,
		&task.Capability,
		&task.Prompt,
		&referenceURL,
		&task.SizeTier,
		&size,
		&outputFormat,
		&task.TransparentBackground,
		&task.Quantity,
		&task.UserIP,
		&task.CostCredits,
		&task.ModelCostCredits,
		&task.RemainingCredits,
		&task.DurationSeconds,
		&status,
		&errorMessage,
		&resultJSON,
		&task.FavoriteEnabled,
		&task.PublicStatus,
		&publicRequestedAt,
		&publicReviewedAt,
		&task.DisplayEnabled,
		&displayNote,
		&task.CreatedAt,
		&task.UpdatedAt,
		&userEmail,
		&modelName,
		&modelDisplayName,
		&providerName,
		&providerBaseURL,
	); err != nil {
		return nil, err
	}
	task.Status = Status(status)
	if referenceURL.Valid {
		task.ReferenceImageURL = &referenceURL.String
	}
	if size.Valid {
		task.Size = &size.String
	}
	if outputFormat.Valid && strings.TrimSpace(outputFormat.String) != "" {
		task.OutputFormat = outputFormat.String
	} else if task.TransparentBackground {
		task.OutputFormat = "png"
	} else {
		task.OutputFormat = "jpeg"
	}
	if errorMessage.Valid {
		task.ErrorMessage = &errorMessage.String
	}
	if resultJSON.Valid {
		var payload any
		if err := json.Unmarshal([]byte(resultJSON.String), &payload); err == nil {
			task.ResultJSON = payload
		}
	}
	if displayNote.Valid {
		task.DisplayNote = &displayNote.String
	}
	if publicRequestedAt.Valid {
		value := appclock.DatabaseTime(publicRequestedAt.Time)
		task.PublicRequestedAt = &value
	}
	if publicReviewedAt.Valid {
		value := appclock.DatabaseTime(publicReviewedAt.Time)
		task.PublicReviewedAt = &value
	}
	if userEmail.Valid {
		task.UserEmail = &userEmail.String
	}
	if modelName.Valid {
		task.ModelName = &modelName.String
	}
	if modelDisplayName.Valid {
		task.ModelDisplayName = &modelDisplayName.String
	}
	if providerName.Valid {
		task.ProviderName = &providerName.String
	}
	if providerBaseURL.Valid {
		task.ProviderBaseURL = &providerBaseURL.String
	}
	task.CreatedAt = appclock.DatabaseTime(task.CreatedAt)
	task.UpdatedAt = appclock.DatabaseTime(task.UpdatedAt)
	return &task, nil
}

func scanTasks(rows *sql.Rows) ([]Task, error) {
	items := []Task{}
	for rows.Next() {
		item, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func scanAdminTaskListItem(row interface{ Scan(dest ...any) error }) (AdminTaskListItem, error) {
	var item AdminTaskListItem
	var userEmail, modelName, modelDisplayName, size, errorMessage, subscriptionPlanName sql.NullString
	var createdAt time.Time
	var status string
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&userEmail,
		&item.ModelID,
		&modelName,
		&modelDisplayName,
		&item.SizeTier,
		&size,
		&item.Quantity,
		&item.UserIP,
		&item.CostCredits,
		&item.DurationSeconds,
		&status,
		&errorMessage,
		&createdAt,
		&subscriptionPlanName,
	)
	if err != nil {
		return AdminTaskListItem{}, err
	}
	item.Status = Status(status)
	item.CreatedAt = appclock.DatabaseTime(createdAt).Format(time.RFC3339)
	if userEmail.Valid {
		item.UserEmail = &userEmail.String
	}
	if modelName.Valid {
		item.ModelName = &modelName.String
	}
	if modelDisplayName.Valid {
		item.ModelDisplayName = &modelDisplayName.String
	}
	if size.Valid {
		item.Size = &size.String
	}
	if errorMessage.Valid {
		item.ErrorMessage = &errorMessage.String
	}
	if subscriptionPlanName.Valid {
		item.UserSubscriptionPlanName = &subscriptionPlanName.String
	}
	return item, nil
}

func ResultURLs(value any) []string {
	seen := map[string]bool{}
	result := []string{}
	var walk func(any, int)
	walk = func(item any, depth int) {
		if item == nil || depth > 10 {
			return
		}
		if text, ok := item.(string); ok {
			if isDisplayURL(text) && !seen[text] {
				seen[text] = true
				result = append(result, text)
			}
			return
		}
		if list, ok := item.([]any); ok {
			for _, child := range list {
				walk(child, depth+1)
			}
			return
		}
		payload, ok := item.(map[string]any)
		if !ok {
			return
		}
		for _, key := range []string{"url", "image_url", "imageUrl", "output_url", "outputUrl", "file_url", "fileUrl"} {
			walk(payload[key], depth+1)
		}
		for _, key := range []string{"data", "result", "results", "output", "outputs", "images", "image", "final", "choices", "message", "content"} {
			walk(payload[key], depth+1)
		}
	}
	walk(value, 0)
	return result
}

func isDisplayURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func RewriteImageURL(providerBaseURL *string, value string) string {
	trimmed := strings.TrimSpace(value)
	if providerBaseURL == nil || strings.TrimSpace(*providerBaseURL) == "" || trimmed == "" || strings.HasPrefix(trimmed, "data:image/") {
		return value
	}
	providerURL, err := url.Parse(strings.TrimSpace(*providerBaseURL))
	if err != nil || providerURL.Scheme == "" || providerURL.Host == "" {
		return value
	}
	if strings.HasPrefix(trimmed, "/") {
		return providerURL.Scheme + "://" + providerURL.Host + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return value
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "127.0.0.1", "localhost", "::1", "0.0.0.0":
		parsed.Scheme = providerURL.Scheme
		parsed.Host = providerURL.Host
		return parsed.String()
	default:
		return value
	}
}
