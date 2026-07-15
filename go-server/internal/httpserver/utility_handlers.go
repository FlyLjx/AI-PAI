package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/settings"
	"aipi-go/internal/users"
)

const upstreamStabilityEndpoint = "https://free-api.yccc.me/health/stability"

func (r *Router) upstreamStability(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamStabilityEndpoint, nil)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": upstreamStabilityFallback("请求创建失败："+err.Error(), 0)})
		return
	}
	upstreamReq.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": upstreamStabilityFallback("状态接口连接失败："+err.Error(), 0)})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusOK, map[string]any{"data": upstreamStabilityFallback("状态接口返回异常："+strings.TrimSpace(string(body)), resp.StatusCode)})
		return
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": upstreamStabilityFallback("状态接口返回格式异常", resp.StatusCode)})
		return
	}
	if data, ok := payload.(map[string]any); ok {
		data["source"] = upstreamStabilityEndpoint
		data["upstream_status_code"] = resp.StatusCode
		data["reachable"] = true
		data["fetched_at"] = time.Now().Format(time.RFC3339)
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"status":               "unknown",
		"source":               upstreamStabilityEndpoint,
		"upstream_status_code": resp.StatusCode,
		"reachable":            true,
		"payload":              payload,
		"fetched_at":           time.Now().Format(time.RFC3339),
	}})
}

func upstreamStabilityFallback(message string, upstreamStatusCode int) map[string]any {
	now := time.Now()
	return map[string]any{
		"window_seconds":       60,
		"window_start":         now.Add(-60 * time.Second).Format(time.RFC3339),
		"window_end":           now.Format(time.RFC3339),
		"generated_at":         now.Format(time.RFC3339),
		"fetched_at":           now.Format(time.RFC3339),
		"total":                0,
		"success":              0,
		"failed":               0,
		"stability_percent":    0,
		"status":               "unreachable",
		"series":               []any{},
		"reachable":            false,
		"source":               upstreamStabilityEndpoint,
		"upstream_status_code": upstreamStatusCode,
		"error":                strings.TrimSpace(message),
	}
}

func (r *Router) accountPoolAccounts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	endpoint := strings.TrimSpace(anyString(values["accountPoolEndpoint"]))
	if endpoint == "" {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}, "message": "未配置号池地址"})
		return
	}
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	apiKey := strings.TrimSpace(anyString(values["accountPoolApiKey"]))
	header := strings.TrimSpace(anyString(values["accountPoolAuthHeader"]))
	if header == "" {
		header = "Authorization"
	}
	if apiKey != "" {
		upstreamReq.Header.Set(header, apiKey)
		if strings.EqualFold(header, "Authorization") && !strings.HasPrefix(strings.ToLower(apiKey), "bearer ") {
			upstreamReq.Header.Set(header, "Bearer "+apiKey)
		}
	}
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		writeError(w, newAppError(http.StatusBadGateway, "号池接口连接失败："+err.Error()))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, newAppError(resp.StatusCode, "号池接口调用失败："+string(body)))
		return
	}
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": payload})
}

func (r *Router) mailBroadcast(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Subject    string   `json:"subject"`
		Content    string   `json:"content"`
		ActionText string   `json:"actionText"`
		ActionURL  string   `json:"actionUrl"`
		Target     string   `json:"target"`
		TargetType string   `json:"targetType"`
		UserIDs    []string `json:"userIds"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	input.Subject = strings.TrimSpace(input.Subject)
	input.Content = strings.TrimSpace(input.Content)
	if input.Subject == "" || input.Content == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请填写邮件标题和正文"))
		return
	}
	targetType := strings.TrimSpace(input.TargetType)
	if targetType == "" {
		targetType = strings.TrimSpace(input.Target)
	}
	if targetType == "" {
		targetType = "all"
	}
	if targetType != "all" && targetType != "active" && targetType != "specific" {
		writeError(w, newAppError(http.StatusBadRequest, "收件范围不正确"))
		return
	}
	if targetType == "specific" && len(input.UserIDs) == 0 {
		writeError(w, newAppError(http.StatusBadRequest, "请选择收件用户"))
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Minute)
	defer cancel()
	settingValues, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	smtpConfig := smtpSettingsFromMap(settingValues)
	if err := smtpConfig.validate(); err != nil {
		writeError(w, err)
		return
	}
	recipients, err := r.mailRecipients(ctx, targetType, input.UserIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(recipients) == 0 {
		writeError(w, newAppError(http.StatusBadRequest, "没有可发送的收件邮箱"))
		return
	}

	success := 0
	failures := []map[string]string{}
	for _, email := range recipients {
		if err := sendSMTPMail(smtpConfig, email, input.Subject, input.Content, mailAction{Text: input.ActionText, URL: input.ActionURL}); err != nil {
			failures = append(failures, formatMailFailure(email, err))
			continue
		}
		success++
	}
	failed := len(failures)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"accepted": true,
		"total":    len(recipients),
		"success":  success,
		"failed":   failed,
		"failures": failures,
		"subject":  input.Subject,
		"message":  smtpSummary(len(recipients), success, failed),
	}})
}

func (r *Router) mailRecipients(ctx context.Context, targetType string, selectedIDs []string) ([]string, error) {
	items, err := users.NewRepository(r.db).FindAll(ctx)
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, id := range selectedIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			selected[id] = true
		}
	}
	seen := map[string]bool{}
	recipients := []string{}
	for _, user := range items {
		email := strings.TrimSpace(user.Email)
		if email == "" {
			continue
		}
		switch targetType {
		case "active":
			if user.Status != "active" {
				continue
			}
		case "specific":
			if !selected[user.ID] {
				continue
			}
		}
		key := strings.ToLower(email)
		if seen[key] {
			continue
		}
		seen[key] = true
		recipients = append(recipients, email)
	}
	return recipients, nil
}

func anyString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
