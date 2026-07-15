package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/providers"
)

const upstreamImageMaxAttempts = 3

func (s *Service) callImageJSON(ctx context.Context, input ImageRequest, attempt int) (any, error) {
	body := map[string]any{
		"model":           input.Model.ModelName,
		"prompt":          buildUpstreamPrompt(input.Prompt, input.Size, input.SizeTier, input.Model.AppendSizeToPrompt, input.TransparentBackground),
		"size":            input.Size,
		"n":               input.Quantity,
		"quality":         "high",
		"response_format": "url",
	}
	if input.OutputFormat != "" {
		body["output_format"] = input.OutputFormat
	}
	if input.TransparentBackground {
		body["background"] = "transparent"
	}
	if len(input.ReferenceImageURLs) > 0 {
		items := make([]map[string]string, 0, len(input.ReferenceImageURLs))
		urls := make([]string, 0, len(input.ReferenceImageURLs))
		base64Images := make([]string, 0, len(input.ReferenceImageURLs))
		for _, url := range input.ReferenceImageURLs {
			if strings.TrimSpace(url) == "" {
				continue
			}
			cleanURL := strings.TrimSpace(url)
			upstreamURL := cleanURL
			if input.Operation == "edit" {
				inlineImage, err := inlineEditImageData(ctx, cleanURL)
				if err != nil {
					return nil, err
				}
				upstreamURL = inlineImage.DataURL
				base64Images = append(base64Images, inlineImage.Base64)
			}
			urls = append(urls, upstreamURL)
			items = append(items, map[string]string{"url": upstreamURL})
		}
		if len(items) > 0 {
			body["referenceImages"] = items
			body["referenceImage"] = map[string]any{
				"count": len(items),
				"items": items,
			}
			if input.Operation == "edit" {
				body["image_url"] = urls[0]
				if len(base64Images) > 0 {
					body["image"] = base64Images[0]
				} else {
					body["image"] = urls[0]
				}
				body["image_urls"] = urls
			}
		}
	}
	if strings.TrimSpace(input.MaskImageURL) != "" {
		maskURL := strings.TrimSpace(input.MaskImageURL)
		if input.Operation == "edit" {
			inlineMask, err := inlineEditImageData(ctx, maskURL)
			if err != nil {
				return nil, err
			}
			maskURL = inlineMask.DataURL
			body["mask"] = inlineMask.Base64
		}
		body["maskImage"] = map[string]string{"url": maskURL}
	}
	payload, _ := json.Marshal(body)
	endpoint := imageEndpoint(input.Provider, input.Operation)

	var lastErr error
	for requestAttempt := 1; requestAttempt <= upstreamImageMaxAttempts; requestAttempt++ {
		result, err := s.callImageJSONOnce(ctx, input, attempt, requestAttempt, endpoint, payload)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if requestAttempt >= upstreamImageMaxAttempts || !isRetryableImageUpstreamError(err) {
			return nil, err
		}
		if s.logger != nil {
			s.logger.Warn("generation upstream image retry",
				"taskId", input.TaskID,
				"providerId", input.Provider.ID,
				"endpoint", endpoint,
				"attempt", attempt,
				"requestAttempt", requestAttempt,
				"error", err.Error(),
			)
		}
		if err := sleepImageRetry(ctx, requestAttempt); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (s *Service) callImageJSONOnce(ctx context.Context, input ImageRequest, attempt int, requestAttempt int, endpoint string, payload []byte) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", providers.AuthorizationHeader(input.Provider.APIKey))
	req.Header.Set("Content-Type", "application/json")

	startedAt := time.Now()
	if s.logger != nil {
		s.logger.Info("generation upstream image request",
			"taskId", input.TaskID,
			"providerId", input.Provider.ID,
			"endpoint", endpoint,
			"attempt", attempt,
			"requestAttempt", requestAttempt,
			"requestedQuantity", input.Quantity,
		)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上游中转服务连接中断：%w", err)
	}
	defer resp.Body.Close()

	responseBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	var responseJSON any
	_ = json.Unmarshal(responseBytes, &responseJSON)
	errorMessage := cleanUpstreamError(responseJSON, string(responseBytes))
	if s.logger != nil {
		s.logger.Info("generation upstream image response",
			"taskId", input.TaskID,
			"providerId", input.Provider.ID,
			"endpoint", endpoint,
			"attempt", attempt,
			"requestAttempt", requestAttempt,
			"status", resp.StatusCode,
			"durationMs", time.Since(startedAt).Milliseconds(),
			"requestedQuantity", input.Quantity,
			"imageCount", len(ExtractImages(responseJSON)),
			"errorMessage", trimLong(errorMessage, 300),
			"auth", providers.APIKeyDiagnostics(input.Provider.APIKey),
		)
	}
	if isHTMLResponse(resp, responseBytes) {
		return nil, htmlImageUpstreamError(resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := errorMessage
		if message == "" {
			message = fmt.Sprintf("上游接口调用失败：%d", resp.StatusCode)
		}
		return nil, imageUpstreamHTTPError{status: resp.StatusCode, message: message}
	}
	if len(ExtractImages(responseJSON)) == 0 {
		message := errorMessage
		if message != "" {
			return nil, errors.New("上游接口未返回图片结果：" + message)
		}
		return nil, errors.New("上游接口未返回图片结果")
	}
	return NormalizeImageResultForProvider(responseJSON, input.Provider), nil
}

type imageUpstreamHTTPError struct {
	status  int
	message string
}

func (e imageUpstreamHTTPError) Error() string {
	return e.message
}

func htmlImageUpstreamError(status int) error {
	switch status {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return imageUpstreamHTTPError{
			status:  status,
			message: fmt.Sprintf("上游服务网关超时或不可用（HTTP %d），请稍后重试或切换接口", status),
		}
	default:
		if status < 200 || status >= 300 {
			return imageUpstreamHTTPError{
				status:  status,
				message: fmt.Sprintf("上游接口返回了 HTML 错误页（HTTP %d），请检查接口服务状态或 Base URL", status),
			}
		}
		return errors.New("上游返回了网页 HTML，不是图片接口 JSON，请检查接口 Base URL")
	}
}

func isRetryableImageUpstreamError(err error) bool {
	var upstreamErr imageUpstreamHTTPError
	if errors.As(err, &upstreamErr) {
		return isRetryableImageStatus(upstreamErr.status) || isTransientImageMessage(upstreamErr.message)
	}
	return isTransientImageMessage(err.Error())
}

func isRetryableImageStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isTransientImageMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	for _, keyword := range []string{
		"curl: (56)",
		"connection closed abruptly",
		"connection reset",
		"unexpected eof",
		"eof",
		"timeout",
		"temporarily unavailable",
		"server closed idle connection",
	} {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

func sleepImageRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt) * 600 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func imageEndpoint(provider providers.Provider, operation string) string {
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	if provider.Type == "custom" || provider.Type == "newapi" {
		if !strings.HasSuffix(baseURL, "/v1") {
			baseURL += "/v1"
		}
	}
	if operation == "edit" {
		return baseURL + "/images/edits"
	}
	return baseURL + "/images/generations"
}
