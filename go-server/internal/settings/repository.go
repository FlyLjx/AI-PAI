package settings

import (
	"context"
	"errors"
	"math"
	"net/mail"
	"net/url"
	"strconv"
	"strings"

	"aipi-go/internal/database"
)

var ErrInvalidRechargeRate = errors.New("充值比例必须大于 0")
var ErrInvalidTaskTimeout = errors.New("API 请求超时必须是 1 到 120 分钟的整数")
var ErrInvalidDynamicConcurrency = errors.New("动态并发配置不正确")
var ErrInvalidInviteSettings = errors.New("邀请奖励或注册风控配置不正确")
var ErrInvalidAdminNotification = errors.New("管理员通知配置不正确")
var ErrInvalidSystemLogCleanup = errors.New("系统日志自动清理配置不正确")
var ErrInvalidSubscriptionAccessUser = errors.New("订阅开放账号配置不正确")

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
		if key == "subscriptionAccessUserId" {
			userID, ok := value.(string)
			if !ok {
				return nil, ErrInvalidSubscriptionAccessUser
			}
			value = strings.TrimSpace(userID)
		}
		if key == "taskTimeoutMinutes" {
			number, ok := numericSettingValue(value)
			if !ok || number < MinTaskTimeoutMinutes || number > MaxTaskTimeoutMinutes || math.Trunc(number) != number || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidTaskTimeout
			}
			value = number
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
		if key == "adminNotificationEmails" {
			normalized, err := NormalizeNotificationEmails(value)
			if err != nil {
				return nil, ErrInvalidAdminNotification
			}
			value = normalized
		}
		if key == "barkEnabled" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidAdminNotification
			}
		}
		if key == "barkServerUrl" {
			server, ok := value.(string)
			if !ok {
				return nil, ErrInvalidAdminNotification
			}
			server = strings.TrimSpace(server)
			if server == "" {
				server = Defaults["barkServerUrl"].(string)
			}
			parsed, err := url.Parse(server)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
				return nil, ErrInvalidAdminNotification
			}
			value = strings.TrimRight(server, "/")
		}
		if key == "barkDeviceKey" || key == "barkGroup" || key == "barkSound" {
			text, ok := value.(string)
			if !ok {
				return nil, ErrInvalidAdminNotification
			}
			value = strings.TrimSpace(text)
		}
		if key == "barkNotifyError" || key == "barkNotifyRecharge" || key == "barkNotifyUpstream" || key == "barkNotifySystem" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidAdminNotification
			}
		}
		if key == "adminRechargeNotificationEnabled" || key == "adminUpstreamNotificationEnabled" || key == "adminOpenAIStatusNotificationEnabled" || key == "upstreamMaintenanceEnabled" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidAdminNotification
			}
		}
		if key == "systemLogAutoCleanupEnabled" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidSystemLogCleanup
			}
		}
		if key == "systemLogRetentionDays" {
			number, ok := numericSettingValue(value)
			if !ok || number < 1 || number > 3650 || math.Trunc(number) != number || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidSystemLogCleanup
			}
			value = number
		}
		if key == "taskImageRetentionDays" {
			number, ok := numericSettingValue(value)
			if !ok || number < 1 || number > 3650 || math.Trunc(number) != number || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, ErrInvalidSystemLogCleanup
			}
			value = number
		}
		if key == "taskImageAutoCleanupEnabled" {
			if _, ok := value.(bool); !ok {
				return nil, ErrInvalidSystemLogCleanup
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
		if key == "inviteRechargeRebateEnabled" || key == "inviteRebateIncludeSubscriptions" || key == "inviteRiskEnabled" || key == "inviteRiskManualReview" || key == "inviteRiskBlockSameIP" || key == "inviteRiskBlockSameDevice" {
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

// NormalizeNotificationEmails accepts comma, semicolon, or newline separated
// addresses and returns a de-duplicated value suitable for system_settings.
func NormalizeNotificationEmails(value any) (string, error) {
	raw, ok := value.(string)
	if !ok {
		return "", ErrInvalidAdminNotification
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		address := strings.TrimSpace(part)
		if address == "" {
			continue
		}
		parsed, err := mail.ParseAddress(address)
		if err != nil || parsed.Address != address || !strings.Contains(parsed.Address, "@") {
			return "", ErrInvalidAdminNotification
		}
		key := strings.ToLower(parsed.Address)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, parsed.Address)
		if len(result) > 20 {
			return "", ErrInvalidAdminNotification
		}
	}
	return strings.Join(result, ", "), nil
}

// ParseNotificationEmails validates and expands the stored recipient list.
func ParseNotificationEmails(value string) ([]string, error) {
	normalized, err := NormalizeNotificationEmails(value)
	if err != nil {
		return nil, err
	}
	if normalized == "" {
		return nil, nil
	}
	return strings.Split(normalized, ", "), nil
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
		if key == "systemLogRetentionDays" && (number < 1 || number > 3650 || math.Trunc(number) != number) {
			return Defaults[key]
		}
		if key == "taskImageRetentionDays" && (number < 1 || number > 3650 || math.Trunc(number) != number) {
			return Defaults[key]
		}
		if key == "taskTimeoutMinutes" && (number < MinTaskTimeoutMinutes || number > MaxTaskTimeoutMinutes || math.Trunc(number) != number) {
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
	if key == "adminNotificationEmails" {
		if normalized, err := NormalizeNotificationEmails(value); err == nil {
			return normalized
		}
		return Defaults[key]
	}
	if key == "barkServerUrl" && strings.TrimSpace(value) == "" {
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
