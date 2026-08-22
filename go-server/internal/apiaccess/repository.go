package apiaccess

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"aipi-go/internal/apierrors"
	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

const markUsedWriteInterval = 10 * time.Second

var markUsedWrites = struct {
	sync.Mutex
	last map[string]time.Time
}{last: make(map[string]time.Time)}

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// IdentitySearchTargets contains the indexed IDs that match an admin search
// for a user email, user ID, API key ID, or API key prefix. Resolving these
// small tables first avoids scanning the much larger api_access_logs table
// with a cross-table wildcard predicate.
type IdentitySearchTargets struct {
	Enabled   bool
	UserIDs   []string
	APIKeyIDs []string
}

func (r *Repository) ResolveIdentitySearchTargets(ctx context.Context, keyword string) (IdentitySearchTargets, error) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if !isIdentitySearchKeyword(keyword) {
		return IdentitySearchTargets{}, nil
	}

	like := "%" + keyword + "%"
	targets := IdentitySearchTargets{Enabled: true, UserIDs: []string{}, APIKeyIDs: []string{}}
	userRows, err := r.db.QueryContext(ctx, `
		SELECT id
		FROM users
		WHERE LOWER(id) LIKE ? OR LOWER(email) LIKE ?
		LIMIT 200
	`, like, like)
	if err != nil {
		return IdentitySearchTargets{}, err
	}
	for userRows.Next() {
		var id string
		if err := userRows.Scan(&id); err != nil {
			userRows.Close()
			return IdentitySearchTargets{}, err
		}
		targets.UserIDs = append(targets.UserIDs, id)
	}
	if err := userRows.Close(); err != nil {
		return IdentitySearchTargets{}, err
	}
	if err := userRows.Err(); err != nil {
		return IdentitySearchTargets{}, err
	}

	keyRows, err := r.db.QueryContext(ctx, `
		SELECT api_access_keys.id
		FROM api_access_keys
		LEFT JOIN users ON users.id = api_access_keys.user_id
		WHERE LOWER(api_access_keys.id) LIKE ?
			OR LOWER(api_access_keys.user_id) LIKE ?
			OR LOWER(api_access_keys.name) LIKE ?
			OR LOWER(api_access_keys.key_prefix) LIKE ?
			OR LOWER(COALESCE(users.email, '')) LIKE ?
		LIMIT 200
	`, like, like, like, like, like)
	if err != nil {
		return IdentitySearchTargets{}, err
	}
	for keyRows.Next() {
		var id string
		if err := keyRows.Scan(&id); err != nil {
			keyRows.Close()
			return IdentitySearchTargets{}, err
		}
		targets.APIKeyIDs = append(targets.APIKeyIDs, id)
	}
	if err := keyRows.Close(); err != nil {
		return IdentitySearchTargets{}, err
	}
	if err := keyRows.Err(); err != nil {
		return IdentitySearchTargets{}, err
	}
	return targets, nil
}

func isIdentitySearchKeyword(keyword string) bool {
	if strings.Contains(keyword, "@") || strings.HasPrefix(keyword, "sk-") {
		return true
	}
	if len(keyword) != 36 {
		return false
	}
	for index, value := range keyword {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if value != '-' {
				return false
			}
			continue
		}
		if !((value >= '0' && value <= '9') || (value >= 'a' && value <= 'f')) {
			return false
		}
	}
	return true
}

type accessStore interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *database.Row
}

func (r *Repository) CreateKey(ctx context.Context, key AccessKey) (*AccessKey, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO api_access_keys (id, user_id, name, key_prefix, key_hash, key_plain, status, concurrency_limit, billing_mode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, key.ID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash, key.KeyPlain, key.Status, normalizedConcurrencyLimit(key.ConcurrencyLimit), key.BillingMode)
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
			api_access_keys.billing_mode,
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
			api_access_keys.concurrency_limit, api_access_keys.billing_mode, api_access_keys.last_used_at, api_access_keys.deleted_at,
			api_access_keys.created_at, api_access_keys.updated_at
		LIMIT 1
	`, id)
	return scanAccessKey(row)
}

func (r *Repository) FindKeyPlainForUser(ctx context.Context, id string, userID string) (*string, error) {
	var keyPlain sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT key_plain
		FROM api_access_keys
		WHERE id = ?
			AND user_id = ?
			AND deleted_at IS NULL
		LIMIT 1
	`, strings.TrimSpace(id), strings.TrimSpace(userID)).Scan(&keyPlain)
	if err != nil {
		return nil, err
	}
	if !keyPlain.Valid || strings.TrimSpace(keyPlain.String) == "" {
		return nil, nil
	}
	return &keyPlain.String, nil
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
			api_access_keys.concurrency_limit, api_access_keys.billing_mode, api_access_keys.last_used_at, api_access_keys.deleted_at,
			api_access_keys.created_at, api_access_keys.updated_at
		ORDER BY api_access_keys.created_at DESC, api_access_keys.id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccessKeys(rows)
}

// ListKeysPage bounds the user-facing key list at the database layer.
func (r *Repository) ListKeysPage(ctx context.Context, userID string, page int, pageSize int) ([]AccessKey, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	userID = strings.TrimSpace(userID)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_access_keys WHERE user_id = ? AND deleted_at IS NULL`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, keyListSelect()+`
		WHERE api_access_keys.user_id = ? AND api_access_keys.deleted_at IS NULL
		GROUP BY api_access_keys.id, api_access_keys.user_id, users.email, api_access_keys.name,
			api_access_keys.key_prefix, api_access_keys.key_hash, api_access_keys.key_plain, api_access_keys.status,
			api_access_keys.concurrency_limit, api_access_keys.billing_mode, api_access_keys.last_used_at, api_access_keys.deleted_at,
			api_access_keys.created_at, api_access_keys.updated_at
		ORDER BY api_access_keys.created_at DESC, api_access_keys.id DESC
		LIMIT ? OFFSET ?
	`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanAccessKeys(rows)
	return items, total, err
}

// ListAdminKeys filters and pages API Keys before loading their usage totals.
// This keeps the management screen bounded even when api_access_logs is large.
func (r *Repository) ListAdminKeys(ctx context.Context, input ListKeysInput) ([]AccessKey, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildAdminKeyWhere(input)
	orderBy, usageSort := adminKeyOrder(input.SortBy, input.SortOrder)
	var total int
	countFrom := "FROM api_access_keys"
	if !input.IdentityOnly && strings.TrimSpace(input.Keyword) != "" {
		countFrom += " LEFT JOIN users ON users.id = api_access_keys.user_id"
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		`+countFrom+`
		`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	queryArgs := append(append([]any{}, args...), pageSize, offset)
	selectUsage := `
			0 AS request_count,
			0 AS success_count,
			0 AS failed_count,
			0 AS image_count,
			NULL AS last_error`
	usageJoin := ""
	if usageSort {
		selectUsage = `
			COALESCE(key_usage.request_count, 0) AS request_count,
			COALESCE(key_usage.success_count, 0) AS success_count,
			COALESCE(key_usage.failed_count, 0) AS failed_count,
			COALESCE(key_usage.image_count, 0) AS image_count,
			key_usage.last_error AS last_error`
		usageJoin = `
		LEFT JOIN (
			SELECT
				api_key_id,
				COUNT(*) AS request_count,
				COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) AS success_count,
				COALESCE(SUM(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed_count,
				COALESCE(SUM(CASE WHEN status = 'success' THEN image_count ELSE 0 END), 0) AS image_count,
				MAX(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN error_message ELSE NULL END) AS last_error
			FROM api_access_logs
			GROUP BY api_key_id
		) AS key_usage ON key_usage.api_key_id = api_access_keys.id`
	}
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
			api_access_keys.billing_mode,
			api_access_keys.last_used_at,
			api_access_keys.deleted_at,
			api_access_keys.created_at,
			api_access_keys.updated_at,
		`+selectUsage+`
		FROM api_access_keys
		LEFT JOIN users ON users.id = api_access_keys.user_id
		`+usageJoin+`
		`+where+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	items, err := scanAccessKeys(rows)
	if err != nil {
		rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if !usageSort {
		if err := r.attachKeyUsageStats(ctx, items); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func adminKeyOrder(sortBy string, sortOrder string) (string, bool) {
	direction := sortDirectionSQL(sortOrder)
	sortBy = strings.TrimSpace(sortBy)
	metric := false
	orderExpression := ""
	switch sortBy {
	case "createdAt":
		orderExpression = "api_access_keys.created_at"
	case "name":
		orderExpression = "LOWER(api_access_keys.name)"
	case "user":
		orderExpression = "LOWER(COALESCE(users.email, api_access_keys.user_id))"
	case "status":
		orderExpression = "api_access_keys.status"
	case "billingMode":
		orderExpression = "COALESCE(api_access_keys.billing_mode, 'auto')"
	case "concurrencyLimit":
		orderExpression = "api_access_keys.concurrency_limit"
	case "requestCount":
		orderExpression = "COALESCE(key_usage.request_count, 0)"
		metric = true
	case "successCount":
		orderExpression = "COALESCE(key_usage.success_count, 0)"
		metric = true
	case "failedCount":
		orderExpression = "COALESCE(key_usage.failed_count, 0)"
		metric = true
	case "imageCount":
		orderExpression = "COALESCE(key_usage.image_count, 0)"
		metric = true
	case "lastUsedAt":
		orderExpression = "api_access_keys.last_used_at"
	default:
		return "api_access_keys.created_at DESC, api_access_keys.id DESC", false
	}
	return orderExpression + " " + direction + ", api_access_keys.created_at DESC, api_access_keys.id DESC", metric
}

func sortDirectionSQL(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "asc") {
		return "ASC"
	}
	return "DESC"
}

func buildAdminKeyWhere(input ListKeysInput) (string, []any) {
	conditions := []string{"api_access_keys.deleted_at IS NULL"}
	args := []any{}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status == "active" || status == "disabled" {
		conditions = append(conditions, "api_access_keys.status = ?")
		args = append(args, status)
	}
	if input.IdentityOnly {
		identityConditions := []string{}
		appendIDFilter(&identityConditions, &args, "api_access_keys.user_id", input.UserIDs)
		appendIDFilter(&identityConditions, &args, "api_access_keys.id", input.APIKeyIDs)
		if len(identityConditions) == 0 {
			conditions = append(conditions, "1 = 0")
		} else {
			conditions = append(conditions, "("+strings.Join(identityConditions, " OR ")+")")
		}
	} else if keyword := strings.ToLower(strings.TrimSpace(input.Keyword)); keyword != "" {
		like := "%" + keyword + "%"
		conditions = append(conditions, `(
			LOWER(api_access_keys.id) LIKE ?
			OR LOWER(api_access_keys.user_id) LIKE ?
			OR LOWER(api_access_keys.name) LIKE ?
			OR LOWER(api_access_keys.key_prefix) LIKE ?
			OR LOWER(COALESCE(users.email, '')) LIKE ?
		)`)
		for range 5 {
			args = append(args, like)
		}
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func appendIDFilter(conditions *[]string, args *[]any, column string, ids []string) {
	uniqueIDs := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return
	}
	placeholders := make([]string, len(uniqueIDs))
	for index, id := range uniqueIDs {
		placeholders[index] = "?"
		*args = append(*args, id)
	}
	*conditions = append(*conditions, column+" IN ("+strings.Join(placeholders, ",")+")")
}

func (r *Repository) attachKeyUsageStats(ctx context.Context, keys []AccessKey) error {
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		ids = append(ids, key.ID)
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
		SELECT
			api_key_id,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed_count,
			COALESCE(SUM(CASE WHEN status = 'success' THEN image_count ELSE 0 END), 0) AS image_count,
			MAX(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN error_message ELSE NULL END) AS last_error
		FROM api_access_logs
		WHERE api_key_id IN (`+strings.Join(placeholders, ",")+`)
		GROUP BY api_key_id
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	type usage struct {
		requestCount int
		successCount int
		failedCount  int
		imageCount   int
		lastError    sql.NullString
	}
	usageByKeyID := map[string]usage{}
	for rows.Next() {
		var id string
		var item usage
		if err := rows.Scan(&id, &item.requestCount, &item.successCount, &item.failedCount, &item.imageCount, &item.lastError); err != nil {
			return err
		}
		usageByKeyID[id] = item
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for index := range keys {
		item, found := usageByKeyID[keys[index].ID]
		if !found {
			continue
		}
		keys[index].RequestCount = item.requestCount
		keys[index].SuccessCount = item.successCount
		keys[index].FailedCount = item.failedCount
		keys[index].ImageCount = item.imageCount
		if item.lastError.Valid && strings.TrimSpace(item.lastError.String) != "" {
			keys[index].LastError = &item.lastError.String
		}
	}
	return nil
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
			api_access_keys.billing_mode,
			api_access_keys.last_used_at,
			api_access_keys.deleted_at,
			api_access_keys.created_at,
			api_access_keys.updated_at,
			COUNT(api_access_logs.id) AS request_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status = 'success' THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status = 'success' THEN api_access_logs.image_count ELSE 0 END), 0) AS image_count,
			MAX(CASE WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN api_access_logs.error_message ELSE NULL END) AS last_error
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
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	now := time.Now()
	markUsedWrites.Lock()
	if last := markUsedWrites.last[id]; !last.IsZero() && now.Sub(last) < markUsedWriteInterval {
		markUsedWrites.Unlock()
		return nil
	}
	markUsedWrites.last[id] = now
	markUsedWrites.Unlock()

	_, err := r.db.ExecContext(ctx, `UPDATE api_access_keys SET last_used_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	if err != nil {
		markUsedWrites.Lock()
		delete(markUsedWrites.last, id)
		markUsedWrites.Unlock()
	}
	return err
}

func (r *Repository) RequestCountSince(ctx context.Context, apiKeyID string, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM api_access_logs
		WHERE api_key_id = ? AND created_at >= ?
	`, strings.TrimSpace(apiKeyID), since).Scan(&count)
	return count, err
}

func (r *Repository) RequestCountsSince(ctx context.Context, apiKeyIDs []string, since time.Time) (map[string]int, error) {
	ids := make([]string, 0, len(apiKeyIDs))
	seen := map[string]bool{}
	for _, id := range apiKeyIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	result := map[string]int{}
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, since)
	for index, id := range ids {
		placeholders[index] = "?"
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT api_key_id, COUNT(*)
		FROM api_access_logs
		WHERE created_at >= ? AND api_key_id IN (`+strings.Join(placeholders, ",")+`)
		GROUP BY api_key_id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		result[id] = count
	}
	return result, rows.Err()
}

func (r *Repository) CreateLog(ctx context.Context, log UsageLog) (*UsageLog, error) {
	return r.createLog(ctx, r.db, log)
}

func (r *Repository) CreateLogWithTx(ctx context.Context, tx *database.Tx, log UsageLog) (*UsageLog, error) {
	if tx == nil {
		return r.CreateLog(ctx, log)
	}
	// Task admission already holds the account transaction lock. Avoid the
	// follow-up wide join used by the non-transactional read path so that log
	// enrichment cannot extend the lock's critical section.
	if err := insertUsageLog(ctx, tx, log); err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *Repository) createLog(ctx context.Context, store accessStore, log UsageLog) (*UsageLog, error) {
	if err := insertUsageLog(ctx, store, log); err != nil {
		return nil, err
	}
	row := store.QueryRowContext(ctx, usageLogSelect()+` WHERE api_access_logs.id = ? LIMIT 1`, log.ID)
	return scanUsageLog(row)
}

func insertUsageLog(ctx context.Context, store accessStore, log UsageLog) error {
	requestParams, err := encodeRequestParams(log.RequestParams)
	if err != nil {
		return err
	}
	responseStatusCode, errorMessage, errorCode, errorDetails := usageLogErrorFields(log)
	_, err = store.ExecContext(ctx, `
		INSERT INTO api_access_logs
			(id, user_id, api_key_id, access_ip, access_host, task_id, endpoint, model, prompt, size, quality, quantity, image_count, response_format, request_params, status, error_message, response_status_code, error_code, error_details, finished_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.ID, log.UserID, log.APIKeyID, log.AccessIP, log.AccessHost, log.TaskID, log.Endpoint, log.Model, log.Prompt, log.Size, log.Quality, log.Quantity, log.ImageCount, log.ResponseFormat, requestParams, log.Status, errorMessage, responseStatusCode, errorCode, errorDetails, log.FinishedAt)
	return err
}

func encodeRequestParams(value map[string]any) (any, error) {
	if len(value) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(body), nil
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
	return r.FinishLogWithDetails(ctx, id, status, imageCount, message, apierrors.Details{})
}

func (r *Repository) FinishLogWithDetails(ctx context.Context, id string, status string, imageCount int, message string, details apierrors.Details) error {
	responseStatusCode, errorMessage, errorCode, errorDetails := usageLogErrorFields(UsageLog{
		Status:             status,
		ErrorMessage:       stringPointer(message),
		ResponseStatusCode: details.StatusCode,
		ErrorCode:          stringPointer(details.Code),
		ErrorDetails:       detailsPointer(details),
	})
	_, err := r.db.ExecContext(ctx, `
		UPDATE api_access_logs
		SET status = ?, image_count = ?, error_message = ?,
			charged_credits = CASE WHEN ? IN ('success', 'succeeded') THEN COALESCE((
				SELECT generation_tasks.cost_credits FROM generation_tasks
				WHERE generation_tasks.id = api_access_logs.task_id LIMIT 1
			), 0) ELSE 0 END,
			model_cost_credits = CASE WHEN ? IN ('success', 'succeeded') THEN COALESCE((
				SELECT generation_tasks.model_cost_credits FROM generation_tasks
				WHERE generation_tasks.id = api_access_logs.task_id LIMIT 1
			), 0) ELSE 0 END,
			response_status_code = ?, error_code = ?, error_details = ?,
			finished_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, imageCount, errorMessage, status, status, responseStatusCode, errorCode, errorDetails, id)
	return err
}

func (r *Repository) FinishLogsForTask(ctx context.Context, taskID string, status string, imageCount int, message string) error {
	return r.FinishLogsForTaskWithDetails(ctx, taskID, status, imageCount, message, apierrors.Details{})
}

func (r *Repository) FinishLogsForTaskWithDetails(ctx context.Context, taskID string, status string, imageCount int, message string, details apierrors.Details) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	responseStatusCode, errorMessage, errorCode, errorDetails := usageLogErrorFields(UsageLog{
		Status:             status,
		ErrorMessage:       stringPointer(message),
		ResponseStatusCode: details.StatusCode,
		ErrorCode:          stringPointer(details.Code),
		ErrorDetails:       detailsPointer(details),
	})
	_, err := r.db.ExecContext(ctx, `
		UPDATE api_access_logs
		SET status = ?, image_count = ?, error_message = ?,
			charged_credits = CASE WHEN ? IN ('success', 'succeeded') THEN COALESCE((
				SELECT generation_tasks.cost_credits FROM generation_tasks
				WHERE generation_tasks.id = api_access_logs.task_id LIMIT 1
			), 0) ELSE 0 END,
			model_cost_credits = CASE WHEN ? IN ('success', 'succeeded') THEN COALESCE((
				SELECT generation_tasks.model_cost_credits FROM generation_tasks
				WHERE generation_tasks.id = api_access_logs.task_id LIMIT 1
			), 0) ELSE 0 END,
			response_status_code = ?, error_code = ?, error_details = ?,
			finished_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
			AND (
				status IN ('queued', 'processing')
				OR (status = 'failed' AND error_message = 'context canceled')
			)
	`, status, imageCount, errorMessage, status, status, responseStatusCode, errorCode, errorDetails, taskID)
	return err
}

func usageLogErrorFields(log UsageLog) (int, any, any, any) {
	status := strings.ToLower(strings.TrimSpace(log.Status))
	responseStatusCode := log.ResponseStatusCode
	switch {
	case status == "success" || status == "succeeded":
		responseStatusCode = http.StatusOK
	case status == "canceled" || status == "cancelled":
		if responseStatusCode == 0 {
			responseStatusCode = 499
		}
	case status == "failed":
		if responseStatusCode == 0 {
			responseStatusCode = http.StatusInternalServerError
		}
	}
	if status != "failed" && status != "canceled" && status != "cancelled" {
		return responseStatusCode, nil, nil, nil
	}
	details := apierrors.Details{StatusCode: responseStatusCode}
	if log.ErrorDetails != nil {
		details = *log.ErrorDetails
	}
	details.StatusCode = responseStatusCode
	if strings.TrimSpace(details.Message) == "" && log.ErrorMessage != nil {
		details.Message = strings.TrimSpace(*log.ErrorMessage)
	}
	if log.ErrorCode != nil && strings.TrimSpace(*log.ErrorCode) != "" {
		details.Code = strings.TrimSpace(*log.ErrorCode)
	}
	apierrors.Normalize(&details)
	encoded, err := json.Marshal(details)
	if err != nil {
		encoded = nil
	}
	return responseStatusCode, details.Message, details.Code, encoded
}

func stringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func detailsPointer(value apierrors.Details) *apierrors.Details {
	if value.StatusCode == 0 && strings.TrimSpace(value.Message) == "" && strings.TrimSpace(value.Code) == "" {
		return nil
	}
	return &value
}

func (r *Repository) SyncTerminalTaskLogs(ctx context.Context, limit int) error {
	_, err := r.syncTerminalTaskLogBatch(ctx, limit)
	return err
}

func (r *Repository) syncTerminalTaskLogBatch(ctx context.Context, limit int) (int, error) {
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
		return 0, err
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
			return 0, err
		}
		updates = append(updates, item)
	}
	if err := rows.Err(); err != nil {
		return 0, err
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
		} else if item.status == "canceled" {
			status = "canceled"
			if message == "" {
				message = "任务已取消"
			}
		} else if message == "" {
			message = "图片生成失败"
		}
		if err := r.FinishLog(ctx, item.id, status, imageCount, message); err != nil {
			return 0, err
		}
	}
	return len(updates), nil
}

func (r *Repository) FindLogByID(ctx context.Context, id string) (*UsageLog, error) {
	row := r.db.QueryRowContext(ctx, usageLogSelect()+` WHERE api_access_logs.id = ? LIMIT 1`, id)
	return scanUsageLog(row)
}

func (r *Repository) ListLogs(ctx context.Context, input ListLogsInput) ([]UsageLog, int, error) {
	items, stats, err := r.ListLogsWithStats(ctx, input)
	return items, stats.Total, err
}

// StreamLogExportRows reads the filtered log projection in creation order.
// The callback lets callers write a workbook without holding the full export
// in memory.
func (r *Repository) StreamLogExportRows(ctx context.Context, input ListLogsInput, fn func(UsageExportRow) error) error {
	where, args := buildLogWhere(input)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			api_access_logs.created_at,
			api_access_logs.endpoint,
			api_access_logs.task_id,
			api_access_logs.model,
			api_access_logs.size,
			api_access_logs.quantity,
			COALESCE(api_access_logs.charged_credits, 0),
			api_access_logs.status,
			api_access_logs.error_message
		FROM api_access_logs
		LEFT JOIN users ON users.id = api_access_logs.user_id
		LEFT JOIN api_access_keys ON api_access_keys.id = api_access_logs.api_key_id
		`+where+`
		ORDER BY api_access_logs.created_at DESC, api_access_logs.id DESC
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item UsageExportRow
		var taskID, errorMessage sql.NullString
		if err := rows.Scan(
			&item.CreatedAt,
			&item.Endpoint,
			&taskID,
			&item.Model,
			&item.Size,
			&item.Quantity,
			&item.ChargedCredits,
			&item.Status,
			&errorMessage,
		); err != nil {
			return err
		}
		if taskID.Valid {
			item.TaskID = taskID.String
		}
		if errorMessage.Valid {
			item.ErrorMessage = errorMessage.String
		}
		item.CreatedAt = appclock.DatabaseTime(item.CreatedAt)
		if err := fn(item); err != nil {
			return err
		}
	}
	return rows.Err()
}

// ListLogsWithStats loads the current page before calculating aggregate values
// for the same filter set. This keeps a slow broad search from blocking the
// first visible page indefinitely.
func (r *Repository) ListLogsWithStats(ctx context.Context, input ListLogsInput) ([]UsageLog, UsageStats, error) {
	page, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where, args := buildLogWhere(input)
	orderBy := adminLogOrder(input.SortBy, input.SortOrder)
	queryArgs := append(append([]any{}, args...), pageSize+1, offset)
	rows, err := r.db.QueryContext(ctx, usageLogSelect()+` `+where+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, UsageStats{}, err
	}
	defer rows.Close()
	items := make([]UsageLog, 0, pageSize)
	hasMore := false
	for rows.Next() {
		item, err := scanUsageLog(rows)
		if err != nil {
			return nil, UsageStats{}, err
		}
		if len(items) >= pageSize {
			hasMore = true
			continue
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, UsageStats{}, err
	}
	if err := rows.Close(); err != nil {
		return nil, UsageStats{}, err
	}

	countFrom := logCountFrom(input)
	var stats UsageStats
	statsCtx := ctx
	var cancel context.CancelFunc
	if strings.TrimSpace(input.Keyword) != "" {
		// A broad log search can require a sequential scan over a large table.
		// The page above is already available, so do not make the visible result
		// wait for an exact aggregate indefinitely.
		statsCtx, cancel = context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
	}
	if err := r.db.QueryRowContext(statsCtx, `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('success', 'succeeded') THEN 1 ELSE 0 END), 0) AS success,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed,
			COALESCE(SUM(CASE
				WHEN api_access_logs.status IN ('success', 'succeeded') THEN 1
				WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN 1
				ELSE 0
			END), 0) AS counted,
			COALESCE(SUM(api_access_logs.image_count), 0) AS image_count,
			COALESCE(SUM(api_access_logs.charged_credits), 0) AS charged_credits,
			COALESCE(SUM(api_access_logs.model_cost_credits), 0) AS model_cost_credits
		`+countFrom+`
		`+where, args...).Scan(
		&stats.Total,
		&stats.Success,
		&stats.Failed,
		&stats.Counted,
		&stats.ImageCount,
		&stats.ChargedCredits,
		&stats.ModelCostCredits,
	); err != nil {
		if ctx.Err() != nil || statsCtx.Err() != context.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
			return nil, UsageStats{}, err
		}
		stats = usageStatsFromPage(items, (page-1)*pageSize)
		stats.HasMore = hasMore
		return items, stats, nil
	}
	stats.TotalExact = true
	stats.HasMore = stats.Total > page*pageSize
	return items, stats, nil
}

func usageStatsFromPage(items []UsageLog, offset int) UsageStats {
	stats := UsageStats{Total: offset + len(items)}
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "success" || status == "succeeded" {
			stats.Success++
			stats.Counted++
		} else if status == "failed" || status == "canceled" || status == "cancelled" {
			stats.Failed++
			stats.Counted++
		}
		stats.ImageCount += item.ImageCount
		stats.ChargedCredits += item.ChargedCredits
		stats.ModelCostCredits += item.ModelCostCredits
	}
	return stats
}

func adminLogOrder(sortBy string, sortOrder string) string {
	direction := sortDirectionSQL(sortOrder)
	sortBy = strings.TrimSpace(sortBy)
	orderExpression := ""
	switch sortBy {
	case "createdAt":
		orderExpression = "api_access_logs.created_at"
	case "user":
		orderExpression = "LOWER(COALESCE(users.email, api_access_logs.user_id))"
	case "endpoint":
		orderExpression = "LOWER(api_access_logs.endpoint)"
	case "model":
		orderExpression = "LOWER(api_access_logs.model)"
	case "imageCount":
		orderExpression = "api_access_logs.image_count"
	case "quantity":
		orderExpression = "api_access_logs.quantity"
	case "chargedCredits":
		orderExpression = "api_access_logs.charged_credits"
	case "modelCostCredits":
		orderExpression = "api_access_logs.model_cost_credits"
	case "durationSeconds":
		orderExpression = "COALESCE(generation_tasks.duration_seconds, 0)"
	case "status":
		orderExpression = "api_access_logs.status"
	default:
		return "api_access_logs.created_at DESC, api_access_logs.id DESC"
	}
	return orderExpression + " " + direction + ", api_access_logs.created_at DESC, api_access_logs.id DESC"
}

func (r *Repository) LogStats(ctx context.Context, input ListLogsInput) (UsageStats, error) {
	where, args := buildLogWhere(input)
	countFrom := logCountFrom(input)
	var stats UsageStats
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('success', 'succeeded') THEN 1 ELSE 0 END), 0) AS success,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed,
			COALESCE(SUM(CASE
				WHEN api_access_logs.status IN ('success', 'succeeded') THEN 1
				WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN 1
				ELSE 0
			END), 0) AS counted,
			COALESCE(SUM(api_access_logs.image_count), 0) AS image_count,
			COALESCE(SUM(api_access_logs.charged_credits), 0) AS charged_credits,
			COALESCE(SUM(api_access_logs.model_cost_credits), 0) AS model_cost_credits
		`+countFrom+`
		`+where, args...).Scan(
		&stats.Total,
		&stats.Success,
		&stats.Failed,
		&stats.Counted,
		&stats.ImageCount,
		&stats.ChargedCredits,
		&stats.ModelCostCredits,
	)
	return stats, err
}

// logCountFrom keeps the COUNT/aggregate queries index-friendly for the
// common case. User/key joins are only needed when a keyword searches fields
// owned by those tables; filtering by IDs and status can be done directly on
// api_access_logs.
func logCountFrom(input ListLogsInput) string {
	if strings.TrimSpace(input.Keyword) == "" || input.IdentityOnly {
		return "FROM api_access_logs"
	}
	return `FROM api_access_logs
		LEFT JOIN users ON users.id = api_access_logs.user_id
		LEFT JOIN api_access_keys ON api_access_keys.id = api_access_logs.api_key_id`
}

func (r *Repository) DailyUsageTrend(ctx context.Context, userID string, startAt time.Time, endAt time.Time) ([]UsageTrendPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			DATE(created_at) AS usage_date,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status IN ('success', 'succeeded') THEN 1 ELSE 0 END), 0) AS success,
			COALESCE(SUM(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed
		FROM api_access_logs
		WHERE user_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at)
	`, strings.TrimSpace(userID), startAt, endAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []UsageTrendPoint{}
	for rows.Next() {
		var day time.Time
		var item UsageTrendPoint
		if err := rows.Scan(&day, &item.Total, &item.Success, &item.Failed); err != nil {
			return nil, err
		}
		item.Date = appclock.DatabaseTime(day).Format("2006-01-02")
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UsageAnalytics(ctx context.Context, userID string, startAt time.Time, endAt time.Time) (UsageAnalytics, error) {
	result := UsageAnalytics{
		Models: []UsageModelStat{},
		Hourly: make([]UsageHourlyPoint, 24),
	}
	for hour := range result.Hourly {
		result.Hourly[hour].Hour = hour
	}

	modelRows, err := r.db.QueryContext(ctx, `
		SELECT
			COALESCE(model, ''),
			COALESCE(size, ''),
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status IN ('success', 'succeeded') THEN 1 ELSE 0 END), 0) AS success,
			COALESCE(SUM(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed
		FROM api_access_logs
		WHERE user_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY model, size
		ORDER BY total DESC, model ASC, size ASC
	`, strings.TrimSpace(userID), startAt, endAt)
	if err != nil {
		return result, err
	}
	for modelRows.Next() {
		var item UsageModelStat
		if err := modelRows.Scan(&item.Model, &item.Size, &item.Total, &item.Success, &item.Failed); err != nil {
			modelRows.Close()
			return result, err
		}
		counted := item.Success + item.Failed
		if counted > 0 {
			item.SuccessRate = float64(item.Success) / float64(counted) * 100
		}
		result.Models = append(result.Models, item)
	}
	if err := modelRows.Close(); err != nil {
		return result, err
	}
	if err := modelRows.Err(); err != nil {
		return result, err
	}

	hourExpression := "HOUR(created_at)"
	if database.CurrentDialect() == database.DialectPostgres {
		hourExpression = "EXTRACT(HOUR FROM created_at)"
	}
	hourRows, err := r.db.QueryContext(ctx, `
		SELECT `+hourExpression+` AS usage_hour,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status IN ('success', 'succeeded') THEN 1 ELSE 0 END), 0) AS success,
			COALESCE(SUM(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed
		FROM api_access_logs
		WHERE user_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY `+hourExpression+`
		ORDER BY `+hourExpression+` ASC
	`, strings.TrimSpace(userID), startAt, endAt)
	if err != nil {
		return result, err
	}
	for hourRows.Next() {
		var hour int
		var point UsageHourlyPoint
		if err := hourRows.Scan(&hour, &point.Total, &point.Success, &point.Failed); err != nil {
			hourRows.Close()
			return result, err
		}
		if hour >= 0 && hour < len(result.Hourly) {
			point.Hour = hour
			result.Hourly[hour] = point
		}
	}
	if err := hourRows.Close(); err != nil {
		return result, err
	}
	if err := hourRows.Err(); err != nil {
		return result, err
	}
	return result, nil
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
			api_access_logs.access_ip,
			api_access_logs.access_host,
			api_access_logs.task_id,
			api_access_logs.endpoint,
			api_access_logs.model,
			api_access_logs.prompt,
			api_access_logs.size,
			api_access_logs.quality,
			api_access_logs.quantity,
			api_access_logs.image_count,
			api_access_logs.response_format,
			api_access_logs.request_params,
			api_access_logs.status,
			api_access_logs.error_message,
			COALESCE(api_access_logs.response_status_code, 0),
			api_access_logs.error_code,
			api_access_logs.error_details,
			COALESCE(api_access_logs.charged_credits, 0),
			COALESCE(api_access_logs.model_cost_credits, 0),
			COALESCE(generation_tasks.duration_seconds, 0),
			generation_tasks.result_json,
			` + usageLogTaskUsageSelect() + `,
			api_access_logs.created_at,
			api_access_logs.finished_at
		FROM api_access_logs
		LEFT JOIN users ON users.id = api_access_logs.user_id
		LEFT JOIN api_access_keys ON api_access_keys.id = api_access_logs.api_key_id
		LEFT JOIN generation_tasks ON generation_tasks.id = api_access_logs.task_id
	`
}

func usageLogTaskUsageSelect() string {
	if database.CurrentDialect() == database.DialectPostgres {
		return "generation_tasks.result_json -> 'usage'"
	}
	return "JSON_EXTRACT(generation_tasks.result_json, '$.usage')"
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
	if input.StartAt != nil && !input.StartAt.IsZero() {
		conditions = append(conditions, "api_access_logs.created_at >= ?")
		args = append(args, *input.StartAt)
	}
	if input.EndAt != nil && !input.EndAt.IsZero() {
		conditions = append(conditions, "api_access_logs.created_at < ?")
		args = append(args, *input.EndAt)
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	switch status {
	case "", "all":
	case "success", "succeeded":
		conditions = append(conditions, "api_access_logs.status IN ('success', 'succeeded')")
	case "canceled", "cancelled":
		conditions = append(conditions, "api_access_logs.status IN ('canceled', 'cancelled')")
	default:
		conditions = append(conditions, "api_access_logs.status = ?")
		args = append(args, status)
	}
	if input.IdentityOnly {
		identityConditions := []string{}
		appendIDFilter(&identityConditions, &args, "api_access_logs.user_id", input.UserIDs)
		appendIDFilter(&identityConditions, &args, "api_access_logs.api_key_id", input.APIKeyIDs)
		if len(identityConditions) == 0 {
			conditions = append(conditions, "1 = 0")
		} else {
			conditions = append(conditions, "("+strings.Join(identityConditions, " OR ")+")")
		}
	} else if keyword := strings.ToLower(strings.TrimSpace(input.Keyword)); keyword != "" {
		like := "%" + keyword + "%"
		requestParamsCondition := "LOWER(COALESCE(CAST(api_access_logs.request_params AS CHAR), '')) LIKE ?"
		if database.CurrentDialect() == database.DialectPostgres {
			requestParamsCondition = "LOWER(COALESCE(CAST(api_access_logs.request_params AS TEXT), '')) LIKE ?"
		}
		conditions = append(conditions, `(
			LOWER(COALESCE(api_access_logs.id, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.user_id, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.api_key_id, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.task_id, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.endpoint, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.model, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.prompt, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.size, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.quality, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.response_format, '')) LIKE ?
			OR LOWER(COALESCE(api_access_logs.error_message, '')) LIKE ?
			OR `+requestParamsCondition+`
			OR LOWER(COALESCE(users.email, '')) LIKE ?
			OR LOWER(COALESCE(api_access_keys.name, '')) LIKE ?
			OR LOWER(COALESCE(api_access_keys.key_prefix, '')) LIKE ?
		)`)
		for range 15 {
			args = append(args, like)
		}
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *Repository) AdminOperations(ctx context.Context, startAt time.Time, now time.Time, rangeKey string, metric string, limit int) (AdminOperationsSnapshot, error) {
	ranking, err := r.AdminOperationsRanking(ctx, startAt, now, rangeKey, metric, limit)
	if err != nil {
		return AdminOperationsSnapshot{}, err
	}
	live, err := r.AdminOperationsLive(ctx, now)
	if err != nil {
		return AdminOperationsSnapshot{}, err
	}
	return AdminOperationsSnapshot{
		Range:                 ranking.Range,
		Metric:                ranking.Metric,
		ActiveUsers:           live.ActiveUsers,
		ActiveRequests:        live.ActiveRequests,
		QueuedRequests:        live.QueuedRequests,
		ProcessingRequests:    live.ProcessingRequests,
		SlowRequests:          live.SlowRequests,
		AverageElapsedSeconds: live.AverageElapsedSeconds,
		TopUsers:              ranking.TopUsers,
		ActiveCalls:           live.ActiveCalls,
		GeneratedAt:           now.Format(time.RFC3339),
	}, nil
}

func (r *Repository) AdminOperationsRanking(ctx context.Context, startAt time.Time, now time.Time, rangeKey string, metric string, limit int) (AdminOperationsRankingSnapshot, error) {
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	metric = normalizeAdminOperationsMetric(metric)
	orderBy := map[string]string{
		"requests": "request_count",
		"images":   "image_count",
		"credits":  "credits_spent",
		"failures": "failed_count",
		"duration": "average_duration_seconds",
	}[metric]
	durationSecondsExpression := adminOperationsLogDurationSecondsExpression()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	snapshot := AdminOperationsRankingSnapshot{
		Range:       strings.TrimSpace(rangeKey),
		Metric:      metric,
		TopUsers:    []AdminOperationsTopUser{},
		GeneratedAt: now.Format(time.RFC3339),
	}
	topRows, err := r.db.QueryContext(ctx, `
		SELECT
			api_access_logs.user_id,
			users.email,
			COALESCE(users.credits, 0) AS available_balance,
			COALESCE(today_usage.today_credits_spent, 0) AS today_credits_spent,
			CASE
				WHEN COUNT(DISTINCT COALESCE(api_access_keys.billing_mode, 'auto')) > 1 THEN 'mixed'
				ELSE COALESCE(MAX(api_access_keys.billing_mode), 'auto')
			END AS billing_mode,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('success', 'succeeded') THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('success', 'succeeded') THEN api_access_logs.image_count ELSE 0 END), 0) AS image_count,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('success', 'succeeded') THEN api_access_logs.charged_credits ELSE 0 END), 0) AS credits_spent,
			COALESCE(AVG(CASE
				WHEN api_access_logs.status IN ('success', 'succeeded', 'failed', 'canceled', 'cancelled') THEN
					CASE
						WHEN generation_tasks.duration_seconds > 0 AND generation_tasks.duration_seconds <= 300 THEN generation_tasks.duration_seconds
						WHEN api_access_logs.finished_at IS NOT NULL
							AND api_access_logs.finished_at >= api_access_logs.created_at
							AND `+durationSecondsExpression+` <= 300 THEN `+durationSecondsExpression+`
						ELSE NULL
					END
			END), 0) AS average_duration_seconds,
			MAX(api_access_logs.created_at) AS last_request_at
		FROM api_access_logs
		LEFT JOIN users ON users.id = api_access_logs.user_id
		LEFT JOIN (
			SELECT
				today_logs.user_id,
				COALESCE(SUM(CASE WHEN today_logs.status IN ('success', 'succeeded') THEN today_logs.charged_credits ELSE 0 END), 0) AS today_credits_spent
			FROM api_access_logs AS today_logs
			WHERE today_logs.created_at >= ?
			GROUP BY today_logs.user_id
		) AS today_usage ON today_usage.user_id = api_access_logs.user_id
		LEFT JOIN api_access_keys ON api_access_keys.id = api_access_logs.api_key_id
		LEFT JOIN generation_tasks ON generation_tasks.id = api_access_logs.task_id
		WHERE api_access_logs.created_at >= ?
			AND api_access_logs.status IN ('queued', 'processing', 'success', 'succeeded', 'failed', 'canceled', 'cancelled')
		GROUP BY api_access_logs.user_id, users.email, users.credits, today_usage.today_credits_spent
		ORDER BY `+orderBy+` DESC, request_count DESC, last_request_at DESC
		LIMIT ?
	`, todayStart, startAt, limit)
	if err != nil {
		return snapshot, err
	}
	for topRows.Next() {
		var item AdminOperationsTopUser
		var email sql.NullString
		var lastRequestAt time.Time
		if err := topRows.Scan(
			&item.UserID,
			&email,
			&item.AvailableBalance,
			&item.TodayCreditsSpent,
			&item.BillingMode,
			&item.RequestCount,
			&item.SuccessCount,
			&item.FailedCount,
			&item.ImageCount,
			&item.CreditsSpent,
			&item.AverageDurationSeconds,
			&lastRequestAt,
		); err != nil {
			topRows.Close()
			return snapshot, err
		}
		if email.Valid {
			item.UserEmail = &email.String
		}
		completed := item.SuccessCount + item.FailedCount
		if completed > 0 {
			item.SuccessRate = float64(item.SuccessCount) / float64(completed) * 100
		}
		item.LastRequestAt = appclock.DatabaseTime(lastRequestAt).Format(time.RFC3339)
		snapshot.TopUsers = append(snapshot.TopUsers, item)
	}
	if err := topRows.Close(); err != nil {
		return snapshot, err
	}
	if err := topRows.Err(); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func adminOperationsLogDurationSecondsExpression() string {
	if database.CurrentDialect() == database.DialectPostgres {
		return "EXTRACT(EPOCH FROM (api_access_logs.finished_at - api_access_logs.created_at))"
	}
	return "TIMESTAMPDIFF(MICROSECOND, api_access_logs.created_at, api_access_logs.finished_at) / 1000000.0"
}

func (r *Repository) AdminOperationsLive(ctx context.Context, now time.Time) (AdminOperationsLiveSnapshot, error) {
	snapshot := AdminOperationsLiveSnapshot{
		ActiveCalls: []AdminOperationsActiveCall{},
		GeneratedAt: now.Format(time.RFC3339),
	}
	activeRows, err := r.db.QueryContext(ctx, `
		SELECT
			api_access_logs.id,
			generation_tasks.id,
			api_access_logs.user_id,
			users.email,
			api_access_logs.api_key_id,
			api_access_keys.name,
			api_access_keys.key_prefix,
			COALESCE(api_access_keys.billing_mode, 'auto'),
			COALESCE(api_access_keys.concurrency_limit, 1),
			COALESCE(ai_models.display_name, api_access_logs.model, ''),
			generation_tasks.size_tier,
			generation_tasks.size,
			generation_tasks.quantity,
			generation_tasks.status,
			generation_tasks.created_at
		FROM generation_tasks
		INNER JOIN api_access_logs ON api_access_logs.task_id = generation_tasks.id
		LEFT JOIN users ON users.id = api_access_logs.user_id
		LEFT JOIN api_access_keys ON api_access_keys.id = api_access_logs.api_key_id
		LEFT JOIN ai_models ON ai_models.id = generation_tasks.model_id
		WHERE generation_tasks.status IN ('queued', 'pending', 'processing')
		ORDER BY generation_tasks.created_at ASC, generation_tasks.id ASC
	`)
	if err != nil {
		return snapshot, err
	}
	activeUsers := map[string]bool{}
	activeByKey := map[string]int{}
	totalElapsed := 0.0
	for activeRows.Next() {
		var item AdminOperationsActiveCall
		var email, keyName, keyPrefix, size sql.NullString
		var createdAt time.Time
		if err := activeRows.Scan(
			&item.LogID,
			&item.TaskID,
			&item.UserID,
			&email,
			&item.APIKeyID,
			&keyName,
			&keyPrefix,
			&item.BillingMode,
			&item.ConcurrencyLimit,
			&item.Model,
			&item.SizeTier,
			&size,
			&item.Quantity,
			&item.Status,
			&createdAt,
		); err != nil {
			activeRows.Close()
			return snapshot, err
		}
		if email.Valid {
			item.UserEmail = &email.String
		}
		if keyName.Valid {
			item.KeyName = &keyName.String
		}
		if keyPrefix.Valid {
			item.KeyPrefix = &keyPrefix.String
		}
		if size.Valid {
			item.Size = &size.String
		}
		item.ConcurrencyLimit = normalizedConcurrencyLimit(item.ConcurrencyLimit)
		createdAt = appclock.DatabaseTime(createdAt)
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.ElapsedSeconds = now.Sub(createdAt).Seconds()
		if item.ElapsedSeconds < 0 {
			item.ElapsedSeconds = 0
		}
		activeUsers[item.UserID] = true
		activeByKey[item.APIKeyID]++
		totalElapsed += item.ElapsedSeconds
		snapshot.ActiveCalls = append(snapshot.ActiveCalls, item)
	}
	if err := activeRows.Close(); err != nil {
		return snapshot, err
	}
	if err := activeRows.Err(); err != nil {
		return snapshot, err
	}
	for index := range snapshot.ActiveCalls {
		item := &snapshot.ActiveCalls[index]
		item.ActiveForKey = activeByKey[item.APIKeyID]
		switch item.Status {
		case "processing":
			snapshot.ProcessingRequests++
		default:
			snapshot.QueuedRequests++
		}
		if item.ElapsedSeconds >= 120 {
			snapshot.SlowRequests++
		}
	}
	snapshot.ActiveUsers = len(activeUsers)
	snapshot.ActiveRequests = len(snapshot.ActiveCalls)
	if snapshot.ActiveRequests > 0 {
		snapshot.AverageElapsedSeconds = totalElapsed / float64(snapshot.ActiveRequests)
	}
	return snapshot, nil
}

func (r *Repository) AdminOperationsTrend(ctx context.Context, now time.Time, minutes int) (AdminOperationsTrendSnapshot, error) {
	if minutes < 1 {
		minutes = 60
	}
	if minutes > 1440 {
		minutes = 1440
	}
	currentMinute := now.Truncate(time.Minute)
	startAt := currentMinute.Add(-time.Duration(minutes-1) * time.Minute)
	endAt := currentMinute.Add(time.Minute)
	bucketExpression := adminOperationsMinuteBucketExpression()
	snapshot := AdminOperationsTrendSnapshot{
		Minutes:     minutes,
		Points:      make([]AdminOperationsTrendPoint, 0, minutes),
		GeneratedAt: now.Format(time.RFC3339),
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			`+bucketExpression+` AS minute_bucket,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('success', 'succeeded') THEN 1 ELSE 0 END), 0) AS success,
			COALESCE(SUM(CASE WHEN api_access_logs.status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS failed
		FROM api_access_logs
		WHERE api_access_logs.created_at >= ?
			AND api_access_logs.created_at < ?
			AND api_access_logs.status IN ('queued', 'pending', 'processing', 'success', 'succeeded', 'failed', 'canceled', 'cancelled')
		GROUP BY `+bucketExpression+`
		ORDER BY `+bucketExpression+` ASC
	`, startAt, endAt)
	if err != nil {
		return snapshot, err
	}
	byMinute := make(map[string]AdminOperationsTrendPoint, minutes)
	for rows.Next() {
		var bucket string
		var point AdminOperationsTrendPoint
		if err := rows.Scan(&bucket, &point.Total, &point.Success, &point.Failed); err != nil {
			rows.Close()
			return snapshot, err
		}
		byMinute[bucket] = point
	}
	if err := rows.Close(); err != nil {
		return snapshot, err
	}
	if err := rows.Err(); err != nil {
		return snapshot, err
	}
	for minute := startAt; minute.Before(endAt); minute = minute.Add(time.Minute) {
		key := minute.Format("2006-01-02 15:04")
		point := byMinute[key]
		point.Timestamp = minute.Format(time.RFC3339)
		snapshot.Points = append(snapshot.Points, point)
	}
	return snapshot, nil
}

func adminOperationsMinuteBucketExpression() string {
	if database.CurrentDialect() == database.DialectPostgres {
		return "TO_CHAR(DATE_TRUNC('minute', api_access_logs.created_at), 'YYYY-MM-DD HH24:MI')"
	}
	return "DATE_FORMAT(api_access_logs.created_at, '%Y-%m-%d %H:%i')"
}

func normalizeAdminOperationsMetric(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "images", "credits", "failures", "duration":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "credits"
	}
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
			COALESCE(SUM(CASE WHEN status IN ('failed', 'canceled', 'cancelled') THEN 1 ELSE 0 END), 0) AS today_failed,
			COALESCE(SUM(CASE WHEN status = 'success' THEN image_count ELSE 0 END), 0) AS today_image_count
		FROM api_access_logs
		WHERE created_at >= CURRENT_DATE
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
	var userEmail, keyPlain, billingMode, lastError sql.NullString
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
		&billingMode,
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
	if billingMode.Valid {
		item.BillingMode = normalizedStoredBillingMode(strings.TrimSpace(billingMode.String))
	} else {
		item.BillingMode = BillingModeAuto
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
	var userEmail, keyName, keyPrefix, accessIP, accessHost, taskID, requestParams, taskResult, taskUsage, errorMessage, errorCode, errorDetails sql.NullString
	var finishedAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&userEmail,
		&item.APIKeyID,
		&keyName,
		&keyPrefix,
		&accessIP,
		&accessHost,
		&taskID,
		&item.Endpoint,
		&item.Model,
		&item.Prompt,
		&item.Size,
		&item.Quality,
		&item.Quantity,
		&item.ImageCount,
		&item.ResponseFormat,
		&requestParams,
		&item.Status,
		&errorMessage,
		&item.ResponseStatusCode,
		&errorCode,
		&errorDetails,
		&item.ChargedCredits,
		&item.ModelCostCredits,
		&item.DurationSeconds,
		&taskResult,
		&taskUsage,
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
	if accessIP.Valid {
		item.AccessIP = accessIP.String
	}
	if accessHost.Valid {
		item.AccessHost = accessHost.String
	}
	if taskID.Valid {
		item.TaskID = &taskID.String
	}
	if requestParams.Valid && strings.TrimSpace(requestParams.String) != "" {
		if err := json.Unmarshal([]byte(requestParams.String), &item.RequestParams); err != nil {
			return nil, err
		}
	}
	if taskUsage.Valid && strings.TrimSpace(taskUsage.String) != "" {
		if err := json.Unmarshal([]byte(taskUsage.String), &item.TaskUsage); err != nil {
			return nil, err
		}
	}
	if taskResult.Valid && strings.TrimSpace(taskResult.String) != "" {
		if err := json.Unmarshal([]byte(taskResult.String), &item.TaskResult); err != nil {
			return nil, err
		}
	}
	if errorMessage.Valid && strings.TrimSpace(errorMessage.String) != "" {
		item.ErrorMessage = &errorMessage.String
	}
	if errorCode.Valid && strings.TrimSpace(errorCode.String) != "" {
		item.ErrorCode = &errorCode.String
	}
	if errorDetails.Valid && strings.TrimSpace(errorDetails.String) != "" {
		var details apierrors.Details
		if err := json.Unmarshal([]byte(errorDetails.String), &details); err != nil {
			return nil, err
		}
		details.StatusCode = item.ResponseStatusCode
		apierrors.Normalize(&details)
		item.ErrorDetails = &details
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
