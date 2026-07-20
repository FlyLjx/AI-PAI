package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aipi-go/internal/settings"
	"aipi-go/internal/users"
)

const upstreamStabilityEndpoint = "https://free-api.yccc.me/health/stability"

type mailBroadcastInput struct {
	Subject    string   `json:"subject"`
	Content    string   `json:"content"`
	ActionText string   `json:"actionText"`
	ActionURL  string   `json:"actionUrl"`
	Target     string   `json:"target"`
	TargetType string   `json:"targetType"`
	UserIDs    []string `json:"userIds"`
	Category   string   `json:"-"`
	ActionPath string   `json:"-"`
}

type mailBroadcastResult struct {
	Accepted bool                `json:"accepted"`
	Total    int                 `json:"total"`
	Success  int                 `json:"success"`
	Failed   int                 `json:"failed"`
	Failures []map[string]string `json:"failures"`
	Subject  string              `json:"subject"`
	Message  string              `json:"message"`
}

func (r *Router) upstreamStability(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, map[string]any{"data": fetchUpstreamStabilitySnapshot(ctx)})
}

func fetchUpstreamStabilitySnapshot(ctx context.Context) map[string]any {
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamStabilityEndpoint, nil)
	if err != nil {
		return upstreamStabilityFallback("请求创建失败："+err.Error(), 0)
	}
	upstreamReq.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		return upstreamStabilityFallback("状态接口连接失败："+err.Error(), 0)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamStabilityFallback("状态接口返回异常："+strings.TrimSpace(string(body)), resp.StatusCode)
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return upstreamStabilityFallback("状态接口返回格式异常", resp.StatusCode)
	}
	if data, ok := payload.(map[string]any); ok {
		data["source"] = upstreamStabilityEndpoint
		data["upstream_status_code"] = resp.StatusCode
		data["reachable"] = true
		data["fetched_at"] = time.Now().Format(time.RFC3339)
		return data
	}
	return map[string]any{
		"status":               "unknown",
		"source":               upstreamStabilityEndpoint,
		"upstream_status_code": resp.StatusCode,
		"reachable":            true,
		"payload":              payload,
		"fetched_at":           time.Now().Format(time.RFC3339),
	}
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
	var input mailBroadcastInput
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Minute)
	defer cancel()
	result, err := r.sendMailBroadcast(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (r *Router) sendMailBroadcast(ctx context.Context, input mailBroadcastInput) (mailBroadcastResult, error) {
	input, err := normalizeMailBroadcastInput(input)
	if err != nil {
		return mailBroadcastResult{}, err
	}
	settingValues, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return mailBroadcastResult{}, err
	}
	if input.ActionURL == "" && input.ActionPath != "" {
		input.ActionURL = notificationActionURL(anyString(settingValues["frontendUrl"]), input.ActionPath)
	}
	if err := validateMailAction(input.ActionText, input.ActionURL); err != nil {
		return mailBroadcastResult{}, err
	}
	category := defaultString(strings.TrimSpace(input.Category), "broadcast")
	if err := validateMailAudience(category, input.Content, []mailAction{{Text: input.ActionText, URL: input.ActionURL}}); err != nil {
		return mailBroadcastResult{}, err
	}
	smtpConfig := smtpSettingsFromMap(settingValues)
	if err := smtpConfig.validate(); err != nil {
		return mailBroadcastResult{}, err
	}
	recipients, err := r.mailRecipients(ctx, input.TargetType, input.UserIDs)
	if err != nil {
		return mailBroadcastResult{}, err
	}
	if len(recipients) == 0 {
		return mailBroadcastResult{}, newAppError(http.StatusBadRequest, "没有可发送的收件邮箱")
	}

	success := 0
	failures := []map[string]string{}
	for _, email := range recipients {
		if err := r.deliverMail(ctx, category, smtpConfig, email, input.Subject, input.Content, mailAction{Text: input.ActionText, URL: input.ActionURL}); err != nil {
			failures = append(failures, formatMailFailure(email, err))
			continue
		}
		success++
	}
	failed := len(failures)
	return mailBroadcastResult{
		Accepted: true,
		Total:    len(recipients),
		Success:  success,
		Failed:   failed,
		Failures: failures,
		Subject:  input.Subject,
		Message:  smtpSummary(len(recipients), success, failed),
	}, nil
}

func normalizeMailBroadcastInput(input mailBroadcastInput) (mailBroadcastInput, error) {
	input.Subject = strings.TrimSpace(input.Subject)
	input.Content = strings.TrimSpace(input.Content)
	input.ActionText = strings.TrimSpace(input.ActionText)
	input.ActionURL = strings.TrimSpace(input.ActionURL)
	input.ActionPath = strings.TrimSpace(input.ActionPath)
	input.Category = strings.TrimSpace(input.Category)
	if input.Subject == "" || input.Content == "" {
		return input, newAppError(http.StatusBadRequest, "请填写邮件标题和正文")
	}
	if len([]rune(input.Subject)) > 255 {
		return input, newAppError(http.StatusBadRequest, "邮件标题不能超过 255 个字符")
	}
	if len([]rune(input.Content)) > 50000 {
		return input, newAppError(http.StatusBadRequest, "邮件正文不能超过 50000 个字符")
	}
	input.TargetType = strings.TrimSpace(input.TargetType)
	if input.TargetType == "" {
		input.TargetType = strings.TrimSpace(input.Target)
	}
	if input.TargetType == "" {
		input.TargetType = "all"
	}
	if input.TargetType != "all" && input.TargetType != "active" && input.TargetType != "specific" {
		return input, newAppError(http.StatusBadRequest, "收件范围不正确")
	}
	seen := map[string]bool{}
	userIDs := make([]string, 0, len(input.UserIDs))
	for _, userID := range input.UserIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" || seen[userID] {
			continue
		}
		seen[userID] = true
		userIDs = append(userIDs, userID)
	}
	input.UserIDs = userIDs
	if input.TargetType == "specific" && len(input.UserIDs) == 0 {
		return input, newAppError(http.StatusBadRequest, "请选择收件用户")
	}
	if input.TargetType != "specific" {
		input.UserIDs = []string{}
	}
	if input.ActionPath == "" {
		if err := validateMailAction(input.ActionText, input.ActionURL); err != nil {
			return input, err
		}
	} else if input.ActionText == "" {
		return input, newAppError(http.StatusBadRequest, "请填写邮件按钮文字")
	}
	return input, nil
}

func validateMailAction(actionText string, actionURL string) error {
	actionText = strings.TrimSpace(actionText)
	actionURL = strings.TrimSpace(actionURL)
	if actionText == "" && actionURL == "" {
		return nil
	}
	if actionText == "" || actionURL == "" {
		return newAppError(http.StatusBadRequest, "邮件按钮文字和链接需要同时填写")
	}
	parsed, err := url.ParseRequestURI(actionURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return newAppError(http.StatusBadRequest, "邮件按钮链接必须是完整的 HTTP 或 HTTPS 地址")
	}
	return nil
}

func (r *Router) mailRecipients(ctx context.Context, targetType string, selectedIDs []string) ([]string, error) {
	items, err := users.NewRepository(r.db).FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return selectMailRecipients(items, targetType, selectedIDs), nil
}

func selectMailRecipients(items []users.User, targetType string, selectedIDs []string) []string {
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
		if user.Role != "user" {
			continue
		}
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
	return recipients
}

func anyString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
