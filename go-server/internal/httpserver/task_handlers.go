package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/tasks"
)

func (r *Router) taskByID(w http.ResponseWriter, req *http.Request) {
	path := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/tasks/"), "/")
	if strings.HasSuffix(path, "/cancel") {
		r.cancelTask(w, req, strings.TrimSuffix(path, "/cancel"))
		return
	}
	if strings.Contains(path, "/images/") {
		r.taskImage(w, req, path)
		return
	}
	if strings.Contains(path, "/thumbnails/") {
		r.taskThumbnail(w, req, strings.Replace(path, "/thumbnails/", "/images/", 1))
		return
	}
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	if path == "" || strings.Contains(path, "/") {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	task, err := tasks.NewRepository(r.db).FindByID(ctx, path)
	if errors.Is(err, sql.ErrNoRows) || task == nil {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tasks.ToPublic(task)})
}

func (r *Router) listTasks(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	page := queryInt(req, "page", 1)
	pageSize := queryInt(req, "pageSize", 20)
	input := tasks.ListInput{
		Page:     page,
		PageSize: pageSize,
		Keyword:  strings.TrimSpace(req.URL.Query().Get("keyword")),
		Status:   strings.TrimSpace(req.URL.Query().Get("status")),
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, total, err := tasks.NewRepository(r.db).FindAdminList(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": items,
		"pagination": map[string]any{
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

func (r *Router) cancelTask(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	task, err := tasks.NewRepository(r.db).Cancel(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if task == nil {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	if task.Status != tasks.StatusCanceled {
		writeError(w, newAppError(http.StatusConflict, "任务已经结束，不能取消"))
		return
	}
	// Cancellation releases any pending balance reservation. The reconcile is
	// idempotent and also handles a repeated cancel request safely.
	if err := tasks.NewRepository(r.db).ReconcileGenerationBalanceReservation(ctx, task.UserID); err != nil && r.logger != nil {
		r.logger.Warn("generation balance reservation reconcile failed", "taskId", task.ID, "userId", task.UserID, "error", err)
	}
	if err := tasks.NewRepository(r.db).ReleaseSubscriptionQuotaForTerminalTask(ctx, task.ID); err != nil && r.logger != nil {
		r.logger.Warn("generation subscription quota release failed", "taskId", task.ID, "error", err)
	}
	if r.queue != nil {
		r.queue.Cancel(task.ID)
	}
	_ = apiaccess.NewRepository(r.db).FinishLogsForTask(ctx, task.ID, "canceled", 0, "任务已取消")
	writeJSON(w, http.StatusOK, map[string]any{"data": tasks.ToPublic(task)})
}

func (r *Router) taskImage(w http.ResponseWriter, req *http.Request, path string) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "images" {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	imageURL, err := tasks.NewRepository(r.db).ImageURLByIndex(ctx, parts[0], index)
	if errors.Is(err, sql.ErrNoRows) || imageURL == "" {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, req, imageURL, http.StatusTemporaryRedirect)
}

func (r *Router) taskThumbnail(w http.ResponseWriter, req *http.Request, path string) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "images" {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	imageURL, err := tasks.NewRepository(r.db).ImageURLByIndex(ctx, parts[0], index)
	if errors.Is(err, sql.ErrNoRows) || imageURL == "" {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, req, imageURL, http.StatusTemporaryRedirect)
}

func writeTaskPage(w http.ResponseWriter, items []tasks.Task, total int, page int, pageSize int) {
	data := make([]tasks.PublicTask, 0, len(items))
	for index := range items {
		data = append(data, tasks.ToPublic(&items[index]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]any{
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}
