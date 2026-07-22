package httpserver

import (
	"context"
	"net/http"
	"time"

	"aipi-go/internal/settings"
	"aipi-go/internal/tasks"
)

type upstreamMaintenanceInput struct {
	Enabled bool `json:"enabled"`
}

func (r *Router) initializeUpstreamMaintenancePause() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("upstream maintenance setting lookup failed", "error", err)
		}
		return
	}
	if anyBool(values["upstreamMaintenanceEnabled"]) {
		r.queue.SetPaused(true)
	}
}

func (r *Router) upstreamMaintenance(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	switch req.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
		defer cancel()
		snapshot, err := r.upstreamMaintenanceSnapshot(ctx)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": snapshot})
	case http.MethodPatch:
		var input upstreamMaintenanceInput
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
		defer cancel()
		if !input.Enabled {
			if err := r.queue.TouchWaitingTasks(ctx); err != nil {
				writeError(w, err)
				return
			}
		}
		values, err := settings.NewRepository(r.db).Update(ctx, settings.Settings{"upstreamMaintenanceEnabled": input.Enabled})
		if err != nil {
			writeError(w, err)
			return
		}
		r.queue.SetPaused(anyBool(values["upstreamMaintenanceEnabled"]))
		snapshot, err := r.upstreamMaintenanceSnapshot(ctx)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": snapshot})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) upstreamMaintenanceSnapshot(ctx context.Context) (map[string]any, error) {
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		return nil, err
	}
	paused, pausedAt := r.queue.PauseSnapshot()
	enabled := anyBool(values["upstreamMaintenanceEnabled"]) || paused
	stats, err := tasks.NewRepository(r.db).Stats(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"enabled":         enabled,
		"queuePaused":     paused,
		"queuedTasks":     int(stats.Queued),
		"pendingTasks":    int(stats.Pending),
		"processingTasks": int(stats.Processing),
		"waitingTasks":    int(stats.Queued + stats.Pending),
		"fetchedAt":       time.Now().Format(time.RFC3339),
	}
	if !pausedAt.IsZero() {
		result["pausedAt"] = pausedAt.Format(time.RFC3339)
	}
	return result, nil
}
