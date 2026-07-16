package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/database"
	"aipi-go/internal/models"
	"aipi-go/internal/operations"
	"aipi-go/internal/tasks"
	"aipi-go/internal/users"
)

type generateImageInput struct {
	UserID                string   `json:"userId"`
	ModelID               string   `json:"modelId"`
	Prompt                string   `json:"prompt"`
	Capability            string   `json:"capability"`
	SizeTier              string   `json:"sizeTier"`
	Size                  string   `json:"size"`
	TransparentBackground bool     `json:"transparentBackground"`
	Quantity              int      `json:"quantity"`
	ReferenceImageURL     string   `json:"referenceImageUrl"`
	ReferenceImageURLs    []string `json:"referenceImageUrls"`
	MaskImageURL          string   `json:"maskImageUrl"`
	OutputFormat          string   `json:"outputFormat"`
	OpenAIParams          any      `json:"openaiParams"`
}

const (
	generationBillingModeAuto         = "auto"
	generationBillingModeSubscription = "subscription"
	generationBillingModeBalance      = "balance"
)

func (r *Router) generateImage(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	savedTask, err := r.createGenerationTask(req)
	if err != nil {
		writeError(w, err)
		return
	}
	r.queue.Enqueue(savedTask.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"data": tasks.ToPublic(savedTask)})
}

func (r *Router) generateImageStream(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	savedTask, err := r.createGenerationTask(req)
	if err != nil {
		writeError(w, err)
		return
	}
	r.queue.Enqueue(savedTask.ID)
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	writeGenerationSSE(w, "task", map[string]any{"data": tasks.ToPublic(savedTask)})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	finalTask, err := r.waitForCompatTask(req.Context(), savedTask.ID)
	if err != nil {
		writeGenerationSSE(w, "error", map[string]any{"message": err.Error()})
		return
	}
	writeGenerationSSE(w, "task", map[string]any{"data": tasks.ToPublic(finalTask)})
	writeGenerationSSE(w, "done", map[string]any{"data": tasks.ToPublic(finalTask)})
}

func (r *Router) createGenerationTask(req *http.Request) (*tasks.Task, error) {
	var input generateImageInput
	if err := decodeJSON(req, &input); err != nil {
		return nil, newAppError(http.StatusBadRequest, "请求参数不正确")
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.ModelID = strings.TrimSpace(input.ModelID)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Capability = defaultString(strings.TrimSpace(input.Capability), "chat_image")
	input.SizeTier = defaultString(strings.TrimSpace(input.SizeTier), "1k")
	input.Size = strings.TrimSpace(input.Size)
	input.OutputFormat = normalizeOutputFormat(input.OutputFormat)
	if input.Quantity == 0 {
		input.Quantity = 1
	}
	if input.UserID == "" || input.ModelID == "" {
		return nil, newAppError(http.StatusBadRequest, "缺少用户或模型")
	}
	if input.Capability == "chat_image" && input.Prompt == "" {
		return nil, newAppError(http.StatusBadRequest, "缺少用户、模型或提示词")
	}
	if input.Capability != "chat_image" {
		return nil, newAppError(http.StatusBadRequest, "任务能力不正确")
	}
	if input.Quantity < 1 || input.Quantity > 10 {
		return nil, newAppError(http.StatusBadRequest, "生成数量必须在 1 到 10 之间")
	}
	if input.SizeTier != "1k" && input.SizeTier != "2k" && input.SizeTier != "4k" {
		return nil, newAppError(http.StatusBadRequest, "清晰度参数不正确")
	}

	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	model, err := models.NewRepository(r.db).FindByID(ctx, input.ModelID)
	if errors.Is(err, sql.ErrNoRows) || model == nil {
		return nil, newAppError(http.StatusNotFound, "模型不存在")
	}
	if err != nil {
		return nil, err
	}
	if model.Status != "active" {
		return nil, newAppError(http.StatusBadRequest, "模型已禁用")
	}
	if model.Capability != input.Capability {
		return nil, newAppError(http.StatusBadRequest, "当前模型用途与任务不匹配，请重新选择")
	}
	if !modelNameMatchesCapability(model.ModelName, input.Capability) {
		return nil, newAppError(http.StatusBadRequest, "当前模型不是生图模型，请重新选择")
	}
	if !sizeTierEnabled(model.EnabledSizeTiers, input.SizeTier) {
		return nil, newAppError(http.StatusBadRequest, "当前模型未开放 "+strings.ToUpper(input.SizeTier)+" 清晰度，请重新选择")
	}
	user, err := users.NewRepository(r.db).FindByID(ctx, input.UserID)
	if errors.Is(err, sql.ErrNoRows) || user == nil {
		return nil, newAppError(http.StatusNotFound, "用户不存在")
	}
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, newAppError(http.StatusForbidden, "用户已被禁用")
	}

	size := input.Size
	if size == "" {
		size = defaultImageSize(input.SizeTier)
	}
	prompt := input.Prompt
	var savedTask *tasks.Task
	if err := r.withUserGenerationLock(ctx, user.ID, func(tx *database.Tx) error {
		costCredits, err := r.generationBillingQuote(ctx, tx, user.ID, *model, input.SizeTier, input.Quantity, generationBillingModeAuto)
		if err != nil {
			return err
		}
		task := tasks.Task{
			ID:                    newID(),
			UserID:                user.ID,
			ModelID:               model.ID,
			ProviderID:            model.ProviderID,
			Capability:            input.Capability,
			Prompt:                prompt,
			ReferenceImageURL:     referenceImagePayload(req, input),
			SizeTier:              input.SizeTier,
			Size:                  &size,
			OutputFormat:          effectiveOutputFormat(input.OutputFormat, input.TransparentBackground),
			TransparentBackground: input.TransparentBackground || input.OutputFormat == "png",
			Quantity:              input.Quantity,
			UserIP:                requestIP(req),
			CostCredits:           costCredits,
			ModelCostCredits:      0,
			RemainingCredits:      0,
			DurationSeconds:       0,
			Status:                tasks.StatusQueued,
			PublicStatus:          "private",
		}
		savedTask, err = tasks.NewRepository(r.db).CreateWithTx(ctx, tx, task)
		return err
	}); err != nil {
		return nil, err
	}
	if r.logger != nil {
		r.logger.Info("[generation:request-accepted]",
			"taskId", savedTask.ID,
			"userId", savedTask.UserID,
			"modelId", savedTask.ModelID,
			"sizeTier", savedTask.SizeTier,
			"size", ptrStringValue(savedTask.Size),
			"outputFormat", savedTask.OutputFormat,
			"quantity", savedTask.Quantity,
			"providerId", savedTask.ProviderID,
		)
	}
	return savedTask, nil
}

func (r *Router) generationBillingQuote(ctx context.Context, tx *database.Tx, userID string, model models.Model, sizeTier string, quantity int, billingMode string) (float64, error) {
	if quantity < 1 {
		quantity = 1
	}
	billingMode = normalizeGenerationBillingMode(billingMode)
	if billingMode == "" {
		return 0, newAppError(http.StatusBadRequest, "API Key 计费模式不正确")
	}
	if billingMode != generationBillingModeBalance {
		entitlement, err := r.currentSubscriptionEntitlement(ctx, userID)
		if err != nil {
			return 0, err
		}
		handled, err := generationSubscriptionBillingQuote(entitlement, model, quantity, billingMode == generationBillingModeSubscription)
		if err != nil {
			return 0, err
		}
		if handled {
			return 0, nil
		}
	}

	price := generationBalanceCost(modelPriceForTier(model, sizeTier), quantity)
	var credits float64
	var reserved float64
	if err := tx.QueryRowContext(ctx, `SELECT credits FROM users WHERE id = ?`, userID).Scan(&credits); err != nil {
		return 0, err
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cost_credits), 0)
		FROM generation_tasks
		WHERE user_id = ? AND status IN ('queued', 'pending', 'processing')
	`, userID).Scan(&reserved); err != nil {
		return 0, err
	}
	if !hasAvailableGenerationBalance(credits, reserved, price) {
		return 0, newAppError(http.StatusPaymentRequired, generationBalanceInsufficientMessage(billingMode))
	}
	return price, nil
}

func generationBalanceInsufficientMessage(billingMode string) string {
	if billingMode == generationBillingModeBalance {
		return "账户余额不足，请先充值"
	}
	return "账户余额不足，请充值或开通订阅"
}

func generationSubscriptionBillingQuote(entitlement *operations.SubscriptionEntitlement, model models.Model, quantity int, required bool) (bool, error) {
	if entitlement == nil || !entitlement.IsPaid {
		if required {
			return true, newAppError(http.StatusPaymentRequired, "订阅已到期或未开通，请续费后再调用")
		}
		return false, nil
	}
	if err := requireGenerationQuotaForEntitlement(entitlement, model, quantity); err != nil {
		return true, err
	}
	return true, nil
}

func normalizeGenerationBillingMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", generationBillingModeAuto:
		return generationBillingModeAuto
	case generationBillingModeSubscription:
		return generationBillingModeSubscription
	case generationBillingModeBalance:
		return generationBillingModeBalance
	default:
		return ""
	}
}

func generationBalanceCost(unitPrice float64, quantity int) float64 {
	if quantity < 1 {
		quantity = 1
	}
	return normalizedCreditAmount(unitPrice * float64(quantity))
}

func hasAvailableGenerationBalance(credits float64, reserved float64, cost float64) bool {
	available := normalizedCreditAmount(credits) - normalizedCreditAmount(reserved)
	return normalizedCreditAmount(available) >= normalizedCreditAmount(cost)
}

func normalizedCreditAmount(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*10000) / 10000
}

func (r *Router) requireGenerationSubscription(ctx context.Context, userID string, model models.Model) error {
	plan, err := operations.NewRepository(r.db).CurrentSubscriptionPlan(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) || plan == nil {
		return newAppError(http.StatusPaymentRequired, "请先开通订阅后再生成图片")
	}
	if err != nil {
		return err
	}
	if len(plan.AllowedProviderIDs) > 0 && !stringInList(plan.AllowedProviderIDs, model.ProviderID) {
		return newAppError(http.StatusForbidden, "当前订阅套餐不支持该接口")
	}
	if len(plan.AllowedModelIDs) > 0 && !stringInList(plan.AllowedModelIDs, model.ID) {
		return newAppError(http.StatusForbidden, "当前订阅套餐不支持该模型")
	}
	return nil
}

func (r *Router) requireGenerationQuota(ctx context.Context, userID string, model models.Model, quantity int) error {
	entitlement, err := r.currentSubscriptionEntitlement(ctx, userID)
	if err != nil {
		return err
	}
	return requireGenerationQuotaForEntitlement(entitlement, model, quantity)
}

func requireGenerationQuotaForEntitlement(entitlement *operations.SubscriptionEntitlement, model models.Model, quantity int) error {
	if quantity < 1 {
		quantity = 1
	}
	if entitlement == nil {
		return newAppError(http.StatusPaymentRequired, "免费额度已用完，请开通订阅")
	}
	if entitlement.IsPaid {
		if len(entitlement.AllowedProviderIDs) > 0 && !stringInList(entitlement.AllowedProviderIDs, model.ProviderID) {
			return newAppError(http.StatusForbidden, "当前订阅套餐不支持该接口")
		}
		if len(entitlement.AllowedModelIDs) > 0 && !stringInList(entitlement.AllowedModelIDs, model.ID) {
			return newAppError(http.StatusForbidden, "当前订阅套餐不支持该模型")
		}
	} else {
		for _, window := range entitlement.QuotaWindows {
			if window.QuotaRemaining < quantity {
				return newAppError(http.StatusPaymentRequired, freeQuotaWindowLimitMessage(window))
			}
		}
	}
	if entitlement.QuotaRemaining < quantity {
		if entitlement.IsPaid {
			return newAppError(http.StatusPaymentRequired, "本周期生成额度不足，请续费或升级订阅")
		}
		return newAppError(http.StatusPaymentRequired, "免费额度已用完，请开通订阅")
	}
	return nil
}

func freeQuotaWindowLimitMessage(window operations.SubscriptionQuotaWindow) string {
	switch window.Key {
	case "hour":
		return "免费版每小时额度不足，请稍后再试或开通订阅"
	case "day":
		return "免费版今日额度不足，请明天再试或开通订阅"
	case "month":
		return "免费版本月额度已用完，请开通订阅"
	default:
		if strings.TrimSpace(window.Label) != "" {
			return "免费版" + strings.TrimSpace(window.Label) + "额度不足，请开通订阅"
		}
		return "免费额度已用完，请开通订阅"
	}
}

func stringInList(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func effectiveOutputFormat(outputFormat string, transparentBackground bool) string {
	normalized := normalizeOutputFormat(outputFormat)
	if transparentBackground || normalized == "png" {
		return "png"
	}
	if normalized == "" {
		return "jpeg"
	}
	return normalized
}

func referenceImagePayload(req *http.Request, input generateImageInput) *string {
	urls := []string{}
	if strings.TrimSpace(input.ReferenceImageURL) != "" {
		urls = appendUniqueReferencePayload(urls, absoluteURL(req, strings.TrimSpace(input.ReferenceImageURL)))
	}
	for _, item := range input.ReferenceImageURLs {
		if strings.TrimSpace(item) != "" {
			urls = appendUniqueReferencePayload(urls, absoluteURL(req, strings.TrimSpace(item)))
		}
	}
	if strings.TrimSpace(input.MaskImageURL) != "" {
		urls = appendUniqueReferencePayload(urls, "mask:"+absoluteURL(req, strings.TrimSpace(input.MaskImageURL)))
	}
	if len(urls) == 0 {
		return nil
	}
	if len(urls) == 1 && !strings.HasPrefix(urls[0], "mask:") {
		return &urls[0]
	}
	bytes, _ := json.Marshal(urls)
	value := string(bytes)
	return &value
}

func appendUniqueReferencePayload(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func writeGenerationSSE(w http.ResponseWriter, event string, payload any) {
	bytes, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
