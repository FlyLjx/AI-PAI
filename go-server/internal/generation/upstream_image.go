package generation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/providers"
)

const (
	upstreamImageMaxAttempts      = 3
	maxUpstreamImageResponseBytes = 120 * 1024 * 1024
	maxInlineResultImageBytes     = 80 * 1024 * 1024
)

func (s *Service) callImageJSON(ctx context.Context, input ImageRequest, attempt int) (any, error) {
	body := map[string]any{
		"model":           input.Model.ModelName,
		"prompt":          buildUpstreamPrompt(input.Prompt, input.Size, input.SizeTier, input.Model.AppendSizeToPrompt, input.TransparentBackground),
		"size":            input.Size,
		"n":               input.Quantity,
		"quality":         "high",
		"response_format": normalizeUpstreamImageResponseFormat(input.ResponseFormat),
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

	responseBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamImageResponseBytes+1))
	if len(responseBytes) > maxUpstreamImageResponseBytes {
		return nil, errors.New("上游图片响应过大")
	}
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

func normalizeUpstreamImageResponseFormat(format string) string {
	if wantsBase64ImageResponse(format) {
		return "b64_json"
	}
	return "url"
}

func wantsBase64ImageResponse(format string) bool {
	format = strings.ToLower(strings.TrimSpace(format))
	return format == "b64_json" || format == "base64" || format == "b64"
}

func EnsureBase64Images(ctx context.Context, images []ExtractedImage) ([]ExtractedImage, error) {
	result := make([]ExtractedImage, 0, len(images))
	for index, image := range images {
		if strings.TrimSpace(image.B64) != "" {
			image.Type = "b64_json"
			image.B64 = compactBase64(image.B64)
			image.URL = ""
			result = append(result, image)
			continue
		}
		if converted, ok := base64ImageFromDataURL(image.URL); ok {
			result = append(result, converted)
			continue
		}
		if strings.TrimSpace(image.URL) == "" {
			return nil, fmt.Errorf("第 %d 张图片缺少可转为 base64 的结果", index+1)
		}
		converted, err := fetchImageAsBase64(ctx, image.URL)
		if err != nil {
			return nil, fmt.Errorf("第 %d 张图片转为 base64 失败：%w", index+1, err)
		}
		result = append(result, converted)
	}
	return result, nil
}

func fetchImageAsBase64(ctx context.Context, imageURL string) (ExtractedImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(imageURL), nil)
	if err != nil {
		return ExtractedImage{}, err
	}
	req.Header.Set("User-Agent", "AI-PAI image base64")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ExtractedImage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ExtractedImage{}, fmt.Errorf("图片下载失败：HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInlineResultImageBytes+1))
	if err != nil {
		return ExtractedImage{}, err
	}
	if len(body) == 0 || len(body) > maxInlineResultImageBytes {
		return ExtractedImage{}, errors.New("图片内容为空或过大")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if !strings.HasPrefix(contentType, "image/") {
		contentType = strings.ToLower(http.DetectContentType(body))
	}
	if !strings.HasPrefix(contentType, "image/") {
		return ExtractedImage{}, errors.New("下载内容不是图片")
	}
	return ExtractedImage{Type: "b64_json", B64: base64.StdEncoding.EncodeToString(body)}, nil
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
