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
	if errors.Is(err, sql.ErrNoRows) || model == nil {
		writeOpenAIError(w, http.StatusNotFound, "模型不存在或已禁用", "invalid_request_error")
		return
	}
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
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
	if err := r.requireGenerationSubscription(ctx, auth.User.ID, *model); err != nil {
		status := http.StatusInternalServerError
		var appErr appError
		if errors.As(err, &appErr) {
			status = appErr.status
		}
		writeOpenAIError(w, status, err.Error(), "insufficient_quota")
		return
	}

	body["model"] = model.ModelName
	stream := boolValue(body["stream"])
	if stream {
		r.forwardOpenAITextStream(w, req, *provider, body, upstreamPath)
		return
	}
	body["stream"] = false
	result, contentType, err := postOpenAIJSON(req.Context(), *provider, upstreamPath, body)
	if err != nil {
		writeOpenAIError(w, upstreamStatus(err), err.Error(), "api_error")
		return
	}
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
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
		writeOpenAIError(w, http.StatusBadGateway, "上游接口连接失败："+err.Error(), "api_error")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		writeOpenAIError(w, resp.StatusCode, upstreamErrorMessage(bodyBytes, resp.StatusCode), "api_error")
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

func postOpenAIJSON(ctx context.Context, provider providers.Provider, upstreamPath string, body map[string]any) ([]byte, string, error) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIProxyEndpoint(provider, upstreamPath), bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", providers.AuthorizationHeader(provider.APIKey))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("上游接口连接失败：%w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", upstreamHTTPError{status: resp.StatusCode, message: upstreamErrorMessage(bodyBytes, resp.StatusCode)}
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json; charset=utf-8"
	}
	return bodyBytes, contentType, nil
}

func openAIProxyEndpoint(provider providers.Provider, upstreamPath string) string {
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}
	return baseURL + "/" + strings.TrimLeft(upstreamPath, "/")
}

type upstreamHTTPError struct {
	status  int
	message string
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
