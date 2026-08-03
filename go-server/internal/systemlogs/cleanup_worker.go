package systemlogs

import (
	"context"
	"log/slog"
	"math"
	"time"

	"aipi-go/internal/database"
	"aipi-go/internal/imagecache"
	"aipi-go/internal/settings"
)

const (
	autoCleanupInterval = time.Hour
	autoCleanupTimeout  = 30 * time.Second
)

type autoCleanupWorker struct {
	service       Service
	logger        *slog.Logger
	loadSettings  func(context.Context) (settings.Settings, error)
	now           func() time.Time
	interval      time.Duration
	taskImageDir  string
	cleanupImages func(string, time.Time, int) (imagecache.TaskImageCleanupResult, error)
}

func StartAutoCleanupWorker(ctx context.Context, db *database.DB, logger *slog.Logger, logDir string) <-chan struct{} {
	repository := settings.NewRepository(db)
	worker := autoCleanupWorker{
		service:       New(logDir),
		logger:        logger,
		loadSettings:  repository.Get,
		now:           time.Now,
		interval:      autoCleanupInterval,
		taskImageDir:  imagecache.Directory(),
		cleanupImages: imagecache.CleanupTaskImagesOlderThanIn,
	}
	return worker.start(ctx)
}

func (w autoCleanupWorker) start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.runOnce(ctx)

		interval := w.interval
		if interval <= 0 {
			interval = autoCleanupInterval
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runOnce(ctx)
			}
		}
	}()
	return done
}

func (w autoCleanupWorker) runOnce(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	checkCtx, cancel := context.WithTimeout(ctx, autoCleanupTimeout)
	defer cancel()
	values, err := w.loadSettings(checkCtx)
	if err != nil {
		w.logWarn("system log cleanup settings load failed", "error", err)
		return
	}
	now := time.Now()
	if w.now != nil {
		now = w.now()
	}

	// System logs and task image caches have separate switches and retention
	// windows. One invalid or disabled policy must not prevent the other from
	// running during the same worker tick.
	if cleanupSettingBool(values["systemLogAutoCleanupEnabled"]) {
		retentionDays, ok := cleanupSettingPositiveInt(values["systemLogRetentionDays"])
		if !ok {
			w.logWarn("system log cleanup settings invalid", "setting", "systemLogRetentionDays")
		} else {
			result, cleanupErr := w.service.CleanupOlderThan(now, retentionDays)
			if cleanupErr != nil {
				w.logWarn("system log cleanup failed", "error", cleanupErr)
			} else {
				for _, failure := range result.Failures {
					w.logWarn("system log file cleanup failed", "name", failure.Name, "error", failure.Error)
				}
				if len(result.Deleted) > 0 || len(result.Failures) > 0 {
					w.logInfo(
						"system log cleanup completed",
						"retentionDays", retentionDays,
						"deleted", len(result.Deleted),
						"skipped", len(result.Skipped),
						"failed", len(result.Failures),
					)
				}
			}
		}
	}

	if cleanupSettingBool(values["taskImageAutoCleanupEnabled"]) {
		retentionDays, ok := cleanupSettingPositiveInt(values["taskImageRetentionDays"])
		if !ok {
			w.logWarn("task image cleanup settings invalid", "setting", "taskImageRetentionDays")
			return
		}
		if w.taskImageDir == "" || w.cleanupImages == nil {
			w.logWarn("task image cleanup unavailable", "directory", w.taskImageDir)
			return
		}
		result, cleanupErr := w.cleanupImages(w.taskImageDir, now, retentionDays)
		if cleanupErr != nil {
			w.logWarn("task image cleanup failed", "directory", w.taskImageDir, "error", cleanupErr)
			return
		}
		for _, failure := range result.Failures {
			w.logWarn("task image cache cleanup failed", "taskId", failure.Name, "error", failure.Error)
		}
		if len(result.Deleted) > 0 || len(result.Failures) > 0 {
			w.logInfo(
				"task image cleanup completed",
				"directory", w.taskImageDir,
				"retentionDays", retentionDays,
				"deleted", len(result.Deleted),
				"skipped", len(result.Skipped),
				"failed", len(result.Failures),
			)
		}
	}
}

func cleanupSettingBool(value any) bool {
	enabled, _ := value.(bool)
	return enabled
}

func cleanupSettingPositiveInt(value any) (int, bool) {
	var number float64
	switch item := value.(type) {
	case float64:
		number = item
	case float32:
		number = float64(item)
	case int:
		return item, item > 0 && item <= maxLogRetentionDays
	case int64:
		if item <= 0 || item > maxLogRetentionDays || int64(int(item)) != item {
			return 0, false
		}
		return int(item), true
	default:
		return 0, false
	}
	if number <= 0 || number > maxLogRetentionDays || math.Trunc(number) != number {
		return 0, false
	}
	return int(number), true
}

func (w autoCleanupWorker) logWarn(message string, args ...any) {
	if w.logger != nil {
		w.logger.Warn(message, args...)
	}
}

func (w autoCleanupWorker) logInfo(message string, args ...any) {
	if w.logger != nil {
		w.logger.Info(message, args...)
	}
}
