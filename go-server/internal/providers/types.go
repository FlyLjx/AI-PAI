package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type Provider struct {
	ID         string
	Name       string
	Type       string
	Capability string
	BaseURL    string
	APIKey     string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type PublicProvider struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Capability string `json:"capability"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

func ToPublic(provider Provider) PublicProvider {
	return PublicProvider{
		ID:         provider.ID,
		Name:       provider.Name,
		Type:       provider.Type,
		Capability: provider.Capability,
		BaseURL:    provider.BaseURL,
		APIKey:     provider.APIKey,
		Status:     provider.Status,
		CreatedAt:  provider.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  provider.UpdatedAt.Format(time.RFC3339),
	}
}

func NormalizeAPIKey(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("Bearer ") && strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		return strings.TrimSpace(value[len("Bearer "):])
	}
	return value
}

func AuthorizationHeader(apiKey string) string {
	return "Bearer " + NormalizeAPIKey(apiKey)
}

func APIKeyDiagnostics(value string) map[string]any {
	normalized := NormalizeAPIKey(value)
	hash := sha256.Sum256([]byte(normalized))
	fingerprint := hex.EncodeToString(hash[:])[:12]
	return map[string]any{
		"hasBearerPrefix": hasBearerPrefix(value),
		"keyLength":       len(normalized),
		"keyFingerprint":  fingerprint,
	}
}

func hasBearerPrefix(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) >= len("Bearer ") && strings.EqualFold(value[:len("Bearer ")], "Bearer ")
}
