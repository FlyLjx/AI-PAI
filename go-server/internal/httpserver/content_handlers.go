package httpserver

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/content"
)

func (r *Router) publicAnnouncements(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	includeSigned := req.URL.Query().Get("includeSigned") == "1" || req.URL.Query().Get("includeSigned") == "true"
	items, err := content.NewRepository(r.db).FindAnnouncements(ctx, true, strings.TrimSpace(req.URL.Query().Get("userId")), includeSigned)
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(req.URL.Query().Get("userId")) == "" {
		w.Header().Set("Cache-Control", "public, max-age=15")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=10")
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) announcements(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := content.NewRepository(r.db)
	switch req.Method {
	case http.MethodGet:
		items, err := repo.FindAnnouncements(ctx, false, "", true)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items})
	case http.MethodPost:
		var input content.Announcement
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		input.ID = defaultString(strings.TrimSpace(input.ID), newID())
		item, err := repo.SaveAnnouncement(ctx, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": item})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) announcementByID(w http.ResponseWriter, req *http.Request) {
	path := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/announcements/"), "/")
	if strings.HasSuffix(path, "/sign") {
		r.signAnnouncement(w, req, strings.TrimSuffix(path, "/sign"))
		return
	}
	id := strings.Trim(path, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "公告不存在"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := content.NewRepository(r.db)
	switch req.Method {
	case http.MethodPatch:
		if _, err := r.requireAdmin(req); err != nil {
			writeError(w, err)
			return
		}
		var input content.Announcement
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		input.ID = id
		item, err := repo.SaveAnnouncement(ctx, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": item})
	case http.MethodDelete:
		if _, err := r.requireAdmin(req); err != nil {
			writeError(w, err)
			return
		}
		ok, err := repo.DeleteAnnouncement(ctx, id)
		if err != nil {
			writeError(w, err)
			return
		}
		if !ok {
			writeError(w, newAppError(http.StatusNotFound, "公告不存在"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) signAnnouncement(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID string `json:"userId"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	if strings.TrimSpace(input.UserID) == "" {
		writeError(w, newAppError(http.StatusBadRequest, "缺少用户信息"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	if err := content.NewRepository(r.db).SignAnnouncement(ctx, strings.Trim(id, "/"), strings.TrimSpace(input.UserID)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"signed": true}})
}
