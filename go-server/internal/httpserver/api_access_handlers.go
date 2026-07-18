package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/users"
)

func (r *Router) userAPIAccessKeys(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.listUserAPIAccessKeys(w, req)
	case http.MethodPost:
		r.createUserAPIAccessKey(w, req)
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) userAPIAccessKeyByID(w http.ResponseWriter, req *http.Request) {
	path := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/api-access/keys/"), "/")
	if strings.HasSuffix(path, "/reveal") {
		id := strings.TrimSuffix(path, "/reveal")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, newAppError(http.StatusNotFound, "API Key 不存在"))
			return
		}
		if req.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		r.revealUserAPIAccessKey(w, req, id)
		return
	}
	id := path
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "API Key 不存在"))
		return
	}
	switch req.Method {
	case http.MethodPatch:
		r.updateUserAPIAccessKey(w, req, id)
	case http.MethodDelete:
		r.deleteUserAPIAccessKey(w, req, id)
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) userAPIAccessLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID, err := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	input := apiaccess.ListLogsInput{
		UserID:   userID,
		APIKeyID: req.URL.Query().Get("apiKeyId"),
		Status:   req.URL.Query().Get("status"),
		Keyword:  req.URL.Query().Get("keyword"),
		Page:     queryInt(req, "page", 1),
		PageSize: queryInt(req, "pageSize", 10),
	}
	service := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db))
	items, total, err := service.ListLogs(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	stats, err := service.ListLogStats(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":       items,
		"pagination": map[string]any{"total": total, "page": input.Page, "pageSize": input.PageSize},
		"summary":    stats,
	})
}

func (r *Router) userAPIAccessTrend(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID, err := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	startDate, endDate, err := usageTrendRange(req, time.Now())
	if err != nil {
		writeError(w, newAppError(http.StatusBadRequest, err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).UsageTrend(ctx, userID, startDate, endDate)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": items,
		"range": map[string]string{
			"startDate": startDate.Format("2006-01-02"),
			"endDate":   endDate.Format("2006-01-02"),
		},
	})
}

func usageTrendRange(req *http.Request, now time.Time) (time.Time, time.Time, error) {
	location := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	parseDate := func(value string) (time.Time, error) {
		return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), location)
	}

	endDate := today
	if value := strings.TrimSpace(req.URL.Query().Get("endDate")); value != "" {
		parsed, err := parseDate(value)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("结束日期格式不正确")
		}
		endDate = parsed
	}
	startDate := endDate.AddDate(0, 0, -6)
	if value := strings.TrimSpace(req.URL.Query().Get("startDate")); value != "" {
		parsed, err := parseDate(value)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("开始日期格式不正确")
		}
		startDate = parsed
	}
	if endDate.After(today) {
		return time.Time{}, time.Time{}, errors.New("结束日期不能晚于今天")
	}
	if startDate.After(endDate) {
		return time.Time{}, time.Time{}, errors.New("开始日期不能晚于结束日期")
	}
	if int(endDate.Sub(startDate).Hours()/24)+1 > 366 {
		return time.Time{}, time.Time{}, errors.New("单次最多查询 366 天数据")
	}
	return startDate, endDate, nil
}

func (r *Router) listUserAPIAccessKeys(w http.ResponseWriter, req *http.Request) {
	userID, err := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	config := r.dynamicConcurrencyConfig(ctx)
	items, err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).
		WithDynamicConcurrencyConfig(config).
		ListUserKeys(ctx, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) createUserAPIAccessKey(w http.ResponseWriter, req *http.Request) {
	var input struct {
		UserID      string `json:"userId"`
		Name        string `json:"name"`
		BillingMode string `json:"billingMode"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	userID, err := r.requireFrontUser(req, input.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	item, err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).CreateUserKey(ctx, userID, input.Name, input.BillingMode)
	if err != nil {
		if errors.Is(err, apiaccess.ErrInvalidBillingMode) {
			writeError(w, newAppError(http.StatusBadRequest, err.Error()))
			return
		}
		if errors.Is(err, users.ErrEmailNotVerified) {
			writeError(w, newAppError(http.StatusForbidden, err.Error()))
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": item})
}

func (r *Router) revealUserAPIAccessKey(w http.ResponseWriter, req *http.Request, id string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	userID, err := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	key, err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).RevealUserKey(ctx, id, userID)
	if err != nil {
		switch {
		case errors.Is(err, apiaccess.ErrAccessKeyNotFound):
			writeError(w, newAppError(http.StatusNotFound, err.Error()))
		case errors.Is(err, apiaccess.ErrKeyPlainUnavailable):
			writeError(w, newAppError(http.StatusConflict, err.Error()))
		default:
			writeError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"key": key}})
}

func (r *Router) updateUserAPIAccessKey(w http.ResponseWriter, req *http.Request, id string) {
	var input struct {
		UserID string `json:"userId"`
		Status string `json:"status"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	userID, err := r.requireFrontUser(req, input.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	item, err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).UpdateKeyStatus(ctx, id, userID, input.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, newAppError(http.StatusNotFound, "API Key 不存在"))
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (r *Router) deleteUserAPIAccessKey(w http.ResponseWriter, req *http.Request, id string) {
	userID, err := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	if err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).DeleteKey(ctx, id, userID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}

func (r *Router) adminAPIAccessKeys(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := apiaccess.NewRepository(r.db)
	config := r.dynamicConcurrencyConfig(ctx)
	items, err := apiaccess.NewService(repo, users.NewRepository(r.db)).
		WithDynamicConcurrencyConfig(config).
		ListAllKeys(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	stats, err := repo.AdminStats(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"items":              items,
		"stats":              stats,
		"dynamicConcurrency": config,
	}})
}

func (r *Router) adminAPIAccessKeyByID(w http.ResponseWriter, req *http.Request) {
	id := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/admin/api-access/keys/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "API Key 不存在"))
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	switch req.Method {
	case http.MethodPatch:
		var input struct {
			Status           string `json:"status"`
			ConcurrencyLimit *int   `json:"concurrencyLimit"`
		}
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		item, err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).UpdateKeySettings(ctx, id, "", input.Status, input.ConcurrencyLimit)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": item})
	case http.MethodDelete:
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		if err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).DeleteKey(ctx, id, ""); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) adminAPIAccessLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, total, err := apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).ListLogs(ctx, apiaccess.ListLogsInput{
		UserID:   req.URL.Query().Get("userId"),
		APIKeyID: req.URL.Query().Get("apiKeyId"),
		Status:   req.URL.Query().Get("status"),
		Keyword:  req.URL.Query().Get("keyword"),
		Page:     queryInt(req, "page", 1),
		PageSize: queryInt(req, "pageSize", 20),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "pagination": map[string]any{"total": total, "page": queryInt(req, "page", 1), "pageSize": queryInt(req, "pageSize", 20)}})
}

func (r *Router) adminAPIAccessOperations(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().In(time.Local)
	rangeKey, startAt := adminOperationsRange(req.URL.Query().Get("range"), now)
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	data, err := apiaccess.NewRepository(r.db).AdminOperations(
		ctx,
		startAt,
		now,
		rangeKey,
		req.URL.Query().Get("metric"),
		queryInt(req, "limit", 10),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func adminOperationsRange(value string, now time.Time) (string, time.Time) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "7d":
		return "7d", dayStart.AddDate(0, 0, -6)
	case "15d":
		return "15d", dayStart.AddDate(0, 0, -14)
	case "30d":
		return "30d", dayStart.AddDate(0, 0, -29)
	default:
		return "today", dayStart
	}
}

func (r *Router) requireFrontUser(req *http.Request, explicitUserID string) (string, error) {
	token := bearerToken(req)
	if token == "" {
		return "", newAppError(http.StatusUnauthorized, "请先登录")
	}
	payload, err := r.tokens.ParseUserToken(token)
	if err != nil {
		return "", newAppError(http.StatusUnauthorized, "登录已失效，请重新登录")
	}
	if strings.TrimSpace(explicitUserID) != "" && strings.TrimSpace(explicitUserID) != payload.UserID {
		return "", newAppError(http.StatusForbidden, "只能操作自己的账户")
	}
	if err := r.ensureFrontUserActive(req.Context(), payload.UserID); err != nil {
		return "", err
	}
	return payload.UserID, nil
}

func (r *Router) ensureFrontUserActive(ctx context.Context, userID string) error {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(checkCtx, userID)
	if errors.Is(err, sql.ErrNoRows) || user == nil {
		return newAppError(http.StatusUnauthorized, "请先登录")
	}
	if err != nil {
		return err
	}
	if user.Status != "active" {
		return newAppError(http.StatusForbidden, "用户已被禁用")
	}
	return nil
}
