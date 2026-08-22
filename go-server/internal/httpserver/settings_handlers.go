package httpserver

import (
	"context"
	"errors"
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
			if errors.Is(err, settings.ErrInvalidRechargeRate) || errors.Is(err, settings.ErrInvalidTaskTimeout) || errors.Is(err, settings.ErrInvalidDynamicConcurrency) || errors.Is(err, settings.ErrInvalidInviteSettings) || errors.Is(err, settings.ErrInvalidAdminNotification) || errors.Is(err, settings.ErrInvalidSystemLogCleanup) || errors.Is(err, settings.ErrInvalidSubscriptionAccessUser) || errors.Is(err, settings.ErrInvalidImageSafetySettings) {
				writeError(w, newAppError(http.StatusBadRequest, err.Error()))
				return
			}
			writeError(w, err)
			return
		}
		r.cacheDynamicConcurrencyConfig(dynamicConcurrencyConfigFromSettings(data))
		r.cacheTaskProcessingTimeout(settings.TaskTimeout(data))
		if r.queue != nil {
			r.queue.SetTaskProcessingTimeout(settings.TaskTimeout(data))
		}
		if _, hasMaintenanceFlag := input["upstreamMaintenanceEnabled"]; hasMaintenanceFlag {
			enabled := anyBool(data["upstreamMaintenanceEnabled"])
			if !enabled && r.queue != nil {
				_ = r.queue.TouchWaitingTasks(ctx)
			}
			if r.queue != nil {
				r.queue.SetPaused(enabled)
			}
		}
		r.invalidateImageSafetyConfig()
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
	if strings.HasSuffix(req.URL.Path, "/test-bark") {
		r.sendTestBark(w, req, values)
		return
	}
	writeError(w, newAppError(http.StatusNotFound, "测试接口不存在"))
}

func (r *Router) sendTestBark(w http.ResponseWriter, req *http.Request, values settings.Settings) {
	var input struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	_ = decodeCompatJSON(req, &input)
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = emailBrandName(anyString(values["siteName"])) + " Bark 测试通知"
	}
	body := strings.TrimSpace(input.Body)
	if body == "" {
		body = "Bark 推送配置已生效。\n发送时间：" + time.Now().Format("2006-01-02 15:04:05")
	}
	barkConfig := barkSettingsFromMap(values)
	if err := barkConfig.validateConfigured(); err != nil {
		writeError(w, err)
		return
	}
	if err := sendBarkNotification(req.Context(), barkConfig, title, body, ""); err != nil {
		writeError(w, newAppError(http.StatusBadGateway, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"sent": true}})
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
