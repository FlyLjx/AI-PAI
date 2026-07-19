package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/requestmonitor"
)

func (r *Router) adminRequestMonitor(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()
	filters := requestmonitor.Filters{
		Range:    strings.TrimSpace(req.URL.Query().Get("range")),
		Keyword:  strings.TrimSpace(req.URL.Query().Get("keyword")),
		Method:   strings.TrimSpace(req.URL.Query().Get("method")),
		Status:   strings.TrimSpace(req.URL.Query().Get("status")),
		Page:     queryInt(req, "page", 1),
		PageSize: queryInt(req, "pageSize", 30),
	}
	snapshot, total, err := requestmonitor.NewRepository(r.db).Snapshot(ctx, filters)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": snapshot,
		"pagination": map[string]any{
			"total": total, "page": filters.Page, "pageSize": filters.PageSize,
		},
	})
}
