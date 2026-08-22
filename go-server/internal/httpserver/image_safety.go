package httpserver

import (
	"context"
	"net/http"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/apierrors"
	"aipi-go/internal/settings"
)

const imageSafetyConfigCacheTTL = 5 * time.Second

func (r *Router) imageSafetyConfig(ctx context.Context) (settings.ImageSafetyConfig, error) {
	now := time.Now()
	r.imageSafetyMu.RLock()
	if !r.imageSafetyCacheAt.IsZero() && now.Sub(r.imageSafetyCacheAt) < imageSafetyConfigCacheTTL {
		config := r.imageSafetyCache
		r.imageSafetyMu.RUnlock()
		return config, nil
	}
	r.imageSafetyMu.RUnlock()

	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return settings.ImageSafetyConfig{}, err
	}
	config := settings.ImageSafetyConfigFromSettings(values)
	r.imageSafetyMu.Lock()
	r.imageSafetyCache = config
	r.imageSafetyCacheAt = now
	r.imageSafetyMu.Unlock()
	return config, nil
}

func (r *Router) invalidateImageSafetyConfig() {
	r.imageSafetyMu.Lock()
	r.imageSafetyCacheAt = time.Time{}
	r.imageSafetyMu.Unlock()
}

func (r *Router) rejectImageSafetyRequest(
	w http.ResponseWriter,
	req *http.Request,
	auth *apiaccess.Authenticated,
	model string,
	prompt string,
	size string,
	quality string,
	quantity int,
	responseFormat string,
	requestParams map[string]any,
) {
	details := apierrors.Details{
		StatusCode:   http.StatusBadRequest,
		Message:      "提示词包含不允许的内容，已拦截本次生图请求",
		Type:         "invalid_request_error",
		Code:         "content_policy_violation",
		Category:     "content_policy",
		Retryable:    false,
		RetryableSet: true,
		Action:       "check_prompt",
		Hint:         "请修改提示词后重新提交",
	}
	apierrors.Normalize(&details)

	accessIP, accessHost := requestAccessMetadata(req)
	errorMessage := details.Message
	logCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, _ = apiaccess.NewRepository(r.db).CreateLog(logCtx, apiaccess.UsageLog{
		ID:                 newID(),
		UserID:             auth.User.ID,
		APIKeyID:           auth.APIKey.ID,
		AccessIP:           accessIP,
		AccessHost:         accessHost,
		Endpoint:           req.URL.Path,
		Model:              model,
		Prompt:             prompt,
		Size:               size,
		Quality:            quality,
		Quantity:           quantity,
		ResponseFormat:     responseFormat,
		RequestParams:      requestParams,
		Status:             "failed",
		ErrorMessage:       &errorMessage,
		ResponseStatusCode: details.StatusCode,
		ErrorCode:          &details.Code,
		ErrorDetails:       &details,
		FinishedAt:         timePointer(time.Now()),
	})
	cancel()
	writeOpenAIErrorDetails(w, details)
}

func timePointer(value time.Time) *time.Time {
	return &value
}
