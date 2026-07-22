package settings

import (
	"context"
	"errors"
	"math"
	"strconv"

	"aipi-go/internal/database"
)

var ErrInvalidRechargeRate = errors.New("充值比例必须大于 0")
var ErrInvalidDynamicConcurrency = errors.New("动态并发配置不正确")
var ErrInvalidInviteSettings = errors.New("邀请奖励或注册风控配置不正确")
var ErrInvalidAdminNotification = errors.New("管理员通知配置不正确")

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
		if key == "rechargeRate" {
			rate, ok := numericSettingValue(value)
			if !ok || rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
				return nil, ErrInvalidRechargeRate
			}
			value = rate
		}
		if key == "dynamicConcurrencyEnabled" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidDynamicConcurrency
			}
		}
		if key == "dynamicConcurrencyWindowUnit" {
			unit, ok := value.(string)
			if !ok || (unit != "minute" && unit != "hour") {
				return nil, ErrInvalidDynamicConcurrency
			}
		}
		if key == "dynamicConcurrencyWindowValue" || key == "dynamicConcurrencyRequestStep" || key == "dynamicConcurrencyIncrement" {
			number, ok := numericSettingValue(value)
			if !ok || number < 1 || number > 1000000 || math.Trunc(number) != number || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidDynamicConcurrency
			}
			value = number
		}
		if key == "adminUpstreamCheckIntervalMinutes" {
			number, ok := numericSettingValue(value)
			if !ok || number < 1 || number > 1440 || math.Trunc(number) != number || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidAdminNotification
			}
			value = number
		}
		if key == "adminRechargeNotificationEnabled" || key == "adminUpstreamNotificationEnabled" || key == "adminOpenAIStatusNotificationEnabled" || key == "upstreamMaintenanceEnabled" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidAdminNotification
			}
		}
		if key == "inviteInviterRewardType" || key == "inviteInviteeRewardType" || key == "inviteRewardType" {
			rewardType, ok := value.(string)
			if !ok || (rewardType != "none" && rewardType != "balance" && rewardType != "subscription") {
				return nil, ErrInvalidInviteSettings
			}
		}
		if key == "inviteInviterRewardCredits" || key == "inviteInviteeRewardCredits" {
			number, ok := numericSettingValue(value)
			if !ok || number < 0 || number > 100000000 || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidInviteSettings
			}
			value = number
		}
		if key == "inviteRechargeRebatePercent" {
			number, ok := numericSettingValue(value)
			if !ok || number <= 0 || number > 100 || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidInviteSettings
			}
			value = number
		}
		if key == "inviteRechargeRebateEnabled" || key == "inviteRebateIncludeSubscriptions" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidInviteSettings
			}
		}
		if isInviteRiskIntegerKey(key) {
			number, ok := numericSettingValue(value)
			if !ok || number < 1 || number > 1000000 || math.Trunc(number) != number || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidInviteSettings
			}
			value = number
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

func isInviteRiskIntegerKey(key string) bool {
	switch key {
	case "inviteRiskMaxPerIP24h", "inviteRiskMaxPerDevice24h", "inviteRiskMaxPerInviter24h",
		"registrationRiskMaxPerIP24h", "registrationRiskMaxPerDevice24h",
		"registrationChallengeMinSeconds", "registrationChallengeMaxPerIPHour":
		return true
	default:
		return false
	}
}

func numericSettingValue(value any) (float64, bool) {
	switch item := value.(type) {
	case float64:
		return item, true
	case float32:
		return float64(item), true
	case int:
		return float64(item), true
	case int64:
		return float64(item), true
	default:
		return 0, false
	}
}

func parseValue(key string, value string) any {
	if _, ok := Defaults[key].(bool); ok {
		return value == "true" || value == "1"
	}
	if _, ok := Defaults[key].(float64); ok {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return Defaults[key]
		}
		if key == "rechargeRate" && number <= 0 {
			return Defaults[key]
		}
		if (key == "dynamicConcurrencyWindowValue" || key == "dynamicConcurrencyRequestStep" || key == "dynamicConcurrencyIncrement") &&
			(number < 1 || number > 1000000 || math.Trunc(number) != number) {
			return Defaults[key]
		}
		if key == "adminUpstreamCheckIntervalMinutes" && (number < 1 || number > 1440 || math.Trunc(number) != number) {
			return Defaults[key]
		}
		if isInviteRiskIntegerKey(key) && (number < 1 || number > 1000000 || math.Trunc(number) != number) {
			return Defaults[key]
		}
		if (key == "inviteInviterRewardCredits" || key == "inviteInviteeRewardCredits") && (number < 0 || number > 100000000) {
			return Defaults[key]
		}
		if key == "inviteRechargeRebatePercent" && (number <= 0 || number > 100) {
			return Defaults[key]
		}
		return number
	}
	if key == "dynamicConcurrencyWindowUnit" && value != "minute" && value != "hour" {
		return Defaults[key]
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
