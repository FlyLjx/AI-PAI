package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/settings"
)

func (r *Router) settings(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		if _, err := r.requireAdmin(req); err != nil {
			writeError(w, err)
			return
		}
		r.getSettings(w, req, false)
	case http.MethodPatch:
		if _, err := r.requireAdmin(req); err != nil {
			writeError(w, err)
			return
		}
		var input settings.Settings
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		data, err := settings.NewRepository(r.db).Update(ctx, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) publicSettings(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	r.getSettings(w, req, true)
}

func (r *Router) accountPoolSettings(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := settings.NewRepository(r.db)
	if req.Method == http.MethodPatch {
		var input settings.Settings
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		if _, err := repo.Update(ctx, input); err != nil {
			writeError(w, err)
			return
		}
	}
	if req.Method != http.MethodGet && req.Method != http.MethodPatch {
		writeMethodNotAllowed(w)
		return
	}
	data, err := repo.Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"accountPoolEndpoint":   data["accountPoolEndpoint"],
		"accountPoolApiKey":     data["accountPoolApiKey"],
		"accountPoolAuthHeader": data["accountPoolAuthHeader"],
	}})
}

func (r *Router) testSettingEndpoint(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 12*time.Second)
	defer cancel()
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.HasSuffix(req.URL.Path, "/test-email") {
		r.sendTestEmail(w, req, values)
		return
	}
	writeError(w, newAppError(http.StatusNotFound, "测试接口不存在"))
}

func (r *Router) sendTestEmail(w http.ResponseWriter, req *http.Request, values settings.Settings) {
	var input struct {
		Email string `json:"email"`
	}
	_ = decodeCompatJSON(req, &input)
	email := strings.TrimSpace(input.Email)
	if email == "" {
		email = strings.TrimSpace(anyString(values["emailFromAddress"]))
	}
	if email == "" {
		email = strings.TrimSpace(anyString(values["emailUser"]))
	}
	if email == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请填写测试收件邮箱"))
		return
	}
	smtpConfig := smtpSettingsFromMap(values)
	body := "这是一封来自 AI-PAI 后端的测试邮件。\n\n如果你收到这封邮件，说明 SMTP 配置可用。"
	if err := sendSMTPMail(smtpConfig, email, "AI-PAI 邮件服务测试", body); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"sent": true, "email": email}})
}

func (r *Router) getSettings(w http.ResponseWriter, req *http.Request, publicOnly bool) {
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	data, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	if publicOnly {
		data = settings.Public(data)
		w.Header().Set("Cache-Control", "public, max-age=15")
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
