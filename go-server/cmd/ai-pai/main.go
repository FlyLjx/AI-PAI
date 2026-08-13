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
	"aipi-go/internal/cleanupstatus"
	"aipi-go/internal/config"
	"aipi-go/internal/database"
	"aipi-go/internal/httpserver"
	"aipi-go/internal/imagecache"
	"aipi-go/internal/logging"
	"aipi-go/internal/operations"
	"aipi-go/internal/systemlogs"
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
	workerContext, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	httpserver.StartServiceNotificationWorker(workerContext, db, logger)
	operations.StartOrderExpiryWorker(workerContext, db, logger)
	logCleanupDone := systemlogs.StartAutoCleanupWorker(workerContext, db, logger, cfg.LogDir)

	cleanupTracker := cleanupstatus.New()
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           httpserver.NewRouter(cfg, db, logger, cleanupTracker),
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
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		cleanupTracker.Start(time.Now())
		cleanupCtx, cancel := context.WithCancel(workerContext)
		defer cancel()
		cleanupCtx = cleanupstatus.WithTracker(cleanupCtx, cleanupTracker)
		var cleanupErr error
		if err := database.CleanupLegacyGenerationImageData(cleanupCtx, db.Raw(), time.Now()); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("legacy generation image data cleanup failed", "error", err)
			cleanupErr = err
		} else if errors.Is(err, context.Canceled) {
			cleanupTracker.Cancel(time.Now())
			return
		} else if err == nil {
			logger.Info("legacy generation image data cleanup completed")
		}
		cleanupTracker.SetPhase("cache")
		result, err := imagecache.PurgeTaskImagesInContext(cleanupCtx, imagecache.Directory(), 100*time.Millisecond)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("legacy task image cache cleanup failed", "error", err)
			cleanupErr = errors.Join(cleanupErr, err)
		} else if errors.Is(err, context.Canceled) {
			cleanupTracker.Cancel(time.Now())
			return
		} else if err == nil && (len(result.Deleted) > 0 || len(result.Failures) > 0) {
			logger.Info("legacy task image cache cleanup completed", "deleted", len(result.Deleted), "failed", len(result.Failures))
		}
		if len(result.Failures) > 0 {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("%d cache directories failed to delete", len(result.Failures)))
		}
		if cleanupErr != nil {
			cleanupTracker.Fail(cleanupErr, time.Now())
			return
		}
		cleanupTracker.Complete(time.Now())
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.Info("shutting down server")
	stopWorkers()
	select {
	case <-cleanupDone:
	case <-time.After(5 * time.Second):
		logger.Warn("background image cleanup still running during shutdown")
	}
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	select {
	case <-logCleanupDone:
	case <-ctx.Done():
		logger.Warn("system log cleanup worker shutdown timed out")
	}
}
