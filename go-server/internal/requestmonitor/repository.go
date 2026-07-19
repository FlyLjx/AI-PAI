package requestmonitor

import (
	"context"
	"encoding/json"
	"fmt"
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

func (repository *Repository) Insert(ctx context.Context, record Record) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO http_request_logs (
			id, method, path, query_params, body_params, source_ip, source_host,
			origin, referer, user_agent, status_code, duration_ms, response_bytes, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.Method, record.Path, nullableJSON(record.QueryParams), nullableJSON(record.BodyParams),
		record.SourceIP, record.SourceHost, record.Origin, record.Referer, record.UserAgent,
		record.StatusCode, record.DurationMS, record.ResponseBytes, record.CreatedAt)
	return err
}

func (repository *Repository) Snapshot(ctx context.Context, filters Filters) (Snapshot, int, error) {
	filters = normalizeFilters(filters)
	where, args := filterWhere(filters)
	result := Snapshot{Range: filters.Range, Trend: []TrendPoint{}, TopEndpoints: []FrequencyItem{}, TopSources: []FrequencyItem{}, Items: []Log{}}

	if err := repository.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status_code >= 400 AND status_code < 500 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status_code >= 500 THEN 1 ELSE 0 END), 0),
			COALESCE(AVG(duration_ms), 0),
			COUNT(DISTINCT CASE WHEN source_ip <> '' THEN source_ip ELSE NULL END)
		FROM http_request_logs `+where, args...).Scan(
		&result.Summary.Total, &result.Summary.Successful, &result.Summary.ClientErrors,
		&result.Summary.ServerErrors, &result.Summary.AverageDurationMS, &result.Summary.UniqueSources,
	); err != nil {
		return result, 0, err
	}
	errors := result.Summary.ClientErrors + result.Summary.ServerErrors
	if result.Summary.Total > 0 {
		result.Summary.ErrorRate = float64(errors) / float64(result.Summary.Total) * 100
	}

	var err error
	if result.Trend, err = repository.trend(ctx, filters, where, args); err != nil {
		return result, 0, err
	}
	if result.TopEndpoints, err = repository.frequency(ctx, "path", where, args); err != nil {
		return result, 0, err
	}
	sourceExpression := "COALESCE(NULLIF(source_host, ''), NULLIF(source_ip, ''), '未知来源')"
	if result.TopSources, err = repository.frequency(ctx, sourceExpression, where, args); err != nil {
		return result, 0, err
	}

	rows, err := repository.db.QueryContext(ctx, `
		SELECT id, method, path, query_params, body_params, source_ip, source_host,
			origin, referer, user_agent, status_code, duration_ms, response_bytes, created_at
		FROM http_request_logs `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, appendArgs(args, filters.PageSize, (filters.Page-1)*filters.PageSize)...)
	if err != nil {
		return result, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var item Log
		var queryParams, bodyParams []byte
		var createdAt time.Time
		if err := rows.Scan(
			&item.ID, &item.Method, &item.Path, &queryParams, &bodyParams, &item.SourceIP, &item.SourceHost,
			&item.Origin, &item.Referer, &item.UserAgent, &item.StatusCode, &item.DurationMS, &item.ResponseBytes, &createdAt,
		); err != nil {
			return result, 0, err
		}
		item.QueryParams = decodeJSON(queryParams)
		item.BodyParams = decodeJSON(bodyParams)
		item.CreatedAt = appclock.DatabaseTime(createdAt).Format(time.RFC3339)
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return result, 0, err
	}
	return result, int(result.Summary.Total), nil
}

func (repository *Repository) frequency(ctx context.Context, expression string, where string, args []any) ([]FrequencyItem, error) {
	query := fmt.Sprintf(`
		SELECT %s AS name, COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END), 0) AS error_count,
			COALESCE(AVG(duration_ms), 0) AS average_duration
		FROM http_request_logs %s
		GROUP BY %s
		ORDER BY request_count DESC, name ASC
		LIMIT 8
	`, expression, where, expression)
	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]FrequencyItem, 0, 8)
	for rows.Next() {
		var item FrequencyItem
		if err := rows.Scan(&item.Name, &item.Count, &item.Errors, &item.AverageDurationMS); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) trend(ctx context.Context, filters Filters, where string, args []any) ([]TrendPoint, error) {
	interval := trendInterval(filters.Range)
	seconds := int64(interval / time.Second)
	bucketExpression := trendBucketExpression(seconds)
	query := fmt.Sprintf(`
		SELECT %s AS bucket, COUNT(*),
			COALESCE(SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END), 0)
		FROM http_request_logs %s
		GROUP BY %s
		ORDER BY bucket ASC
	`, bucketExpression, where, bucketExpression)
	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type totals struct{ total, successful, errors int64 }
	values := map[int64]totals{}
	for rows.Next() {
		var bucket int64
		var value totals
		if err := rows.Scan(&bucket, &value.total, &value.successful, &value.errors); err != nil {
			return nil, err
		}
		values[bucket] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	location := appclock.ConfigureDefault()
	cutoff := rangeCutoff(filters)
	start := cutoff.Unix() / seconds * seconds
	end := filters.Now.Unix() / seconds * seconds
	points := make([]TrendPoint, 0, int((end-start)/seconds)+1)
	for bucket := start; bucket <= end; bucket += seconds {
		value := values[bucket]
		points = append(points, TrendPoint{
			Time:  time.Unix(bucket, 0).In(location).Format(time.RFC3339),
			Total: value.total, Successful: value.successful, Errors: value.errors,
		})
	}
	return points, nil
}

func trendBucketExpression(seconds int64) string {
	if database.CurrentDialect() == database.DialectPostgres {
		return fmt.Sprintf("CAST(FLOOR(EXTRACT(EPOCH FROM (created_at AT TIME ZONE 'Asia/Shanghai')) / %d) AS BIGINT) * %d", seconds, seconds)
	}
	return fmt.Sprintf("FLOOR(UNIX_TIMESTAMP(created_at) / %d) * %d", seconds, seconds)
}

func normalizeFilters(filters Filters) Filters {
	filters.Range = strings.ToLower(strings.TrimSpace(filters.Range))
	switch filters.Range {
	case "1h", "24h", "7d", "30d":
	default:
		filters.Range = "24h"
	}
	filters.Keyword = strings.ToLower(strings.TrimSpace(filters.Keyword))
	filters.Method = strings.ToUpper(strings.TrimSpace(filters.Method))
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 {
		filters.PageSize = 30
	}
	if filters.PageSize > 100 {
		filters.PageSize = 100
	}
	if filters.Now.IsZero() {
		filters.Now = time.Now().In(appclock.ConfigureDefault())
	} else {
		filters.Now = filters.Now.In(appclock.ConfigureDefault())
	}
	return filters
}

func filterWhere(filters Filters) (string, []any) {
	conditions := []string{"created_at >= ?"}
	args := []any{rangeCutoff(filters)}
	if filters.Keyword != "" {
		pattern := "%" + filters.Keyword + "%"
		conditions = append(conditions, "(LOWER(path) LIKE ? OR LOWER(source_ip) LIKE ? OR LOWER(source_host) LIKE ? OR LOWER(user_agent) LIKE ?)")
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if filters.Method != "" && filters.Method != "ALL" {
		conditions = append(conditions, "method = ?")
		args = append(args, filters.Method)
	}
	switch filters.Status {
	case "success":
		conditions = append(conditions, "status_code < 400")
	case "client_error":
		conditions = append(conditions, "status_code >= 400 AND status_code < 500")
	case "server_error":
		conditions = append(conditions, "status_code >= 500")
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func rangeCutoff(filters Filters) time.Time {
	switch filters.Range {
	case "1h":
		return filters.Now.Add(-time.Hour)
	case "7d":
		return filters.Now.Add(-7 * 24 * time.Hour)
	case "30d":
		return filters.Now.Add(-30 * 24 * time.Hour)
	default:
		return filters.Now.Add(-24 * time.Hour)
	}
}

func trendInterval(selectedRange string) time.Duration {
	switch selectedRange {
	case "1h":
		return 5 * time.Minute
	case "7d":
		return 6 * time.Hour
	case "30d":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

func decodeJSON(value []byte) any {
	if len(value) == 0 {
		return map[string]any{}
	}
	var result any
	if json.Unmarshal(value, &result) != nil {
		return string(value)
	}
	return result
}

func appendArgs(args []any, values ...any) []any {
	result := make([]any, 0, len(args)+len(values))
	result = append(result, args...)
	return append(result, values...)
}
