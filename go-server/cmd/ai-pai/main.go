package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/config"
	"aipi-go/internal/database"
	"aipi-go/internal/httpserver"
	"aipi-go/internal/logging"
)

func main() {
	appclock.ConfigureDefault()
	cfg := config.Load()
	logger := logging.New(cfg.LogLevel, cfg.LogDir)

	sqlDB, err := database.Open(cfg.Database)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	db := database.Wrap(sqlDB)
	defer db.Close()
	if err := database.EnsureSchema(db.Raw()); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpserver.NewRouter(cfg, db, logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       90 * time.Second,
		WriteTimeout:      15 * time.Minute,
		IdleTimeout:       180 * time.Second,
	}

	go func() {
		logger.Info("ai-pai server started", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.Info("shutting down server")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
