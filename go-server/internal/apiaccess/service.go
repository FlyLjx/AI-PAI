package apiaccess

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"aipi-go/internal/users"
)

var (
	ErrMissingKey = errors.New("缺少 API Key")
	ErrInvalidKey = errors.New("API Key 无效或已禁用")
)

type Authenticated struct {
	APIKey AccessKey
	User   users.User
}

type Service struct {
	keys  *Repository
	users *users.Repository
}

func NewService(keyRepo *Repository, userRepo *users.Repository) Service {
	return Service{keys: keyRepo, users: userRepo}
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
	return publicKeys(keys), nil
}

func (s Service) ListAllKeys(ctx context.Context) ([]PublicAccessKey, error) {
	_ = s.keys.SyncTerminalTaskLogs(ctx, 200)
	keys, err := s.keys.ListKeys(ctx, "")
	if err != nil {
		return nil, err
	}
	return publicKeys(keys), nil
}

func (s Service) CreateUserKey(ctx context.Context, userID string, name string) (*PublicAccessKey, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, errors.New("用户已被禁用")
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
	})
	if err != nil {
		return nil, err
	}
	public := ToPublicKey(*key)
	public.Key = &raw
	return &public, nil
}

func (s Service) UpdateKeyStatus(ctx context.Context, id string, userID string, status string) (*PublicAccessKey, error) {
	return s.UpdateKeySettings(ctx, id, userID, status, nil)
}

func (s Service) UpdateKeySettings(ctx context.Context, id string, userID string, status string, concurrencyLimit *int) (*PublicAccessKey, error) {
	if strings.TrimSpace(status) != "" && status != "active" && status != "disabled" {
		return nil, errors.New("状态不正确")
	}
	if concurrencyLimit != nil {
		if *concurrencyLimit < 1 || *concurrencyLimit > 50 {
			return nil, errors.New("并发上限必须在 1 到 50 之间")
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

func publicKeys(keys []AccessKey) []PublicAccessKey {
	result := make([]PublicAccessKey, 0, len(keys))
	for _, key := range keys {
		result = append(result, ToPublicKey(key))
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
