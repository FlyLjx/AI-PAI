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
	"aipi-go/internal/resultdata"
)

type Repository struct {
	db *database.DB
}

const maxTaskErrorMessageBytes = 8 << 10

const taskHasResultImageSQL = `(generation_tasks.result_json IS NOT NULL OR EXISTS (
	SELECT 1 FROM generation_result_images
	WHERE generation_result_images.task_id = generation_tasks.id
))`

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
	// Re-reading the task through the wide user/model/provider join here would
	// keep the admission transaction open while unrelated rows are resolved.
	// The task fields supplied by the caller are authoritative at insert time;
	// let the normal FindByID path enrich a task after the transaction commits.
	if err := insertTask(ctx, tx, task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *Repository) create(ctx context.Context, store taskStore, task Task) (*Task, error) {
	if err := insertTask(ctx, store, task); err != nil {
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

func insertTask(ctx context.Context, store taskStore, task Task) error {
	resultJSON, err := durableResultJSON(task.ResultJSON)
	if err != nil {
		return err
	}
	_, err = store.ExecContext(ctx, `
		INSERT INTO generation_tasks
			(id, user_id, model_id, provider_id, capability, prompt, reference_image_url, size_tier, size, output_format, transparent_background, quantity, user_ip,
			 subscription_quota_units, cost_credits, model_cost_credits, remaining_credits, duration_seconds, status, error_message, result_json)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			 ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.UserID, task.ModelID, task.ProviderID, task.Capability, task.Prompt, task.ReferenceImageURL, task.SizeTier, task.Size, task.OutputFormat, task.TransparentBackground, task.Quantity, task.UserIP,
		task.SubscriptionQuotaUnits, task.CostCredits, task.ModelCostCredits, task.RemainingCredits, task.DurationSeconds, task.Status, task.ErrorMessage, resultJSON)
	return err
}

func (r *Repository) FindByID(ctx context.Context, id string) (*Task, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+taskSelectColumns+`
		FROM generation_tasks
		`+taskJoins+`
		WHERE generation_tasks.id = ?
		LIMIT 1
	`, id)
	task, err := scanTask(row)
	if err != nil {
		return nil, err
	}
	if err := r.attachStoredResultURLs(ctx, []*Task{task}); err != nil {
		return nil, err
	}
	return task, nil
}

func (r *Repository) attachStoredResultURLsToTasks(ctx context.Context, tasks []Task) error {
	pointers := make([]*Task, 0, len(tasks))
	for index := range tasks {
		pointers = append(pointers, &tasks[index])
	}
	return r.attachStoredResultURLs(ctx, pointers)
}

func (r *Repository) attachStoredResultURLs(ctx context.Context, tasks []*Task) error {
	byID := make(map[string]*Task, len(tasks))
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task == nil || task.Status != StatusSuccess || strings.TrimSpace(task.ID) == "" {
			continue
		}
		byID[task.ID] = task
		ids = append(ids, task.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for index, id := range ids {
		placeholders[index] = "?"
		args[index] = id
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT task_id, image_url
		FROM generation_result_images
		WHERE task_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY task_id, created_at, id
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID, imageURL string
		if err := rows.Scan(&taskID, &imageURL); err != nil {
			return err
		}
		if task := byID[taskID]; task != nil && strings.TrimSpace(imageURL) != "" {
			task.StoredResultURLs = append(task.StoredResultURLs, imageURL)
		}
	}
	return rows.Err()
}

func (r *Repository) FindAll(ctx context.Context, input ListInput) ([]Task, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildTaskWhere(input.Keyword, input.Status)
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
	if err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	return items, total, r.attachStoredResultURLsToTasks(ctx, items)
}

func (r *Repository) FindAdminList(ctx context.Context, input ListInput) ([]AdminTaskListItem, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildTaskWhere(input.Keyword, input.Status)
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

func (r *Repository) FindImages(ctx context.Context, input ListInput) ([]Task, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildTaskWhere(input.Keyword, "")
	if where == "" {
		where = "WHERE generation_tasks.status = 'success' AND " + taskHasResultImageSQL
	} else {
		where += " AND generation_tasks.status = 'success' AND " + taskHasResultImageSQL
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
	if err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	return items, total, r.attachStoredResultURLsToTasks(ctx, items)
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
		SELECT `+taskSelectColumnsWithoutResultJSON()+`
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
	items, err := scanTasks(rows)
	if err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status Status) (*Task, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE generation_tasks SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

// ReconcileGenerationBalanceReservation rebuilds the small per-user
// reservation cache from active generation tasks. It is intentionally
// idempotent, so it is safe to call after success, failure, cancellation, or
// a timeout sweep; a repeated status callback can never subtract twice.
func (r *Repository) ReconcileGenerationBalanceReservation(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lockedUserID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id FROM users WHERE id = ? FOR UPDATE
	`, userID).Scan(&lockedUserID); err != nil {
		return err
	}
	var reserved float64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cost_credits), 0)
		FROM generation_tasks
		WHERE user_id = ? AND status IN ('queued', 'pending', 'processing')
	`, userID).Scan(&reserved); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET generation_reserved_credits = ? WHERE id = ?
	`, reserved, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// ReleaseSubscriptionQuotaForTerminalTask returns a failed or canceled task's
// admission reservation exactly once by clearing its stored quota units.
func (r *Repository) ReleaseSubscriptionQuotaForTerminalTask(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID, status string
	var quantity, units int
	if err := tx.QueryRowContext(ctx, `SELECT user_id, status, quantity, subscription_quota_units FROM generation_tasks WHERE id=? FOR UPDATE`, id).Scan(&userID, &status, &quantity, &units); err != nil {
		return err
	}
	if (status != string(StatusFailed) && status != string(StatusCanceled)) || quantity < 1 || units < 1 {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_subscriptions SET quota_remaining=quota_remaining + ? WHERE user_id=? AND status='active'`, quantity*units, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE generation_tasks SET subscription_quota_units=0 WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
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
	resultJSON, err := durableResultJSON(result)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'success',
			result_json = ?,
			error_message = NULL,
			duration_seconds = ?
		WHERE id = ?
	`, resultJSON, durationSeconds, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) ReplaceResultJSON(ctx context.Context, id string, result any) error {
	encoded, err := durableResultJSON(result)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE generation_tasks SET result_json = ? WHERE id = ?`, encoded, id)
	return err
}

func (r *Repository) FinishFailed(ctx context.Context, id string, message string, durationSeconds float64) (*Task, error) {
	message = truncateUTF8(message, maxTaskErrorMessageBytes)
	referenceImageURL, err := r.terminalReferenceValue(ctx, id)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'failed',
			error_message = ?,
			reference_image_url = ?,
			duration_seconds = ?
		WHERE id = ? AND status = 'processing'
	`, message, referenceImageURL, durationSeconds, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) FinishFailedWithDetails(ctx context.Context, id string, message string, durationSeconds float64, details any) (*Task, error) {
	message = truncateUTF8(message, maxTaskErrorMessageBytes)
	referenceImageURL, err := r.terminalReferenceValue(ctx, id)
	if err != nil {
		return nil, err
	}
	resultJSON, err := errorDetailsResultJSON(details)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'failed',
			error_message = ?,
			result_json = ?,
			reference_image_url = ?,
			duration_seconds = ?
		WHERE id = ? AND status = 'processing'
	`, message, resultJSON, referenceImageURL, durationSeconds, id)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func truncateUTF8(value string, limit int) string {
	value = strings.ToValidUTF8(value, "\uFFFD")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	for limit > 0 && (value[limit]&0xc0) == 0x80 {
		limit--
	}
	return value[:limit]
}

const errorDetailsResultKey = "__aipi_error_details"

func errorDetailsResultJSON(details any) (string, error) {
	result := map[string]any{errorDetailsResultKey: details}
	encoded, err := durableResultJSON(result)
	if err != nil {
		return "", err
	}
	if encoded == nil {
		return "", nil
	}
	return encoded.(string), nil
}

// durableResultJSON is the repository persistence boundary for task results.
// Results may contain inline image bytes for the waiting request, but those
// bytes must never be stored in generation_tasks.result_json.
func durableResultJSON(value any) (any, error) {
	return resultdata.MarshalWithoutInlineImages(value)
}

func ErrorDetailsFromResult(result any) (any, bool) {
	payload, ok := result.(map[string]any)
	if !ok {
		return nil, false
	}
	details, ok := payload[errorDetailsResultKey]
	return details, ok && details != nil
}

func (r *Repository) terminalReferenceValue(ctx context.Context, id string) (any, error) {
	var raw sql.NullString
	if err := r.db.QueryRowContext(ctx, `
		SELECT reference_image_url FROM generation_tasks WHERE id = ?
	`, id).Scan(&raw); err != nil {
		return nil, err
	}
	if !raw.Valid {
		return nil, nil
	}
	cleaned, _ := resultdata.ReferenceURLsOnly(raw.String)
	if cleaned == nil {
		return nil, nil
	}
	return *cleaned, nil
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
		SELECT id, created_at, reference_image_url
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
		id             string
		createdAt      time.Time
		referenceImage sql.NullString
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.createdAt, &item.referenceImage); err != nil {
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
		var referenceImageURL any
		if item.referenceImage.Valid {
			if cleaned, _ := resultdata.ReferenceURLsOnly(item.referenceImage.String); cleaned != nil {
				referenceImageURL = *cleaned
			}
		}
		result, err := r.db.ExecContext(ctx, `
			UPDATE generation_tasks
			SET status = 'failed',
				error_message = ?,
				duration_seconds = ?,
				reference_image_url = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
				AND `+where+`
				AND updated_at <= ?
		`, message, durationSeconds, referenceImageURL, item.id, cutoff)
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
	referenceImageURL, err := r.terminalReferenceValue(ctx, id)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE generation_tasks
		SET status = 'canceled',
			error_message = '任务已取消',
			reference_image_url = ?
		WHERE id = ?
			AND status IN ('queued', 'processing', 'pending')
	`, referenceImageURL, id)
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

func (r *Repository) ImageURLByIndex(ctx context.Context, id string, index int) (string, error) {
	if index < 0 {
		return "", sql.ErrNoRows
	}
	// Successful tasks normally have their upstream image references in the
	// normalized table. Read that small row instead of loading/parsing the
	// potentially hundreds-of-KB result_json column for every image request.
	var status, imageURL string
	var providerBaseURL sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT generation_tasks.status,
			generation_result_images.image_url,
			api_providers.base_url
		FROM generation_result_images
		INNER JOIN generation_tasks ON generation_tasks.id = generation_result_images.task_id
		LEFT JOIN api_providers ON api_providers.id = generation_tasks.provider_id
		WHERE generation_result_images.task_id = ?
		ORDER BY generation_result_images.created_at ASC, generation_result_images.id ASC
		LIMIT 1 OFFSET ?
	`, id, index).Scan(&status, &imageURL, &providerBaseURL)
	if err == nil {
		if status != string(StatusSuccess) || strings.TrimSpace(imageURL) == "" {
			return "", sql.ErrNoRows
		}
		baseURL := ""
		if providerBaseURL.Valid {
			baseURL = providerBaseURL.String
		}
		return RewriteImageURL(&baseURL, imageURL), nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	// Keep the legacy JSON fallback for older tasks created before the image
	// cache table was populated or when caching an upstream URL failed.
	task, err := r.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	urls := ResultURLs(task.ResultJSON)
	if task.Status != StatusSuccess || index >= len(urls) {
		return "", sql.ErrNoRows
	}
	return RewriteImageURL(task.ProviderBaseURL, urls[index]), nil
}

type ListInput struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
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
	generation_tasks.created_at,
	generation_tasks.updated_at,
	users.email AS user_email,
	ai_models.model_name,
	ai_models.display_name AS model_display_name,
	api_providers.name AS provider_name,
	api_providers.base_url AS provider_base_url
`

func taskSelectColumnsWithoutResultJSON() string {
	return strings.Replace(taskSelectColumns, "generation_tasks.result_json,", "NULL AS result_json,", 1)
}

const taskJoins = `
	LEFT JOIN users ON users.id = generation_tasks.user_id
	LEFT JOIN ai_models ON ai_models.id = generation_tasks.model_id
	LEFT JOIN api_providers ON api_providers.id = generation_tasks.provider_id
`

func (r *Repository) count(ctx context.Context, where string, args []any) (int, error) {
	from := "FROM generation_tasks"
	// Status/display filters only touch the task table. Avoid joining the
	// relatively wide user/model/provider tables for the COUNT(*) query; joins
	// are still included when a keyword actually searches those tables.
	if strings.Contains(where, "users.") || strings.Contains(where, "ai_models.") || strings.Contains(where, "api_providers.") {
		from += taskJoins
	}
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		`+from+`
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

func buildTaskWhere(keyword string, status string) (string, []any) {
	conditions := []string{}
	args := []any{}
	if status != "" && status != "all" {
		conditions = append(conditions, "generation_tasks.status = ?")
		args = append(args, status)
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

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner) (*Task, error) {
	var task Task
	var referenceURL, size, outputFormat, errorMessage, resultJSON sql.NullString
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
