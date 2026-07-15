package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/content"
	"aipi-go/internal/operations"
	"aipi-go/internal/settings"
)

func (r *Router) homeBootstrap(w http.ResponseWriter, req *http.Request) {
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
	contentRepo := content.NewRepository(r.db)
	operationRepo := operations.NewRepository(r.db)

	announcements, err := contentRepo.FindAnnouncements(ctx, true, userID, false)
	if err != nil {
		writeError(w, err)
		return
	}
	plans, err := operationRepo.Plans(ctx, true)
	if err != nil {
		writeError(w, err)
		return
	}

	if userID == "" {
		w.Header().Set("Cache-Control", "public, max-age=15")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=10")
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"settings":          settings.Public(values),
		"announcements":     announcements,
		"subscriptionPlans": plans,
	}})
}
