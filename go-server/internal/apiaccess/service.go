package apiaccess

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aipi-go/internal/users"
)

var (
	ErrMissingKey          = errors.New("缺少 API Key")
	ErrInvalidKey          = errors.New("API Key 无效或已禁用")
	ErrAccessKeyNotFound   = errors.New("API Key 不存在或无权查看")
	ErrKeyPlainUnavailable = errors.New("当前 API Key 没有可查看的明文")
	ErrInvalidBillingMode  = errors.New("计费模式必须是 subscription 或 balance")
)

type Authenticated struct {
	APIKey AccessKey
	User   users.User
}

type Service struct {
	keys                     *Repository
	users                    *users.Repository
	dynamicConcurrencyConfig DynamicConcurrencyConfig
}

func NewService(keyRepo *Repository, userRepo *users.Repository) Service {
	return Service{keys: keyRepo, users: userRepo, dynamicConcurrencyConfig: DefaultDynamicConcurrencyConfig()}
}

func (s Service) WithDynamicConcurrencyConfig(config DynamicConcurrencyConfig) Service {
	s.dynamicConcurrencyConfig = NormalizeDynamicConcurrencyConfig(config)
	return s
}

func (s Service) Authenticate(ctx context.Context, raw string) (*Authenticated, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return nil, ErrMissingKey
	}
	hash := HashKey(key)
	candidates, err := s.keys.FindActiveByPrefix(ctx, KeyPrefix(key))
	if err != nil {
		return nil, err
	}
	var matched *AccessKey
	for index := range candidates {
		if subtle.ConstantTimeCompare([]byte(candidates[index].KeyHash), []byte(hash)) == 1 {
			matched = &candidates[index]
			break
		}
	}
	if matched == nil {
		return nil, ErrInvalidKey
	}
	user, err := s.users.FindByID(ctx, matched.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrInvalidKey
	}
	_ = s.keys.MarkUsed(ctx, matched.ID)
	return &Authenticated{APIKey: *matched, User: *user}, nil
}

func (s Service) ListUserKeys(ctx context.Context, userID string) ([]PublicAccessKey, error) {
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return nil, err
	}
	_ = s.keys.SyncTerminalTaskLogs(ctx, 200)
	keys, err := s.keys.ListKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.attachWindowRequestCounts(ctx, keys); err != nil {
		return nil, err
	}
	return publicKeys(keys, s.dynamicConcurrencyConfig), nil
}

func (s Service) ListAllKeys(ctx context.Context) ([]PublicAccessKey, error) {
	_ = s.keys.SyncTerminalTaskLogs(ctx, 200)
	keys, err := s.keys.ListKeys(ctx, "")
	if err != nil {
		return nil, err
	}
	if err := s.attachWindowRequestCounts(ctx, keys); err != nil {
		return nil, err
	}
	return publicKeys(keys, s.dynamicConcurrencyConfig), nil
}

func (s Service) attachWindowRequestCounts(ctx context.Context, keys []AccessKey) error {
	config := NormalizeDynamicConcurrencyConfig(s.dynamicConcurrencyConfig)
	if !config.Enabled {
		return nil
	}
	ids := make([]string, 0, len(keys))
	for _, key := range keys {
		ids = append(ids, key.ID)
	}
	counts, err := s.keys.RequestCountsSince(ctx, ids, time.Now().Add(-config.Window()))
	if err != nil {
		return err
	}
	for index := range keys {
		keys[index].WindowRequestCount = counts[keys[index].ID]
	}
	return nil
}

func (s Service) CreateUserKey(ctx context.Context, userID string, name string, billingMode string) (*PublicAccessKey, error) {
	billingMode, err := normalizeNewBillingMode(billingMode)
	if err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, errors.New("用户已被禁用")
	}
	if err := users.RequireEmailVerifiedForAPIKey(user); err != nil {
		return nil, err
	}
	raw := CreateRawKey()
	keyName := strings.TrimSpace(name)
	if keyName == "" {
		keyName = "默认 Key"
	}
	plain := raw
	key, err := s.keys.CreateKey(ctx, AccessKey{
		ID:               NewID(),
		UserID:           userID,
		Name:             keyName,
		KeyPrefix:        KeyPrefix(raw),
		KeyHash:          HashKey(raw),
		KeyPlain:         &plain,
		Status:           "active",
		ConcurrencyLimit: 10,
		BillingMode:      billingMode,
	})
	if err != nil {
		return nil, err
	}
	public := ToPublicKey(*key)
	public.Key = &raw
	return &public, nil
}

func normalizeNewBillingMode(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return BillingModeBalance, nil
	}
	if value != BillingModeSubscription && value != BillingModeBalance {
		return "", ErrInvalidBillingMode
	}
	return value, nil
}

func (s Service) UpdateKeyStatus(ctx context.Context, id string, userID string, status string) (*PublicAccessKey, error) {
	return s.UpdateKeySettings(ctx, id, userID, status, nil)
}

func (s Service) RevealUserKey(ctx context.Context, id string, userID string) (string, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(userID) == "" {
		return "", ErrAccessKeyNotFound
	}
	keyPlain, err := s.keys.FindKeyPlainForUser(ctx, id, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrAccessKeyNotFound
	}
	if err != nil {
		return "", err
	}
	if keyPlain == nil || strings.TrimSpace(*keyPlain) == "" {
		return "", ErrKeyPlainUnavailable
	}
	return *keyPlain, nil
}

func (s Service) UpdateKeySettings(ctx context.Context, id string, userID string, status string, concurrencyLimit *int) (*PublicAccessKey, error) {
	if strings.TrimSpace(status) != "" && status != "active" && status != "disabled" {
		return nil, errors.New("状态不正确")
	}
	if concurrencyLimit != nil {
		if *concurrencyLimit < 1 {
			return nil, errors.New("并发上限必须大于 0")
		}
	}
	updated, err := s.keys.UpdateKeySettings(ctx, id, userID, status, concurrencyLimit)
	if err != nil {
		return nil, err
	}
	public := ToPublicKey(*updated)
	return &public, nil
}

func (s Service) DeleteKey(ctx context.Context, id string, userID string) error {
	deleted, err := s.keys.DeleteKey(ctx, id, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return errors.New("API Key 不存在或已删除")
	}
	return nil
}

func (s Service) ListLogs(ctx context.Context, input ListLogsInput) ([]PublicUsageLog, int, error) {
	_ = s.keys.SyncTerminalTaskLogs(ctx, 200)
	items, total, err := s.keys.ListLogs(ctx, input)
	if err != nil {
		return nil, 0, err
	}
	result := make([]PublicUsageLog, 0, len(items))
	for _, item := range items {
		result = append(result, ToPublicLog(item))
	}
	return result, total, nil
}

func (s Service) ListAdminLogs(ctx context.Context, input ListLogsInput) ([]AdminPublicUsageLog, int, error) {
	_ = s.keys.SyncTerminalTaskLogs(ctx, 200)
	items, total, err := s.keys.ListLogs(ctx, input)
	if err != nil {
		return nil, 0, err
	}
	result := make([]AdminPublicUsageLog, 0, len(items))
	for _, item := range items {
		result = append(result, ToAdminPublicLog(item))
	}
	return result, total, nil
}

func (s Service) ListLogStats(ctx context.Context, input ListLogsInput) (UsageStats, error) {
	return s.keys.LogStats(ctx, input)
}

func (s Service) UsageTrend(ctx context.Context, userID string, startDate time.Time, endDate time.Time) ([]UsageTrendPoint, error) {
	for {
		synced, err := s.keys.syncTerminalTaskLogBatch(ctx, 500)
		if err != nil {
			return nil, err
		}
		if synced < 500 {
			break
		}
	}
	items, err := s.keys.DailyUsageTrend(ctx, userID, startDate, endDate.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	return fillUsageTrend(startDate, endDate, items), nil
}

func fillUsageTrend(startDate time.Time, endDate time.Time, items []UsageTrendPoint) []UsageTrendPoint {
	byDate := make(map[string]UsageTrendPoint, len(items))
	for _, item := range items {
		byDate[item.Date] = item
	}

	result := make([]UsageTrendPoint, 0)
	for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		item, ok := byDate[date]
		if !ok {
			item = UsageTrendPoint{Date: date}
		}
		result = append(result, item)
	}
	return result
}

func publicKeys(keys []AccessKey, config DynamicConcurrencyConfig) []PublicAccessKey {
	result := make([]PublicAccessKey, 0, len(keys))
	for _, key := range keys {
		result = append(result, ToPublicKeyWithConfig(key, config))
	}
	return result
}

func HashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func KeyPrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 18 {
		return value
	}
	return value[:18]
}

func CreateRawKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "sk-aipai-fallback"
	}
	return "sk-aipai-" + base64.RawURLEncoding.EncodeToString(bytes)
}

func NewID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	value := hex.EncodeToString(bytes)
	return fmt.Sprintf("%s-%s-%s-%s-%s", value[0:8], value[8:12], value[12:16], value[16:20], value[20:32])
}
