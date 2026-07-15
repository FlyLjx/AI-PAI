package pricing

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/database"
)

const MinUnitPrice = 0.001

type SettingsGetter interface {
	Get(context.Context) (map[string]any, error)
}

type Plan struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
	// NewUserDays is kept for compatibility with old settings, but activity pricing is site-wide.
	NewUserDays     int     `json:"newUserDays"`
	MinUnitPrice    float64 `json:"minUnitPrice"`
	Rules           []Rule  `json:"rules"`
	TodayImageCount int     `json:"todayImageCount,omitempty"`
}

type Rule struct {
	MinImages       int     `json:"minImages"`
	DiscountPercent float64 `json:"discountPercent"`
}

type Result struct {
	Active          bool    `json:"active"`
	Reason          string  `json:"reason"`
	PlanName        string  `json:"planName"`
	TodayImages     int     `json:"todayImages"`
	DiscountPercent float64 `json:"discountPercent"`
	MinUnitPrice    float64 `json:"minUnitPrice"`
	Rule            *Rule   `json:"rule,omitempty"`
	NextRule        *Rule   `json:"nextRule,omitempty"`
	StartedAt       string  `json:"startedAt,omitempty"`
	EndedAt         string  `json:"endedAt,omitempty"`
}

func Evaluate(ctx context.Context, db *database.DB, values map[string]any, userID string, now time.Time) (Result, error) {
	plan := ParsePlan(values)
	result := Result{
		PlanName:     plan.Name,
		MinUnitPrice: plan.MinUnitPrice,
		StartedAt:    plan.StartAt,
		EndedAt:      plan.EndAt,
	}
	if !plan.Enabled {
		result.Reason = "disabled"
		return result, nil
	}
	if len(plan.Rules) == 0 {
		result.Reason = "no_rules"
		return result, nil
	}
	if !withinWindow(plan, now) {
		result.Reason = "outside_time"
		return result, nil
	}
	if strings.TrimSpace(userID) == "" {
		result.Reason = "missing_user"
		return result, nil
	}
	count, err := todaySuccessImages(ctx, db)
	if err != nil {
		return result, err
	}
	result.TodayImages = count
	result.Active = true
	result.Reason = "active"
	result.Rule = activeRule(plan.Rules, count)
	result.NextRule = nextRule(plan.Rules, count)
	if result.Rule != nil {
		result.DiscountPercent = normalizeDiscount(result.Rule.DiscountPercent)
	}
	return result, nil
}

func ApplyUnitPrice(unitPrice float64, incentive Result, subscriptionDiscountPercent float64) (float64, float64, string) {
	price := math.Max(0, unitPrice)
	bestDiscount := 0.0
	source := ""
	if incentive.Active && incentive.DiscountPercent > 0 {
		bestDiscount = normalizeDiscount(incentive.DiscountPercent)
		source = "activity"
	}
	if subscriptionDiscountPercent > bestDiscount {
		bestDiscount = normalizeDiscount(subscriptionDiscountPercent)
		source = "subscription"
	}
	if bestDiscount <= 0 {
		return round4(price), 0, ""
	}
	if bestDiscount >= 100 {
		return 0, 100, source
	}
	minUnit := incentive.MinUnitPrice
	if minUnit <= 0 {
		minUnit = MinUnitPrice
	}
	discounted := price * (1 - bestDiscount/100)
	if price > 0 && discounted < minUnit {
		discounted = minUnit
	}
	return round4(discounted), bestDiscount, source
}

func CurrentSubscriptionDiscount(ctx context.Context, db *database.DB, userID string) (float64, error) {
	var discount sql.NullFloat64
	err := db.QueryRowContext(ctx, `
		SELECT subscription_plans.discount_percent
		FROM user_subscriptions
		LEFT JOIN subscription_plans ON subscription_plans.id = user_subscriptions.plan_id
		WHERE user_subscriptions.user_id = ?
			AND user_subscriptions.status = 'active'
			AND user_subscriptions.expires_at > NOW()
		ORDER BY user_subscriptions.expires_at DESC
		LIMIT 1
	`, userID).Scan(&discount)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !discount.Valid {
		return 0, nil
	}
	return normalizeDiscount(discount.Float64), nil
}

func ParsePlan(values map[string]any) Plan {
	plan := Plan{
		Name:         "全站生图活动",
		NewUserDays:  0,
		MinUnitPrice: MinUnitPrice,
		Rules: []Rule{
			{MinImages: 10, DiscountPercent: 10},
			{MinImages: 30, DiscountPercent: 20},
			{MinImages: 60, DiscountPercent: 30},
		},
	}
	plan.Enabled = boolValue(values["incentiveEnabled"])
	if text := strings.TrimSpace(stringValue(values["incentiveName"])); text != "" {
		plan.Name = text
	}
	plan.StartAt = strings.TrimSpace(stringValue(values["incentiveStartAt"]))
	plan.EndAt = strings.TrimSpace(stringValue(values["incentiveEndAt"]))
	if days := int(numberValue(values["incentiveNewUserDays"], float64(plan.NewUserDays))); days >= 0 {
		plan.NewUserDays = days
	}
	if minUnit := normalizeMinUnitPrice(numberValue(values["incentiveMinUnitPrice"], MinUnitPrice)); minUnit > 0 {
		plan.MinUnitPrice = minUnit
	}
	if rules := parseRules(stringValue(values["incentiveRules"])); len(rules) > 0 {
		plan.Rules = rules
	}
	sortRules(plan.Rules)
	return plan
}

func activeRule(rules []Rule, count int) *Rule {
	var found *Rule
	for index := range rules {
		if count >= rules[index].MinImages {
			rule := rules[index]
			found = &rule
		}
	}
	return found
}

func nextRule(rules []Rule, count int) *Rule {
	for index := range rules {
		if count < rules[index].MinImages {
			rule := rules[index]
			return &rule
		}
	}
	return nil
}

func todaySuccessImages(ctx context.Context, db *database.DB) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM generation_tasks
		WHERE status = 'success'
			AND created_at >= CURDATE()
	`).Scan(&count)
	return count, err
}

func withinWindow(plan Plan, now time.Time) bool {
	if start, ok := parseTime(plan.StartAt); ok && now.Before(start) {
		return false
	}
	if end, ok := parseTime(plan.EndAt); ok && now.After(end) {
		return false
	}
	return true
}

func parseRules(raw string) []Rule {
	var rules []Rule
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &rules) != nil {
		return nil
	}
	cleaned := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if rule.MinImages < 0 || rule.DiscountPercent <= 0 {
			continue
		}
		rule.DiscountPercent = normalizeDiscount(rule.DiscountPercent)
		cleaned = append(cleaned, rule)
	}
	sortRules(cleaned)
	return cleaned
}

func sortRules(rules []Rule) {
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].MinImages < rules[j].MinImages
	})
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func normalizeDiscount(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeMinUnitPrice(value float64) float64 {
	if value <= 0 {
		return MinUnitPrice
	}
	// Older installs may still have the previous default 0.01 saved in DB.
	if value == 0.01 {
		return MinUnitPrice
	}
	if value < MinUnitPrice {
		return MinUnitPrice
	}
	return value
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func boolValue(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return item == "true" || item == "1"
	default:
		return false
	}
}

func numberValue(value any, fallback float64) float64 {
	switch item := value.(type) {
	case float64:
		return item
	case float32:
		return float64(item)
	case int:
		return float64(item)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(item), 64); err == nil {
			return parsed
		}
		return fallback
	default:
		return fallback
	}
}
