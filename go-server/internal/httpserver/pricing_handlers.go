package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/pricing"
	"aipi-go/internal/settings"
)

func (r *Router) incentiveStatus(w http.ResponseWriter, req *http.Request) {
	r.activityStatus(w, req)
}

func (r *Router) activityStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID := strings.TrimSpace(req.URL.Query().Get("userId"))
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := pricing.Evaluate(ctx, r.db, values, userID, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}
