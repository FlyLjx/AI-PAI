package apikeys

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
	APIKey APIKey
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
	hash := hashKey(key)
	candidates, err := s.keys.FindActiveByPrefix(ctx, keyPrefix(key))
	if err != nil {
		return nil, err
	}
	var matched *APIKey
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

func (s Service) ListUserKeys(ctx context.Context, userID string) ([]PublicAPIKey, error) {
	if _, err := s.users.FindByID(ctx, userID); err != nil {
		return nil, err
	}
	keys, err := s.keys.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]PublicAPIKey, 0, len(keys))
	for _, key := range keys {
		result = append(result, ToPublic(key))
	}
	return result, nil
}

func (s Service) CreateUserKey(ctx context.Context, userID string, name string) (*PublicAPIKey, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, errors.New("用户已被禁用")
	}
	existing, err := s.keys.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, errors.New("每个用户只允许生成一个 API Key")
	}
	raw := createRawKey()
	plain := raw
	key, err := s.keys.Create(ctx, APIKey{
		ID:        newAPIKeyID(),
		UserID:    userID,
		Name:      strings.TrimSpace(name),
		KeyPrefix: keyPrefix(raw),
		KeyHash:   hashKey(raw),
		KeyPlain:  &plain,
		Status:    "active",
	})
	if err != nil {
		return nil, err
	}
	public := ToPublic(*key)
	public.Key = &raw
	return &public, nil
}

func (s Service) UpdateUserKeyStatus(ctx context.Context, id string, userID string, status string) (*PublicAPIKey, error) {
	if status != "active" && status != "disabled" {
		return nil, errors.New("状态不正确")
	}
	key, err := s.keys.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if userID != "" && key.UserID != userID {
		return nil, errors.New("无权操作该 API Key")
	}
	updated, err := s.keys.UpdateStatus(ctx, id, status, key.UserID)
	if err != nil {
		return nil, err
	}
	public := ToPublic(*updated)
	return &public, nil
}

func (s Service) DeleteUserKey(ctx context.Context, id string, userID string) error {
	deleted, err := s.keys.DeleteByUserID(ctx, id, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return errors.New("API Key 不存在或已删除")
	}
	return nil
}

func (s Service) ListAll(ctx context.Context) ([]PublicAPIKey, error) {
	keys, err := s.keys.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PublicAPIKey, 0, len(keys))
	for _, key := range keys {
		result = append(result, ToPublic(key))
	}
	return result, nil
}

func hashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func keyPrefix(value string) string {
	if len(value) <= 16 {
		return value
	}
	return value[:16]
}

func createRawKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "sk-aipi-fallback"
	}
	return "sk-aipi-" + base64.RawURLEncoding.EncodeToString(bytes)
}

func newAPIKeyID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	value := hex.EncodeToString(bytes)
	return fmt.Sprintf("%s-%s-%s-%s-%s", value[0:8], value[8:12], value[12:16], value[16:20], value[20:32])
}
