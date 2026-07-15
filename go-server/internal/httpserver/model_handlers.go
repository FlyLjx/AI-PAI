package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"aipi-go/internal/models"
	"aipi-go/internal/providers"
)

type modelInput struct {
	ProviderID         string   `json:"providerId"`
	ModelName          string   `json:"modelName"`
	DisplayName        string   `json:"displayName"`
	Capability         string   `json:"capability"`
	Cost1K             float64  `json:"cost1k"`
	Cost2K             float64  `json:"cost2k"`
	Cost4K             float64  `json:"cost4k"`
	MarkupPercent      float64  `json:"markupPercent"`
	PriceChangePercent float64  `json:"priceChangePercent"`
	Price1K            float64  `json:"price1k"`
	Price2K            float64  `json:"price2k"`
	Price4K            float64  `json:"price4k"`
	AppendSizeToPrompt bool     `json:"appendSizeToPrompt"`
	EnabledSizeTiers   []string `json:"enabledSizeTiers"`
	SortOrder          int      `json:"sortOrder"`
	Status             string   `json:"status"`
}

func (r *Router) listModels(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		r.createModel(w, req)
		return
	}
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, err := models.NewRepository(r.db).FindAll(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	data := make([]models.PublicModel, 0, len(items))
	frontendList := strings.EqualFold(req.URL.Query().Get("dedupe"), "display")
	for _, item := range items {
		if frontendList && isTextOnlyChatModel(item.ModelName, item.DisplayName) {
			continue
		}
		data = append(data, models.ToPublic(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) modelByID(w http.ResponseWriter, req *http.Request) {
	path := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/models/"), "/")
	if path == "sync" && req.Method == http.MethodPost {
		r.syncModels(w, req)
		return
	}
	if path == "delete-many" && req.Method == http.MethodPost {
		r.deleteModels(w, req)
		return
	}
	if path == "sort-orders" && req.Method == http.MethodPatch {
		r.updateModelSortOrders(w, req)
		return
	}
	if path == "" || strings.Contains(path, "/") {
		writeError(w, newAppError(http.StatusNotFound, "模型不存在"))
		return
	}
	switch req.Method {
	case http.MethodPatch:
		r.updateModel(w, req, path)
	case http.MethodDelete:
		r.deleteModel(w, req, path)
	default:
		writeMethodNotAllowed(w)
	}
}

type syncModelsInput struct {
	ProviderID    string  `json:"providerId"`
	Capability    string  `json:"capability"`
	Keyword       string  `json:"keyword"`
	AliasPrefix   string  `json:"aliasPrefix"`
	MarkupPercent float64 `json:"markupPercent"`
}

func (r *Router) syncModels(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input syncModelsInput
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.Capability = defaultString(strings.TrimSpace(input.Capability), "chat_image")
	input.Keyword = strings.ToLower(strings.TrimSpace(input.Keyword))
	input.AliasPrefix = strings.TrimSpace(input.AliasPrefix)
	if input.ProviderID == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请选择接口配置"))
		return
	}
	if !isSupportedModelCapability(input.Capability) {
		writeError(w, newAppError(http.StatusBadRequest, "模型用途不正确"))
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 45*time.Second)
	defer cancel()
	provider, err := providers.NewRepository(r.db).FindByID(ctx, input.ProviderID)
	if errors.Is(err, sql.ErrNoRows) || provider == nil {
		writeError(w, newAppError(http.StatusNotFound, "接口配置不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if provider.Capability != input.Capability {
		writeError(w, newAppError(http.StatusBadRequest, "请选择相同用途的接口进行模型同步"))
		return
	}
	remoteModels, err := r.fetchProviderModelDetails(ctx, provider.BaseURL, provider.APIKey)
	if err != nil {
		writeError(w, providerProbeError(err))
		return
	}

	repo := models.NewRepository(r.db)
	remoteByName := map[string]remoteModel{}
	names := []string{}
	for _, item := range remoteModels {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, exists := remoteByName[name]; exists {
			continue
		}
		remoteByName[name] = item
		if modelNameMatchesCapability(name, input.Capability) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	saved := []models.PublicModel{}
	for _, modelName := range names {
		if input.Keyword != "" && !strings.Contains(strings.ToLower(modelName), input.Keyword) {
			continue
		}
		existing, err := repo.FindByProviderNameAndCapability(ctx, provider.ID, modelName, input.Capability)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeError(w, err)
			return
		}
		remote := remoteByName[modelName]
		cost1k, cost2k, cost4k := remote.Cost1K, remote.Cost2K, remote.Cost4K
		if existing != nil && !hasRemotePrice(remote.remoteModelPrice) {
			cost1k, cost2k, cost4k = existing.Cost1K, existing.Cost2K, existing.Cost4K
		}
		price1k := salePrice(cost1k, input.MarkupPercent)
		price2k := salePrice(cost2k, input.MarkupPercent)
		price4k := salePrice(cost4k, input.MarkupPercent)
		displayName := input.AliasPrefix + modelName
		appendSizeToPrompt := false
		enabledSizeTiers := []string{"1k", "2k", "4k"}
		sortOrder := 100
		priceChangePercent := 0.0
		if existing != nil {
			displayName = existing.DisplayName
			if displayName == "" {
				displayName = input.AliasPrefix + modelName
			}
			if existing.Price1K > 0 {
				price1k = existing.Price1K
			}
			if existing.Price2K > 0 {
				price2k = existing.Price2K
			}
			if existing.Price4K > 0 {
				price4k = existing.Price4K
			}
			appendSizeToPrompt = existing.AppendSizeToPrompt
			enabledSizeTiers = models.ParseEnabledSizeTiersFromStrings(existing.EnabledSizeTiers)
			sortOrder = existing.SortOrder
			priceChangePercent = existing.PriceChangePercent
		}
		item, err := repo.Create(ctx, models.Model{
			ID:                 newID(),
			ProviderID:         provider.ID,
			ModelName:          modelName,
			DisplayName:        displayName,
			Capability:         input.Capability,
			Cost1K:             cost1k,
			Cost2K:             cost2k,
			Cost4K:             cost4k,
			MarkupPercent:      input.MarkupPercent,
			PriceChangePercent: priceChangePercent,
			Price1K:            price1k,
			Price2K:            price2k,
			Price4K:            price4k,
			AppendSizeToPrompt: appendSizeToPrompt,
			EnabledSizeTiers:   enabledSizeTiers,
			SortOrder:          sortOrder,
			Status:             "active",
		})
		if err != nil {
			writeError(w, err)
			return
		}
		saved = append(saved, models.ToPublic(*item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": saved})
}

func (r *Router) deleteModels(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, raw := range input.IDs {
		id := strings.TrimSpace(raw)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		writeError(w, newAppError(http.StatusBadRequest, "请选择要删除的模型"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 20*time.Second)
	defer cancel()
	repo := models.NewRepository(r.db)
	deletedCount := 0
	disabledCount := 0
	usedCount := int64(0)
	for _, id := range ids {
		count, err := repo.CountTaskReferences(ctx, id)
		if err != nil {
			writeError(w, err)
			return
		}
		if count > 0 {
			if _, err := repo.Disable(ctx, id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				writeError(w, err)
				return
			}
			disabledCount++
			usedCount += count
			continue
		}
		deleted, err := repo.Delete(ctx, id)
		if err != nil {
			writeError(w, err)
			return
		}
		if deleted {
			deletedCount++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"deletedCount":  deletedCount,
		"disabledCount": disabledCount,
		"usedCount":     usedCount,
	}})
}

func (r *Router) createModel(w http.ResponseWriter, req *http.Request) {
	input, ok := r.parseModelInput(w, req)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	item, err := models.NewRepository(r.db).Create(ctx, modelFromInput(newID(), input))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": models.ToPublic(*item)})
}

func modelNameMatchesCapability(modelName string, capability string) bool {
	if capability != "chat_image" {
		return true
	}
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	if normalized == "gpt-5-5" {
		return false
	}
	for _, keyword := range []string{
		"gpt-image",
		"codex-gpt-image",
		"dall-e",
		"image",
		"nano-banana",
		"nano banana",
		"nanobanana",
		"banana",
		"gemini-2.5-flash-image",
		"midjourney",
		"stable-diffusion",
		"flux",
		"recraft",
		"ideogram",
	} {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

func isTextOnlyChatModel(modelName string, displayName string) bool {
	for _, value := range []string{modelName, displayName} {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "gpt-5-5" || normalized == "gpt-5.5" || normalized == "gpt5-5" || normalized == "gpt5.5" {
			return true
		}
	}
	return false
}

func salePrice(cost float64, markupPercent float64) float64 {
	value := cost * (1 + markupPercent/100)
	return float64(int(value*10000+0.5)) / 10000
}

func (r *Router) updateModel(w http.ResponseWriter, req *http.Request, id string) {
	input, ok := r.parseModelInput(w, req)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	item, err := models.NewRepository(r.db).Update(ctx, id, modelFromInput(id, input))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "模型不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": models.ToPublic(*item)})
}

func (r *Router) deleteModel(w http.ResponseWriter, req *http.Request, id string) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := models.NewRepository(r.db)
	count, err := repo.CountTaskReferences(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if count > 0 {
		item, err := repo.Disable(ctx, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"action": "disabled",
				"model":  models.ToPublic(*item),
			},
			"message": "模型已有历史任务，已自动改为禁用",
		})
		return
	}
	deleted, err := repo.Delete(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if !deleted {
		writeError(w, newAppError(http.StatusNotFound, "模型不存在"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":    map[string]any{"action": "deleted"},
		"message": "模型已删除",
	})
}

func (r *Router) updateModelSortOrders(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Items []struct {
			ID        string `json:"id"`
			SortOrder int    `json:"sortOrder"`
		} `json:"items"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	items := make([]models.SortOrderItem, 0, len(input.Items))
	for _, item := range input.Items {
		if item.ID == "" {
			continue
		}
		items = append(items, models.SortOrderItem{ID: item.ID, SortOrder: item.SortOrder})
	}
	ctx, cancel := context.WithTimeout(req.Context(), 12*time.Second)
	defer cancel()
	updated, err := models.NewRepository(r.db).UpdateSortOrders(ctx, items)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"updatedCount": updated}})
}

func (r *Router) parseModelInput(w http.ResponseWriter, req *http.Request) (modelInput, bool) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return modelInput{}, false
	}
	var input modelInput
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return modelInput{}, false
	}
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Capability = defaultString(strings.TrimSpace(input.Capability), "chat_image")
	input.Status = defaultString(strings.TrimSpace(input.Status), "active")
	if input.SortOrder == 0 {
		input.SortOrder = 100
	}
	if input.ProviderID == "" || input.ModelName == "" || input.DisplayName == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请填写服务商、模型名和展示名称"))
		return modelInput{}, false
	}
	if !isSupportedModelCapability(input.Capability) {
		writeError(w, newAppError(http.StatusBadRequest, "模型用途不正确"))
		return modelInput{}, false
	}
	if input.Status != "active" && input.Status != "disabled" {
		writeError(w, newAppError(http.StatusBadRequest, "模型状态不正确"))
		return modelInput{}, false
	}
	input.EnabledSizeTiers = models.ParseEnabledSizeTiersFromStrings(input.EnabledSizeTiers)
	return input, true
}

func modelFromInput(id string, input modelInput) models.Model {
	return models.Model{
		ID:                 id,
		ProviderID:         input.ProviderID,
		ModelName:          input.ModelName,
		DisplayName:        input.DisplayName,
		Capability:         input.Capability,
		Cost1K:             input.Cost1K,
		Cost2K:             input.Cost2K,
		Cost4K:             input.Cost4K,
		MarkupPercent:      input.MarkupPercent,
		PriceChangePercent: input.PriceChangePercent,
		Price1K:            input.Price1K,
		Price2K:            input.Price2K,
		Price4K:            input.Price4K,
		AppendSizeToPrompt: input.AppendSizeToPrompt,
		EnabledSizeTiers:   input.EnabledSizeTiers,
		SortOrder:          input.SortOrder,
		Status:             input.Status,
	}
}

func isSupportedModelCapability(value string) bool {
	switch strings.TrimSpace(value) {
	case "chat_image":
		return true
	default:
		return false
	}
}
