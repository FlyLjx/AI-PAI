package main

import (
	"context"
	"fmt"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/config"
	"aipi-go/internal/content"
	"aipi-go/internal/database"
	"aipi-go/internal/operations"
	"aipi-go/internal/pricing"
	"aipi-go/internal/settings"
	"aipi-go/internal/tasks"
)

func main() {
	appclock.ConfigureDefault()
	cfg := config.Load()
	raw, err := database.Open(cfg.Database)
	if err != nil {
		panic(err)
	}
	defer raw.Close()
	db := database.Wrap(raw)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	values, err := settings.NewRepository(db).Get(ctx)
	if err != nil {
		panic(fmt.Errorf("settings.Get: %w", err))
	}
	if _, err := pricing.Evaluate(ctx, db, values, "", time.Now()); err != nil {
		panic(fmt.Errorf("pricing.Evaluate: %w", err))
	}
	if _, err := operations.NewRepository(db).Dashboard(ctx); err != nil {
		panic(fmt.Errorf("operations.Dashboard: %w", err))
	}
	if _, _, err := tasks.NewRepository(db).FindAdminList(ctx, tasks.ListInput{Page: 1, PageSize: 10}); err != nil {
		panic(fmt.Errorf("tasks.FindAdminList: %w", err))
	}
	if _, _, err := tasks.NewRepository(db).FindImages(ctx, tasks.ListInput{Page: 1, PageSize: 10}); err != nil {
		panic(fmt.Errorf("tasks.FindImages: %w", err))
	}
	if _, err := content.NewRepository(db).FindAnnouncements(ctx, true, "", false); err != nil {
		panic(fmt.Errorf("content.FindAnnouncements: %w", err))
	}
	if _, err := operations.NewRepository(db).Plans(ctx, true); err != nil {
		panic(fmt.Errorf("operations.Plans: %w", err))
	}

	fmt.Println("postgres smoke ok")
}
