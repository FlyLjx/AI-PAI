package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/models"
	"aipi-go/internal/pricing"
	"aipi-go/internal/users"
)

func (r *Router) adminUserModelPrices(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := pricing.NewUserModelPriceRepository(r.db)
	if req.Method == http.MethodGet {
		items, err := repo.FindAll(ctx)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items})
		return
	}

	var input struct {
		UserID    string  `json:"userId"`
		ModelID   string  `json:"modelId"`
		UnitPrice float64 `json:"unitPrice"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.ModelID = strings.TrimSpace(input.ModelID)
	if input.UserID == "" || input.ModelID == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请选择用户和生图模型"))
		return
	}
	unitPrice, ok := pricing.NormalizeUserModelUnitPrice(input.UnitPrice)
	if !ok {
		writeError(w, newAppError(http.StatusBadRequest, pricing.ErrInvalidUserModelUnitPrice.Error()))
		return
	}

	user, err := users.NewRepository(r.db).FindByID(ctx, input.UserID)
	if errors.Is(err, sql.ErrNoRows) || user == nil {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.EqualFold(strings.TrimSpace(user.Role), "admin") {
		writeError(w, newAppError(http.StatusBadRequest, "专属扣费只支持普通用户"))
		return
	}

	model, err := models.NewRepository(r.db).FindByID(ctx, input.ModelID)
	if errors.Is(err, sql.ErrNoRows) || model == nil {
		writeError(w, newAppError(http.StatusNotFound, "模型不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if model.Capability != "chat_image" || !modelNameMatchesCapability(model.ModelName, model.Capability) {
		writeError(w, newAppError(http.StatusBadRequest, "请选择生图模型"))
		return
	}

	item, err := repo.Save(ctx, pricing.UserModelPriceOverride{
		ID:        newID(),
		UserID:    user.ID,
		ModelID:   model.ID,
		UnitPrice: unitPrice,
	})
	if errors.Is(err, pricing.ErrInvalidUserModelUnitPrice) {
		writeError(w, newAppError(http.StatusBadRequest, pricing.ErrInvalidUserModelUnitPrice.Error()))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (r *Router) adminUserModelPriceByID(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	id := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/admin/user-model-prices/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusBadRequest, "扣费规则不存在"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	deleted, err := pricing.NewUserModelPriceRepository(r.db).Delete(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if !deleted {
		writeError(w, newAppError(http.StatusNotFound, "扣费规则不存在"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}
