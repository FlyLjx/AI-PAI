package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/apierrors"
	"aipi-go/internal/models"
	"aipi-go/internal/providers"
)

func (r *Router) compatChatCompletions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	auth, err := r.authenticateAPIKey(req)
	if err != nil {
		writeCompatAuthError(w, err)
		return
	}
	var body map[string]any
	if err := decodeCompatJSON(req, &body); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "请求参数不正确", "invalid_request_error")
		return
	}
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		writeOpenAIError(w, http.StatusBadRequest, "缺少模型", "invalid_request_error")
		return
	}
	if _, ok := body["messages"].([]any); !ok {
		writeOpenAIError(w, http.StatusBadRequest, "缺少 messages", "invalid_request_error")
		return
	}
	r.forwardOpenAIText(w, req, auth, body, "chat/completions")
}

func compatChatPrompt(value any) string {
	messages, ok := value.([]any)
	if !ok {
		return ""
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message, ok := messages[index].(map[string]any)
		if !ok || !strings.EqualFold(strings.TrimSpace(stringValue(message["role"])), "user") {
			continue
		}
		if content := compatChatContentText(message["content"]); content != "" {
			return content
		}
	}
	for index := len(messages) - 1; index >= 0; index-- {
		if message, ok := messages[index].(map[string]any); ok {
			if content := compatChatContentText(message["content"]); content != "" {
				return content
			}
		}
	}
	return ""
}

func compatChatContentText(value any) string {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case []any:
		parts := make([]string, 0, len(item))
		for _, child := range item {
			if text := compatChatContentText(child); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text := strings.TrimSpace(stringValue(item["text"])); text != "" {
			return text
		}
		return compatChatContentText(item["content"])
	default:
		return ""
	}
}

func compatChatImageCount(value any) int {
	switch item := value.(type) {
	case float64:
		integer := int(item)
		if item >= 1 && item <= 10 && float64(integer) == item {
			return integer
		}
	case int:
		if item >= 1 && item <= 10 {
			return item
		}
	}
	return 1
}

func (r *Router) compatResponses(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	auth, err := r.authenticateAPIKey(req)
	if err != nil {
		writeCompatAuthError(w, err)
		return
	}
	var body map[string]any
	if err := decodeCompatJSON(req, &body); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "请求参数不正确", "invalid_request_error")
		return
	}
	if strings.TrimSpace(stringValue(body["model"])) == "" {
		writeOpenAIError(w, http.StatusBadRequest, "缺少模型", "invalid_request_error")
		return
	}
	if _, ok := body["input"]; !ok {
		writeOpenAIError(w, http.StatusBadRequest, "缺少 input", "invalid_request_error")
		return
	}
	r.forwardOpenAIText(w, req, auth, body, "responses")
}

func (r *Router) forwardOpenAIText(w http.ResponseWriter, req *http.Request, auth *apiaccess.Authenticated, body map[string]any, upstreamPath string) {
	ctx, cancel := context.WithTimeout(req.Context(), 20*time.Second)
	defer cancel()
	modelName := strings.TrimSpace(stringValue(body["model"]))
	model, err := models.NewRepository(r.db).FindActiveByNameOrDisplayName(ctx, modelName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeOpenAIError(w, http.StatusNotFound, "模型不存在或已禁用", "invalid_request_error")
			return
		}
		if errors.Is(err, models.ErrAmbiguousModelName) {
			writeOpenAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
			return
		}
		writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}
	if model == nil {
		writeOpenAIError(w, http.StatusNotFound, "模型不存在或已禁用", "invalid_request_error")
		return
	}
	if model.Capability == "chat_image" {
		moderationConfig, moderationErr := r.imageSafetyConfig(ctx)
		if moderationErr != nil {
			writeOpenAIError(w, http.StatusInternalServerError, "内容检测配置读取失败", "api_error")
			return
		}
		prompt := compatTextRequestPrompt(upstreamPath, body)
		if moderationConfig.Match(prompt) != "" {
			r.rejectImageSafetyRequest(w, req, auth, modelName, prompt, stringValue(body["size"]), stringValue(body["quality"]), compatChatImageCount(body["n"]), stringValue(body["response_format"]), body)
			return
		}
	}
	provider, err := providers.NewRepository(r.db).FindByID(ctx, model.ProviderID)
	if errors.Is(err, sql.ErrNoRows) || provider == nil || provider.Status != "active" {
		writeOpenAIError(w, http.StatusNotFound, "接口配置不存在或已禁用", "invalid_request_error")
		return
	}
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}
	stream := boolValue(body["stream"])
	if stream {
		r.forwardOpenAITextStream(w, req, *provider, body, upstreamPath)
		return
	}
	result, contentType, status, err := postOpenAIJSON(req.Context(), *provider, upstreamPath, body)
	if err != nil {
		writeOpenAIError(w, http.StatusBadGateway, err.Error(), "api_error")
		return
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		writeUpstreamResponse(w, status, contentType, result)
		return
	}
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(result)
}

func compatTextRequestPrompt(upstreamPath string, body map[string]any) string {
	switch strings.Trim(strings.ToLower(upstreamPath), "/") {
	case "chat/completions":
		return compatChatPrompt(body["messages"])
	case "responses":
		return compatChatContentText(body["input"])
	default:
		return ""
	}
}

func (r *Router) forwardOpenAITextStream(w http.ResponseWriter, req *http.Request, provider providers.Provider, body map[string]any, upstreamPath string) {
	body["stream"] = true
	payload, _ := json.Marshal(body)
	upstreamReq, err := http.NewRequestWithContext(req.Context(), http.MethodPost, openAIProxyEndpoint(provider, upstreamPath), bytes.NewReader(payload))
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}
	upstreamReq.Header.Set("Authorization", providers.AuthorizationHeader(provider.APIKey))
	upstreamReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		writeOpenAIErrorDetails(w, apierrors.Parse(http.StatusBadGateway, nil, []byte("上游接口连接失败："+err.Error())))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		writeUpstreamResponse(w, resp.StatusCode, resp.Header.Get("Content-Type"), bodyBytes)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(w, resp.Body)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	if copyErr != nil {
		return
	}
}

func postOpenAIJSON(ctx context.Context, provider providers.Provider, upstreamPath string, body map[string]any) ([]byte, string, int, error) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIProxyEndpoint(provider, upstreamPath), bytes.NewReader(payload))
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("Authorization", providers.AuthorizationHeader(provider.APIKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	return bodyBytes, contentType, resp.StatusCode, nil
}

func writeUpstreamResponse(w http.ResponseWriter, status int, contentType string, body []byte) {
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func openAIProxyEndpoint(provider providers.Provider, upstreamPath string) string {
	baseURL := provider.EffectiveBaseURL()
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}
	return baseURL + "/" + strings.TrimLeft(upstreamPath, "/")
}

type upstreamHTTPError struct {
	status  int
	message string
	details apierrors.Details
}

func (e upstreamHTTPError) Error() string {
	return e.message
}

func upstreamStatus(err error) int {
	var upstreamErr upstreamHTTPError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.status
	}
	return http.StatusBadGateway
}

func upstreamErrorDetails(err error) apierrors.Details {
	var upstreamErr upstreamHTTPError
	if errors.As(err, &upstreamErr) {
		details := upstreamErr.details
		if details.StatusCode == 0 {
			details.StatusCode = upstreamErr.status
		}
		if strings.TrimSpace(details.Message) == "" {
			details.Message = upstreamErr.message
		}
		apierrors.Normalize(&details)
		return details
	}
	return apierrors.Parse(http.StatusBadGateway, nil, []byte(err.Error()))
}

func upstreamErrorMessage(body []byte, status int) string {
	var payload any
	if err := json.Unmarshal(body, &payload); err == nil {
		if message := findErrorMessage(payload); message != "" {
			return message
		}
	}
	text := strings.TrimSpace(string(body))
	if text != "" && !strings.HasPrefix(strings.ToLower(text), "<!doctype html") && !strings.HasPrefix(strings.ToLower(text), "<html") {
		return text
	}
	return fmt.Sprintf("上游接口调用失败：HTTP %d", status)
}

func decodeJSONPayload(body []byte) any {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload
}

func findErrorMessage(value any) string {
	payload, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if message := stringValue(payload["message"]); message != "" {
		return message
	}
	if errorPayload, ok := payload["error"].(map[string]any); ok {
		if message := stringValue(errorPayload["message"]); message != "" {
			return message
		}
	}
	return ""
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func boolValue(value any) bool {
	if flag, ok := value.(bool); ok {
		return flag
	}
	return false
}
