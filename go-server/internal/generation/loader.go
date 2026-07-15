package generation

import (
	"context"

	"aipi-go/internal/database"
	"aipi-go/internal/models"
	"aipi-go/internal/providers"
	"aipi-go/internal/tasks"
)

func modelAndProvider(ctx context.Context, db *database.DB, task *tasks.Task) (*models.Model, *providers.Provider, error) {
	model, err := models.NewRepository(db).FindByID(ctx, task.ModelID)
	if err != nil {
		return nil, nil, err
	}
	provider, err := providers.NewRepository(db).FindByID(ctx, model.ProviderID)
	if err != nil {
		return nil, nil, err
	}
	return model, provider, nil
}
