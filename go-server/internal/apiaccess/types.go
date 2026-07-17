package apiaccess

import "time"

const (
	BillingModeAuto         = "auto"
	BillingModeSubscription = "subscription"
	BillingModeBalance      = "balance"

	DynamicConcurrencyWindowMinute = "minute"
	DynamicConcurrencyWindowHour   = "hour"
)

type DynamicConcurrencyConfig struct {
	Enabled     bool   `json:"enabled"`
	WindowValue int    `json:"windowValue"`
	WindowUnit  string `json:"windowUnit"`
	RequestStep int    `json:"requestStep"`
	Increment   int    `json:"increment"`
}

func DefaultDynamicConcurrencyConfig() DynamicConcurrencyConfig {
	return DynamicConcurrencyConfig{
		Enabled:     true,
		WindowValue: 1,
		WindowUnit:  DynamicConcurrencyWindowHour,
		RequestStep: 50,
		Increment:   5,
	}
}

func NormalizeDynamicConcurrencyConfig(config DynamicConcurrencyConfig) DynamicConcurrencyConfig {
	defaults := DefaultDynamicConcurrencyConfig()
	if config.WindowValue < 1 {
		config.WindowValue = defaults.WindowValue
	}
	if config.WindowUnit != DynamicConcurrencyWindowMinute && config.WindowUnit != DynamicConcurrencyWindowHour {
		config.WindowUnit = defaults.WindowUnit
	}
	if config.RequestStep < 1 {
		config.RequestStep = defaults.RequestStep
	}
	if config.Increment < 1 {
		config.Increment = defaults.Increment
	}
	return config
}

func (config DynamicConcurrencyConfig) Window() time.Duration {
	config = NormalizeDynamicConcurrencyConfig(config)
	if config.WindowUnit == DynamicConcurrencyWindowMinute {
		return time.Duration(config.WindowValue) * time.Minute
	}
	return time.Duration(config.WindowValue) * time.Hour
}

type AccessKey struct {
	ID                 string
	UserID             string
	UserEmail          *string
	Name               string
	KeyPrefix          string
	KeyHash            string
	KeyPlain           *string
	Status             string
	ConcurrencyLimit   int
	BillingMode        string
	LastUsedAt         *time.Time
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	RequestCount       int
	SuccessCount       int
	FailedCount        int
	ImageCount         int
	LastError          *string
	WindowRequestCount int
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
	ID                      string  `json:"id"`
	UserID                  string  `json:"userId"`
	UserEmail               *string `json:"userEmail,omitempty"`
	Name                    string  `json:"name"`
	KeyPrefix               string  `json:"keyPrefix"`
	Key                     *string `json:"key,omitempty"`
	Status                  string  `json:"status"`
	ConcurrencyLimit        int     `json:"concurrencyLimit"`
	BaseConcurrencyLimit    int     `json:"baseConcurrencyLimit"`
	WindowRequestCount      int     `json:"windowRequestCount"`
	HourlyRequestCount      int     `json:"hourlyRequestCount"`
	DynamicConcurrencyBonus int     `json:"dynamicConcurrencyBonus"`
	BillingMode             string  `json:"billingMode"`
	LastUsedAt              *string `json:"lastUsedAt"`
	DeletedAt               *string `json:"deletedAt,omitempty"`
	CreatedAt               string  `json:"createdAt"`
	UpdatedAt               string  `json:"updatedAt"`
	RequestCount            int     `json:"requestCount"`
	SuccessCount            int     `json:"successCount"`
	FailedCount             int     `json:"failedCount"`
	ImageCount              int     `json:"imageCount"`
	LastError               *string `json:"lastError,omitempty"`
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

type UsageStats struct {
	Total      int `json:"total"`
	Success    int `json:"success"`
	Failed     int `json:"failed"`
	ImageCount int `json:"imageCount"`
}

type UsageTrendPoint struct {
	Date    string `json:"date"`
	Total   int    `json:"total"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
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
	return ToPublicKeyWithConfig(key, DefaultDynamicConcurrencyConfig())
}

func ToPublicKeyWithConfig(key AccessKey, config DynamicConcurrencyConfig) PublicAccessKey {
	config = NormalizeDynamicConcurrencyConfig(config)
	baseConcurrency := normalizedConcurrencyLimit(key.ConcurrencyLimit)
	dynamicBonus := DynamicConcurrencyBonusWithConfig(key.WindowRequestCount, config)
	return PublicAccessKey{
		ID:                      key.ID,
		UserID:                  key.UserID,
		UserEmail:               key.UserEmail,
		Name:                    key.Name,
		KeyPrefix:               key.KeyPrefix,
		Status:                  key.Status,
		ConcurrencyLimit:        baseConcurrency + dynamicBonus,
		BaseConcurrencyLimit:    baseConcurrency,
		WindowRequestCount:      key.WindowRequestCount,
		HourlyRequestCount:      key.WindowRequestCount,
		DynamicConcurrencyBonus: dynamicBonus,
		BillingMode:             normalizedStoredBillingMode(key.BillingMode),
		LastUsedAt:              formatTime(key.LastUsedAt),
		DeletedAt:               formatTime(key.DeletedAt),
		CreatedAt:               key.CreatedAt.Format(time.RFC3339),
		UpdatedAt:               key.UpdatedAt.Format(time.RFC3339),
		RequestCount:            key.RequestCount,
		SuccessCount:            key.SuccessCount,
		FailedCount:             key.FailedCount,
		ImageCount:              key.ImageCount,
		LastError:               key.LastError,
	}
}

func DynamicConcurrencyBonus(hourlyRequestCount int) int {
	return DynamicConcurrencyBonusWithConfig(hourlyRequestCount, DefaultDynamicConcurrencyConfig())
}

func DynamicConcurrencyBonusWithConfig(requestCount int, config DynamicConcurrencyConfig) int {
	config = NormalizeDynamicConcurrencyConfig(config)
	if !config.Enabled || requestCount < config.RequestStep {
		return 0
	}
	return (requestCount / config.RequestStep) * config.Increment
}

func DynamicConcurrencyLimit(baseConcurrency int, hourlyRequestCount int) int {
	return DynamicConcurrencyLimitWithConfig(baseConcurrency, hourlyRequestCount, DefaultDynamicConcurrencyConfig())
}

func DynamicConcurrencyLimitWithConfig(baseConcurrency int, requestCount int, config DynamicConcurrencyConfig) int {
	return normalizedConcurrencyLimit(baseConcurrency) + DynamicConcurrencyBonusWithConfig(requestCount, config)
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

func normalizedStoredBillingMode(value string) string {
	switch value {
	case BillingModeSubscription, BillingModeBalance:
		return value
	default:
		return BillingModeAuto
	}
}
