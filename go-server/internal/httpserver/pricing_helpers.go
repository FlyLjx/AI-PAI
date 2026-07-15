package httpserver

import (
	"context"
	"time"

	"aipi-go/internal/models"
	"aipi-go/internal/pricing"
	"aipi-go/internal/settings"
)

func (r *Router) imageUnitPrice(ctx context.Context, userID string, model models.Model, sizeTier string) (float64, pricing.Result, error) {
	baseUnit := modelPriceForTier(model, sizeTier)
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return baseUnit, pricing.Result{}, err
	}
	incentive, err := pricing.Evaluate(ctx, r.db, values, userID, time.Now())
	if err != nil {
		return baseUnit, incentive, err
	}
	subscriptionDiscount, err := pricing.CurrentSubscriptionDiscount(ctx, r.db, userID)
	if err != nil {
		return baseUnit, incentive, err
	}
	unit, _, _ := pricing.ApplyUnitPrice(baseUnit, incentive, subscriptionDiscount)
	return unit, incentive, nil
}
