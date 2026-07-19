package httpserver

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"aipi-go/internal/content"
)

func (r *Router) publicAnnouncements(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID := strings.TrimSpace(req.URL.Query().Get("userId"))
	if userID != "" {
		var err error
		userID, err = r.requireFrontUser(req, userID)
		if err != nil {
			writeError(w, err)
			return
		}
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	includeSigned := req.URL.Query().Get("includeSigned") == "1" || req.URL.Query().Get("includeSigned") == "true"
	items, err := content.NewRepository(r.db).FindAnnouncements(ctx, true, userID, includeSigned)
	if err != nil {
		writeError(w, err)
		return
	}
	if userID == "" {
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
		if err := normalizeAnnouncementInput(&input); err != nil {
			writeError(w, err)
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
		if err := normalizeAnnouncementInput(&input); err != nil {
			writeError(w, err)
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
	userID, err := r.requireFrontUser(req, input.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	announcementID := strings.Trim(id, "/")
	if announcementID == "" || strings.Contains(announcementID, "/") {
		writeError(w, newAppError(http.StatusNotFound, "公告不存在"))
		return
	}
	repo := content.NewRepository(r.db)
	items, err := repo.FindAnnouncements(ctx, true, userID, true)
	if err != nil {
		writeError(w, err)
		return
	}
	allowed := slices.ContainsFunc(items, func(item content.Announcement) bool {
		return item.ID == announcementID && item.DisplayMode == "popup"
	})
	if !allowed {
		writeError(w, newAppError(http.StatusNotFound, "公告不存在或无需确认"))
		return
	}
	if err := repo.SignAnnouncement(ctx, announcementID, userID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"signed": true}})
}

func normalizeAnnouncementInput(input *content.Announcement) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Content = strings.TrimSpace(input.Content)
	input.DisplayMode = defaultString(strings.TrimSpace(input.DisplayMode), "popup")
	input.TargetType = defaultString(strings.TrimSpace(input.TargetType), "all")
	input.Status = defaultString(strings.TrimSpace(input.Status), "active")

	if input.Title == "" {
		return newAppError(http.StatusBadRequest, "请输入公告标题")
	}
	if len([]rune(input.Title)) > 120 {
		return newAppError(http.StatusBadRequest, "公告标题不能超过 120 个字符")
	}
	if input.Content == "" {
		return newAppError(http.StatusBadRequest, "请输入公告内容")
	}
	if input.DisplayMode != "popup" && input.DisplayMode != "banner" {
		return newAppError(http.StatusBadRequest, "公告展示方式不正确")
	}
	if input.TargetType != "all" && input.TargetType != "users" {
		return newAppError(http.StatusBadRequest, "公告接收范围不正确")
	}
	if input.Status != "active" && input.Status != "disabled" {
		return newAppError(http.StatusBadRequest, "公告状态不正确")
	}

	seen := make(map[string]struct{}, len(input.UserIDs))
	userIDs := make([]string, 0, len(input.UserIDs))
	for _, userID := range input.UserIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	input.UserIDs = userIDs
	if input.TargetType == "all" {
		input.UserIDs = []string{}
	} else if len(input.UserIDs) == 0 {
		return newAppError(http.StatusBadRequest, "请选择至少一个接收用户")
	}
	return nil
}
