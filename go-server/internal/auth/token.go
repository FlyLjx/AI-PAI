package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aipi-go/internal/config"
)

const (
	adminTokenVersion = "v1"
	userTokenVersion  = "user-v1"
)

type TokenPayload struct {
	Version string `json:"version"`
	UserID  string `json:"userId"`
	Exp     int64  `json:"exp"`
}

type TokenManager struct {
	cfg config.DatabaseConfig
}

func NewTokenManager(cfg config.DatabaseConfig) TokenManager {
	return TokenManager{cfg: cfg}
}

func (m TokenManager) CreateAdminToken(userID string) (string, error) {
	return m.createToken(userID, adminTokenVersion, 12*time.Hour, "")
}

func (m TokenManager) ParseAdminToken(token string) (*TokenPayload, error) {
	return m.parseToken(token, adminTokenVersion, "")
}

func (m TokenManager) CreateUserToken(userID string) (string, error) {
	return m.createToken(userID, userTokenVersion, 30*24*time.Hour, ":user")
}

func (m TokenManager) ParseUserToken(token string) (*TokenPayload, error) {
	return m.parseToken(token, userTokenVersion, ":user")
}

func (m TokenManager) createToken(userID string, version string, ttl time.Duration, suffix string) (string, error) {
	payload, err := json.Marshal(TokenPayload{
		Version: version,
		UserID:  userID,
		Exp:     time.Now().Add(ttl).UnixMilli(),
	})
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	return encodedPayload + "." + m.sign(encodedPayload, suffix), nil
}

func (m TokenManager) parseToken(token string, version string, suffix string) (*TokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, errors.New("invalid token")
	}
	if subtle.ConstantTimeCompare([]byte(m.sign(parts[0], suffix)), []byte(parts[1])) != 1 {
		return nil, errors.New("invalid token signature")
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var payload TokenPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return nil, err
	}
	if payload.Version != version || payload.UserID == "" || payload.Exp < time.Now().UnixMilli() {
		return nil, errors.New("token expired")
	}
	return &payload, nil
}

func (m TokenManager) sign(payload string, suffix string) string {
	secret := m.cfg.Password + ":" + m.cfg.RootPassword + ":" + m.cfg.Name + suffix
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
