package httpserver

import (
	"context"
	"math"
	"net/http"
	"strings"

	"aipi-go/internal/models"
	"aipi-go/internal/operations"
)

const (
	generationBillingModeAuto         = "auto"
	generationBillingModeSubscription = "subscription"
	generationBillingModeBalance      = "balance"
)

func (r *Router) generationBillingQuote(ctx context.Context, userID string, model models.Model, sizeTier string, quantity int, billingMode string) (float64, int, error) {
	if quantity < 1 {
		quantity = 1
	}
	billingMode = normalizeGenerationBillingMode(billingMode)
	if billingMode == "" {
		return 0, 0, newAppError(http.StatusBadRequest, "API Key 计费模式不正确")
	}
	if billingMode != generationBillingModeBalance {
		entitlement, err := r.currentSubscriptionEntitlement(ctx, userID)
		if err != nil {
			return 0, 0, err
		}
		quotaUnitsPerImage := subscriptionQuotaUnitsPerImage(modelPriceForTier(model, sizeTier))
		quotaUnits := subscriptionQuotaUnitsForRequest(quotaUnitsPerImage, quantity)
		handled, err := generationSubscriptionBillingQuote(entitlement, model, quotaUnits, billingMode == generationBillingModeSubscription)
		if err != nil {
			return 0, 0, err
		}
		if handled {
			return 0, quotaUnitsPerImage, nil
		}
	}

	unitPrice, err := r.modelPriceForUser(ctx, userID, model, sizeTier)
	if err != nil {
		return 0, 0, err
	}
	return generationBalanceCost(unitPrice, quantity), 0, nil
}

func generationBalanceInsufficientMessage(billingMode string) string {
	if billingMode == generationBillingModeBalance {
		return "账户余额不足，请先充值"
	}
	return "账户余额不足，请充值或开通订阅"
}

func generationSubscriptionBillingQuote(entitlement *operations.SubscriptionEntitlement, model models.Model, quotaUnits int, required bool) (bool, error) {
	if entitlement == nil || !entitlement.IsPaid {
		if required {
			return true, newAppError(http.StatusPaymentRequired, "订阅已到期或未开通，请续费后再调用")
		}
		return false, nil
	}
	if err := requireGenerationQuotaForEntitlement(entitlement, model, quotaUnits); err != nil {
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

func subscriptionQuotaUnitsPerImage(unitPrice float64) int {
	unitPrice = normalizedCreditAmount(unitPrice)
	if unitPrice <= 0 {
		return 0
	}
	scaled := unitPrice * 100
	if scaled >= float64(math.MaxInt) {
		return math.MaxInt
	}
	return int(math.Ceil(scaled - 1e-9))
}

func subscriptionQuotaUnitsForRequest(perImage int, quantity int) int {
	if perImage <= 0 {
		return 0
	}
	if quantity < 1 {
		quantity = 1
	}
	if quantity > math.MaxInt/perImage {
		return math.MaxInt
	}
	return perImage * quantity
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

func requireGenerationQuotaForEntitlement(entitlement *operations.SubscriptionEntitlement, model models.Model, quantity int) error {
	if quantity < 0 {
		quantity = 0
	}
	if entitlement == nil || !entitlement.IsPaid {
		return newAppError(http.StatusPaymentRequired, "订阅已到期或未开通，请续费后再调用")
	}
	if len(entitlement.AllowedProviderIDs) > 0 && !stringInList(entitlement.AllowedProviderIDs, model.ProviderID) {
		return newAppError(http.StatusForbidden, "当前订阅套餐不支持该接口")
	}
	if len(entitlement.AllowedModelIDs) > 0 && !stringInList(entitlement.AllowedModelIDs, model.ID) {
		return newAppError(http.StatusForbidden, "当前订阅套餐不支持该模型")
	}
	if entitlement.QuotaRemaining < quantity {
		return newAppError(http.StatusPaymentRequired, "本周期生成额度不足，请续费或升级订阅")
	}
	return nil
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

func effectiveOutputFormat(outputFormat string) string {
	normalized := normalizeOutputFormat(outputFormat)
	if normalized == "" {
		return "png"
	}
	return normalized
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
