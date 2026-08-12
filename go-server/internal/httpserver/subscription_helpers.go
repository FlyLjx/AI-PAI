package httpserver

import (
	"context"
	"net/http"
	"strings"

	"aipi-go/internal/operations"
	"aipi-go/internal/settings"
)

func (r *Router) currentSubscriptionEntitlement(ctx context.Context, userID string) (*operations.SubscriptionEntitlement, error) {
	return operations.NewRepository(r.db).CurrentSubscription(ctx, userID)
}

func subscriptionAccessAllowed(values settings.Settings, userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}
	rawUserIDs := strings.TrimSpace(anyString(values["subscriptionAccessUserIds"]))
	if rawUserIDs == "" {
		rawUserIDs = strings.TrimSpace(anyString(values["subscriptionAccessUserId"]))
	}
	for _, configuredUserID := range strings.FieldsFunc(rawUserIDs, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	}) {
		if strings.TrimSpace(configuredUserID) == userID {
			return true
		}
	}
	return false
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
	return r.currentSubscriptionEntitlement(ctx, userID)
}
