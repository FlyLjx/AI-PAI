package apiaccess

import "time"

type AccessKey struct {
	ID               string
	UserID           string
	UserEmail        *string
	Name             string
	KeyPrefix        string
	KeyHash          string
	KeyPlain         *string
	Status           string
	ConcurrencyLimit int
	LastUsedAt       *time.Time
	DeletedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RequestCount     int
	SuccessCount     int
	FailedCount      int
	ImageCount       int
	LastError        *string
}

type UsageLog struct {
	ID             string
	UserID         string
	UserEmail      *string
	APIKeyID       string
	KeyName        *string
	KeyPrefix      *string
	TaskID         *string
	Endpoint       string
	Model          string
	Prompt         string
	Size           string
	Quality        string
	Quantity       int
	ImageCount     int
	ResponseFormat string
	Status         string
	ErrorMessage   *string
	CreatedAt      time.Time
	FinishedAt     *time.Time
}

type PublicAccessKey struct {
	ID               string  `json:"id"`
	UserID           string  `json:"userId"`
	UserEmail        *string `json:"userEmail,omitempty"`
	Name             string  `json:"name"`
	KeyPrefix        string  `json:"keyPrefix"`
	KeyPlain         *string `json:"keyPlain,omitempty"`
	Key              *string `json:"key,omitempty"`
	Status           string  `json:"status"`
	ConcurrencyLimit int     `json:"concurrencyLimit"`
	LastUsedAt       *string `json:"lastUsedAt"`
	DeletedAt        *string `json:"deletedAt,omitempty"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	RequestCount     int     `json:"requestCount"`
	SuccessCount     int     `json:"successCount"`
	FailedCount      int     `json:"failedCount"`
	ImageCount       int     `json:"imageCount"`
	LastError        *string `json:"lastError,omitempty"`
}

type PublicUsageLog struct {
	ID             string  `json:"id"`
	UserID         string  `json:"userId"`
	UserEmail      *string `json:"userEmail,omitempty"`
	APIKeyID       string  `json:"apiKeyId"`
	KeyName        *string `json:"keyName,omitempty"`
	KeyPrefix      *string `json:"keyPrefix,omitempty"`
	TaskID         *string `json:"taskId,omitempty"`
	Endpoint       string  `json:"endpoint"`
	Model          string  `json:"model"`
	Prompt         string  `json:"prompt"`
	Size           string  `json:"size"`
	Quality        string  `json:"quality"`
	Quantity       int     `json:"quantity"`
	ImageCount     int     `json:"imageCount"`
	ResponseFormat string  `json:"responseFormat"`
	Status         string  `json:"status"`
	ErrorMessage   *string `json:"errorMessage,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	FinishedAt     *string `json:"finishedAt"`
}

type ListLogsInput struct {
	UserID   string
	APIKeyID string
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

type AdminStats struct {
	TotalKeys       int `json:"totalKeys"`
	ActiveKeys      int `json:"activeKeys"`
	TodayRequests   int `json:"todayRequests"`
	TodaySuccess    int `json:"todaySuccess"`
	TodayFailed     int `json:"todayFailed"`
	TodayImageCount int `json:"todayImageCount"`
}

func ToPublicKey(key AccessKey) PublicAccessKey {
	return PublicAccessKey{
		ID:               key.ID,
		UserID:           key.UserID,
		UserEmail:        key.UserEmail,
		Name:             key.Name,
		KeyPrefix:        key.KeyPrefix,
		KeyPlain:         key.KeyPlain,
		Status:           key.Status,
		ConcurrencyLimit: normalizedConcurrencyLimit(key.ConcurrencyLimit),
		LastUsedAt:       formatTime(key.LastUsedAt),
		DeletedAt:        formatTime(key.DeletedAt),
		CreatedAt:        key.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        key.UpdatedAt.Format(time.RFC3339),
		RequestCount:     key.RequestCount,
		SuccessCount:     key.SuccessCount,
		FailedCount:      key.FailedCount,
		ImageCount:       key.ImageCount,
		LastError:        key.LastError,
	}
}

func ToPublicLog(log UsageLog) PublicUsageLog {
	return PublicUsageLog{
		ID:             log.ID,
		UserID:         log.UserID,
		UserEmail:      log.UserEmail,
		APIKeyID:       log.APIKeyID,
		KeyName:        log.KeyName,
		KeyPrefix:      log.KeyPrefix,
		TaskID:         log.TaskID,
		Endpoint:       log.Endpoint,
		Model:          log.Model,
		Prompt:         log.Prompt,
		Size:           log.Size,
		Quality:        log.Quality,
		Quantity:       log.Quantity,
		ImageCount:     log.ImageCount,
		ResponseFormat: log.ResponseFormat,
		Status:         log.Status,
		ErrorMessage:   log.ErrorMessage,
		CreatedAt:      log.CreatedAt.Format(time.RFC3339),
		FinishedAt:     formatTime(log.FinishedAt),
	}
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	text := value.Format(time.RFC3339)
	return &text
}

func normalizedConcurrencyLimit(value int) int {
	if value < 1 {
		return 10
	}
	return value
}
