package httpserver

import (
	"context"

	"aipi-go/internal/models"
	"aipi-go/internal/pricing"
)

func (r *Router) modelPriceForUser(ctx context.Context, userID string, model models.Model, sizeTier string) (float64, error) {
	baseUnit := modelPriceForTier(model, sizeTier)
	if model.ID == "" || userID == "" {
		return baseUnit, nil
	}
	override, found, err := pricing.NewUserModelPriceRepository(r.db).FindForUserAndModel(ctx, userID, model.ID)
	if err != nil {
		return 0, err
	}
	if found {
		return override, nil
	}
	return baseUnit, nil
}
