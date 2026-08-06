package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"aipi-go/internal/operations"
	"aipi-go/internal/settings"
)

const (
	defaultFreeHourlyGenerationQuota = 2
	defaultFreeDailyGenerationQuota  = 5
	defaultFreeGenerationQuota       = 10
)

func (r *Router) currentSubscriptionEntitlement(ctx context.Context, userID string) (*operations.SubscriptionEntitlement, error) {
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return nil, err
	}
	return r.currentSubscriptionEntitlementWithLimits(ctx, userID, freeQuotaLimitsFromSettings(values))
}

func (r *Router) currentSubscriptionEntitlementWithLimits(ctx context.Context, userID string, limits operations.FreeQuotaLimits) (*operations.SubscriptionEntitlement, error) {
	return operations.NewRepository(r.db).CurrentSubscription(ctx, userID, limits)
}

func freeQuotaLimitsFromSettings(values settings.Settings) operations.FreeQuotaLimits {
	return operations.FreeQuotaLimits{
		Hourly:  generationQuotaSetting(values["freeHourlyGenerationQuota"], defaultFreeHourlyGenerationQuota),
		Daily:   generationQuotaSetting(values["freeDailyGenerationQuota"], defaultFreeDailyGenerationQuota),
		Monthly: generationQuotaSetting(values["freeGenerationQuota"], defaultFreeGenerationQuota),
	}
}

func subscriptionAccessAllowed(values settings.Settings, userID string) bool {
	configuredUserID := strings.TrimSpace(anyString(values["subscriptionAccessUserId"]))
	return configuredUserID != "" && configuredUserID == strings.TrimSpace(userID)
}

func (r *Router) requireSubscriptionAccess(ctx context.Context, userID string) error {
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return err
	}
	if !subscriptionAccessAllowed(values, userID) {
		return newAppError(http.StatusForbidden, "当前账号暂未开放订阅功能")
	}
	return nil
}

func (r *Router) currentSubscriptionEntitlementForFrontUser(ctx context.Context, userID string) (*operations.SubscriptionEntitlement, error) {
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return nil, err
	}
	if !subscriptionAccessAllowed(values, userID) {
		return nil, nil
	}
	return r.currentSubscriptionEntitlementWithLimits(ctx, userID, freeQuotaLimitsFromSettings(values))
}

func generationQuotaSetting(value any, fallback int) int {
	switch item := value.(type) {
	case int:
		if item >= 0 {
			return item
		}
	case int64:
		if item >= 0 {
			return int(item)
		}
	case float64:
		if item >= 0 {
			return int(item)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(item))
		if err == nil && parsed >= 0 {
			return parsed
		}
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			parsed, err := strconv.Atoi(text)
			if err == nil && parsed >= 0 {
				return parsed
			}
		}
	}
	return fallback
}
