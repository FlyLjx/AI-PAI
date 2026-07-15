package database

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reDateSubNowHour = regexp.MustCompile(`DATE_SUB\(NOW\(\),\s*INTERVAL\s+(\d+)\s+HOUR\)`)
	reDateSubNowDay  = regexp.MustCompile(`DATE_SUB\(NOW\(\),\s*INTERVAL\s+(\d+)\s+DAY\)`)
	reDateSubCurDay  = regexp.MustCompile(`DATE_SUB\(CURDATE\(\),\s*INTERVAL\s+(\d+)\s+DAY\)`)
	reDateSubNowQH   = regexp.MustCompile(`DATE_SUB\(NOW\(\),\s*INTERVAL\s+\?\s+HOUR\)`)
	reDateSubNowQD   = regexp.MustCompile(`DATE_SUB\(NOW\(\),\s*INTERVAL\s+\?\s+DAY\)`)
	reDateSubCurQD   = regexp.MustCompile(`DATE_SUB\(CURDATE\(\),\s*INTERVAL\s+\?\s+DAY\)`)
	reNowFn          = regexp.MustCompile(`\bNOW\(\)`)
	reCurDateFn      = regexp.MustCompile(`\bCURDATE\(\)`)
	reSlowCount      = regexp.MustCompile(`COALESCE\(SUM\(duration_ms\s*>=\s*\?\),0\)`)
)

func NormalizeQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" || CurrentDialect() != DialectPostgres {
		return Rebind(query)
	}

	query = reDateSubNowHour.ReplaceAllString(query, `(CURRENT_TIMESTAMP - INTERVAL '$1 hour')`)
	query = reDateSubNowDay.ReplaceAllString(query, `(CURRENT_TIMESTAMP - INTERVAL '$1 day')`)
	query = reDateSubCurDay.ReplaceAllString(query, `(CURRENT_DATE - INTERVAL '$1 day')`)
	query = reDateSubNowQH.ReplaceAllString(query, `(CURRENT_TIMESTAMP - (? * INTERVAL '1 hour'))`)
	query = reDateSubNowQD.ReplaceAllString(query, `(CURRENT_TIMESTAMP - (? * INTERVAL '1 day'))`)
	query = reDateSubCurQD.ReplaceAllString(query, `(CURRENT_DATE - (? * INTERVAL '1 day'))`)
	query = reNowFn.ReplaceAllString(query, "CURRENT_TIMESTAMP")
	query = reCurDateFn.ReplaceAllString(query, "CURRENT_DATE")
	query = strings.ReplaceAll(query, "GROUP_CONCAT(DISTINCT announcement_users.user_id)", "STRING_AGG(DISTINCT announcement_users.user_id, ',')")
	query = strings.ReplaceAll(query, "GROUP_CONCAT(DISTINCT display_name ORDER BY display_name SEPARATOR ', ')", "STRING_AGG(DISTINCT display_name, ', ' ORDER BY display_name)")
	query = strings.ReplaceAll(query, "COALESCE(SUM(status='success'),0)", "COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0)")
	query = strings.ReplaceAll(query, "COALESCE(SUM(status='failed'),0)", "COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0)")
	query = strings.ReplaceAll(query, "COALESCE(SUM(status IN ('queued','pending','processing')),0)", "COALESCE(SUM(CASE WHEN status IN ('queued','pending','processing') THEN 1 ELSE 0 END),0)")
	query = strings.ReplaceAll(query, "COALESCE(SUM(status IN ('failed','canceled')),0)", "COALESCE(SUM(CASE WHEN status IN ('failed','canceled') THEN 1 ELSE 0 END),0)")
	query = reSlowCount.ReplaceAllString(query, "COALESCE(SUM(CASE WHEN duration_ms >= ? THEN 1 ELSE 0 END),0)")
	query = normalizeOnDuplicateKey(query)
	return Rebind(query)
}

func normalizeOnDuplicateKey(query string) string {
	if !strings.Contains(query, "ON DUPLICATE KEY UPDATE") {
		return query
	}
	switch {
	case strings.Contains(query, "INSERT INTO subscription_plans"):
		return postgresUpsert(query, "subscription_plans", "id", []string{"name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent", "allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status"}, true)
	case strings.Contains(query, "INSERT INTO redeem_codes"):
		return postgresUpsert(query, "redeem_codes", "id", []string{"code", "credits", "status", "remark", "expires_at"}, true)
	case strings.Contains(query, "INSERT INTO system_settings"):
		return postgresUpsert(query, "system_settings", "setting_key", []string{"setting_value"}, false)
	case strings.Contains(query, "INSERT INTO announcements"):
		return postgresUpsert(query, "announcements", "id", []string{"title", "content", "display_mode", "target_type", "status", "sort_order"}, true)
	case strings.Contains(query, "INSERT INTO ai_models"):
		return postgresUpsert(query, "ai_models", "id", []string{"display_name", "cost_1k", "cost_2k", "cost_4k", "markup_percent", "price_change_percent", "price_1k", "price_2k", "price_4k", "append_size_to_prompt", "enabled_size_tiers", "sort_order"}, true)
	case strings.Contains(query, "INSERT INTO announcement_receipts"):
		return strings.Replace(query, "ON DUPLICATE KEY UPDATE signed_at = CURRENT_TIMESTAMP", "ON CONFLICT (announcement_id, user_id) DO UPDATE SET signed_at = CURRENT_TIMESTAMP", 1)
	default:
		return query
	}
}

func postgresUpsert(query string, table string, conflictColumn string, updateColumns []string, touchUpdatedAt bool) string {
	head, _, found := strings.Cut(query, "ON DUPLICATE KEY UPDATE")
	if !found {
		return query
	}
	assignments := make([]string, 0, len(updateColumns)+1)
	for _, column := range updateColumns {
		assignments = append(assignments, fmt.Sprintf("%s = EXCLUDED.%s", column, column))
	}
	if touchUpdatedAt {
		assignments = append(assignments, "updated_at = CURRENT_TIMESTAMP")
	}
	return strings.TrimSpace(head) + fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", conflictColumn, strings.Join(assignments, ", "))
}
