package apikeys

import "time"

type APIKey struct {
	ID         string
	UserID     string
	UserEmail  *string
	Name       string
	KeyPrefix  string
	KeyHash    string
	KeyPlain   *string
	Status     string
	LastUsedAt *time.Time
	DeletedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type PublicAPIKey struct {
	ID         string  `json:"id"`
	UserID     string  `json:"userId"`
	UserEmail  *string `json:"userEmail,omitempty"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"keyPrefix"`
	KeyPlain   *string `json:"keyPlain,omitempty"`
	Key        *string `json:"key,omitempty"`
	Status     string  `json:"status"`
	LastUsedAt *string `json:"lastUsedAt"`
	DeletedAt  *string `json:"deletedAt"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

func ToPublic(key APIKey) PublicAPIKey {
	return PublicAPIKey{
		ID:         key.ID,
		UserID:     key.UserID,
		UserEmail:  key.UserEmail,
		Name:       key.Name,
		KeyPrefix:  key.KeyPrefix,
		KeyPlain:   key.KeyPlain,
		Status:     key.Status,
		LastUsedAt: formatTime(key.LastUsedAt),
		DeletedAt:  formatTime(key.DeletedAt),
		CreatedAt:  key.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  key.UpdatedAt.Format(time.RFC3339),
	}
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.Format(time.RFC3339)
	return &text
}
