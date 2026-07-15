package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/models"
	"aipi-go/internal/providers"
)

type providerInput struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Capability string `json:"capability"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	Status     string `json:"status"`
}

func (r *Router) listProviders(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.createProvider(w, req)
		return
	}
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
	items, err := providers.NewRepository(r.db).FindAll(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	data := make([]providers.PublicProvider, 0, len(items))
	for _, item := range items {
		data = append(data, providers.ToPublic(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) providerByID(w http.ResponseWriter, req *http.Request) {
	id := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/api-providers/"), "/")
	if strings.HasSuffix(id, "/test") {
		r.testProvider(w, req, strings.TrimSuffix(id, "/test"))
		return
	}
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "接口不存在"))
		return
	}
	switch req.Method {
	case http.MethodPatch:
		r.updateProvider(w, req, id)
	case http.MethodDelete:
		r.deleteProvider(w, req, id)
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) createProvider(w http.ResponseWriter, req *http.Request) {
	input, ok := r.parseProviderInput(w, req)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	item, err := providers.NewRepository(r.db).Create(ctx, providers.Provider{
		ID:         newID(),
		Name:       input.Name,
		Type:       input.Type,
		Capability: input.Capability,
		BaseURL:    input.BaseURL,
		APIKey:     input.APIKey,
		Status:     input.Status,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": providers.ToPublic(*item)})
}

func (r *Router) updateProvider(w http.ResponseWriter, req *http.Request, id string) {
	input, ok := r.parseProviderInput(w, req)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	item, err := providers.NewRepository(r.db).Update(ctx, id, providers.Provider{
		Name:       input.Name,
		Type:       input.Type,
		Capability: input.Capability,
		BaseURL:    input.BaseURL,
		APIKey:     input.APIKey,
		Status:     input.Status,
	})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "接口不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": providers.ToPublic(*item)})
}

func (r *Router) deleteProvider(w http.ResponseWriter, req *http.Request, id string) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	if _, _, err := models.NewRepository(r.db).DeleteByProviderID(ctx, id); err != nil {
		writeError(w, err)
		return
	}
	deleted, err := providers.NewRepository(r.db).Delete(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if !deleted {
		writeError(w, newAppError(http.StatusNotFound, "接口不存在"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) parseProviderInput(w http.ResponseWriter, req *http.Request) (providerInput, bool) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return providerInput{}, false
	}
	var input providerInput
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return providerInput{}, false
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.TrimSpace(input.Type)
	input.Capability = defaultString(strings.TrimSpace(input.Capability), "chat_image")
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	input.APIKey = providers.NormalizeAPIKey(input.APIKey)
	input.Status = defaultString(strings.TrimSpace(input.Status), "active")
	if input.Name == "" || input.BaseURL == "" || input.APIKey == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请填写接口名称、地址和密钥"))
		return providerInput{}, false
	}
	if input.Type != "sub2api" && input.Type != "custom" && input.Type != "newapi" {
		writeError(w, newAppError(http.StatusBadRequest, "接口类型不正确"))
		return providerInput{}, false
	}
	if !isSupportedModelCapability(input.Capability) {
		writeError(w, newAppError(http.StatusBadRequest, "接口用途不正确"))
		return providerInput{}, false
	}
	if input.Status != "active" && input.Status != "disabled" {
		writeError(w, newAppError(http.StatusBadRequest, "接口状态不正确"))
		return providerInput{}, false
	}
	return input, true
}
