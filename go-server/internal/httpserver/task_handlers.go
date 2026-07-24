package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"html"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/imagecache"
	"aipi-go/internal/tasks"

	_ "image/gif"
	_ "image/png"
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

func (r *Router) taskStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	stats, err := tasks.NewRepository(r.db).Stats(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": stats})
}

func (r *Router) estimateTaskDuration(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	quantity := queryInt(req, "quantity", 1)
	if quantity < 1 {
		quantity = 1
	}
	if quantity > 10 {
		quantity = 10
	}
	sizeTier := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("sizeTier")))
	baseSeconds := 35
	switch sizeTier {
	case "4k":
		baseSeconds = 75
	case "2k":
		baseSeconds = 50
	}
	estimatedSeconds := baseSeconds
	if quantity > 1 {
		estimatedSeconds += (quantity - 1) * 8
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"estimatedSeconds": estimatedSeconds,
			"minSeconds":       maxInt(10, estimatedSeconds-15),
			"maxSeconds":       estimatedSeconds + 30,
			"queueing":         false,
		},
	})
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

func (r *Router) exportTasks(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	items, err := tasks.NewRepository(r.db).FindAllForExport(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	filename := "tasks-" + time.Now().Format("2006-01-02") + ".xls"
	body := "\ufeff" + buildTaskExportHTML(items)
	w.Header().Set("Content-Type", "application/vnd.ms-excel; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len([]byte(body))))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func (r *Router) listTaskImages(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	page := queryInt(req, "page", 1)
	pageSize := queryInt(req, "pageSize", 24)
	input := tasks.ListInput{
		Page:     page,
		PageSize: pageSize,
		Keyword:  strings.TrimSpace(req.URL.Query().Get("keyword")),
		Display:  strings.TrimSpace(req.URL.Query().Get("display")),
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, total, err := tasks.NewRepository(r.db).FindImages(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeTaskPage(w, items, total, page, pageSize)
}

func buildTaskExportHTML(items []tasks.Task) string {
	headers := []string{
		"任务ID",
		"用户",
		"用户IP",
		"用途",
		"模型",
		"服务商",
		"规格",
		"分辨率",
		"数量",
		"用时(s)",
		"状态",
		"失败原因",
		"提示词",
		"创建时间",
		"更新时间",
	}
	var builder strings.Builder
	builder.WriteString(`<!doctype html><html><head><meta charset="utf-8" /></head><body><table border="1"><thead><tr>`)
	for _, header := range headers {
		builder.WriteString("<th>")
		builder.WriteString(html.EscapeString(header))
		builder.WriteString("</th>")
	}
	builder.WriteString("</tr></thead><tbody>")
	for index := range items {
		item := items[index]
		row := []string{
			item.ID,
			taskUserLabel(item),
			item.UserIP,
			capabilityLabel(item.Capability),
			taskModelLabel(item),
			taskProviderLabel(item),
			item.SizeTier,
			ptrStringValue(item.Size),
			strconv.Itoa(item.Quantity),
			formatFloat(item.DurationSeconds),
			taskStatusLabel(item.Status),
			ptrStringValue(item.ErrorMessage),
			item.Prompt,
			item.CreatedAt.Format(time.RFC3339),
			item.UpdatedAt.Format(time.RFC3339),
		}
		builder.WriteString("<tr>")
		for _, cell := range row {
			builder.WriteString("<td>")
			builder.WriteString(html.EscapeString(cell))
			builder.WriteString("</td>")
		}
		builder.WriteString("</tr>")
	}
	builder.WriteString("</tbody></table></body></html>")
	return builder.String()
}

func taskUserLabel(item tasks.Task) string {
	if item.UserEmail != nil && strings.TrimSpace(*item.UserEmail) != "" {
		return *item.UserEmail
	}
	return item.UserID
}

func taskModelLabel(item tasks.Task) string {
	if item.ModelName != nil && strings.TrimSpace(*item.ModelName) != "" {
		return *item.ModelName
	}
	if item.ModelDisplayName != nil && strings.TrimSpace(*item.ModelDisplayName) != "" {
		return *item.ModelDisplayName
	}
	return item.ModelID
}

func taskProviderLabel(item tasks.Task) string {
	if item.ProviderName != nil && strings.TrimSpace(*item.ProviderName) != "" {
		return *item.ProviderName
	}
	return item.ProviderID
}

func ptrStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func capabilityLabel(capability string) string {
	if capability == "chat_image" {
		return "对话生图"
	}
	return capability
}

func taskStatusLabel(status tasks.Status) string {
	switch status {
	case tasks.StatusQueued, tasks.StatusPending:
		return "等待中"
	case tasks.StatusProcessing:
		return "创作中"
	case tasks.StatusSuccess:
		return "成功"
	case tasks.StatusFailed:
		return "失败"
	case tasks.StatusCanceled:
		return "已取消"
	default:
		return string(status)
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (r *Router) checkTaskImage(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	taskID := strings.TrimSpace(req.URL.Query().Get("taskId"))
	index := queryInt(req, "index", 0)
	if taskID == "" {
		writeError(w, newAppError(http.StatusBadRequest, "缺少任务 ID"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	imageURL, err := tasks.NewRepository(r.db).ImageURLByIndex(ctx, taskID, index)
	if errors.Is(err, sql.ErrNoRows) || imageURL == "" {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"ok": false, "message": "图片跑丢了"}})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if _, ok := imagecache.FindTaskImage(taskID, index); ok {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"ok": true, "message": ""}})
		return
	}
	ok := probeImageURL(req.Context(), imageURL)
	message := ""
	if !ok {
		message = "图片跑丢了"
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"ok": ok, "message": message}})
}

func (r *Router) taskHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID := strings.TrimSpace(req.URL.Query().Get("userId"))
	if userID == "" {
		writeError(w, newAppError(http.StatusBadRequest, "缺少用户信息"))
		return
	}
	page := queryInt(req, "page", 1)
	pageSize := queryInt(req, "pageSize", 24)
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, total, err := tasks.NewRepository(r.db).FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}
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

func (r *Router) listFavorites(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID := strings.TrimSpace(req.URL.Query().Get("userId"))
	if userID == "" {
		writeError(w, newAppError(http.StatusBadRequest, "缺少用户信息"))
		return
	}
	page := queryInt(req, "page", 1)
	pageSize := queryInt(req, "pageSize", 24)
	input := tasks.ListInput{
		Page:     page,
		PageSize: pageSize,
		Keyword:  strings.TrimSpace(req.URL.Query().Get("keyword")),
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, total, err := tasks.NewRepository(r.db).FindFavoritesByUserID(ctx, userID, input)
	if err != nil {
		writeError(w, err)
		return
	}
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

func (r *Router) listPublicDisplay(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, err := tasks.NewRepository(r.db).FindPublicDisplay(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	data := make([]tasks.PublicTask, 0, len(items))
	for index := range items {
		data = append(data, tasks.ToPublic(&items[index]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
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
	if r.queue != nil {
		r.queue.Cancel(task.ID)
	}
	_ = apiaccess.NewRepository(r.db).FinishLogsForTask(ctx, task.ID, "canceled", 0, "任务已取消")
	if r.taskHub != nil {
		r.taskHub.PublishTask(*task)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tasks.ToPublic(task)})
}

func (r *Router) updateTaskDisplay(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPatch {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		DisplayEnabled bool   `json:"displayEnabled"`
		DisplayNote    string `json:"displayNote"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	id = strings.Trim(id, "/")
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	task, err := tasks.NewRepository(r.db).UpdateDisplay(ctx, id, input.DisplayEnabled, input.DisplayNote)
	if errors.Is(err, sql.ErrNoRows) || task == nil {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	if errors.Is(err, tasks.ErrNoResultImage) {
		writeError(w, newAppError(http.StatusBadRequest, err.Error()))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if r.taskHub != nil {
		r.taskHub.PublishTask(*task)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tasks.ToPublic(task)})
}

func (r *Router) reviewTaskPublic(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPatch {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Status      string `json:"status"`
		DisplayNote string `json:"displayNote"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	id = strings.Trim(id, "/")
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	task, err := tasks.NewRepository(r.db).ReviewPublic(ctx, id, strings.TrimSpace(input.Status), input.DisplayNote)
	if errors.Is(err, sql.ErrNoRows) || task == nil {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	if errors.Is(err, tasks.ErrNoResultImage) || errors.Is(err, tasks.ErrInvalidPublicStatus) {
		writeError(w, newAppError(http.StatusBadRequest, err.Error()))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if r.taskHub != nil {
		r.taskHub.PublishTask(*task)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tasks.ToPublic(task)})
}

func (r *Router) updateTaskFavorite(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPatch {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID          string `json:"userId"`
		FavoriteEnabled bool   `json:"favoriteEnabled"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	task, err := tasks.NewRepository(r.db).UpdateFavorite(ctx, strings.TrimSpace(id), strings.TrimSpace(input.UserID), input.FavoriteEnabled)
	if errors.Is(err, sql.ErrNoRows) || task == nil {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	if errors.Is(err, tasks.ErrForbiddenTask) {
		writeError(w, newAppError(http.StatusForbidden, err.Error()))
		return
	}
	if errors.Is(err, tasks.ErrNoResultImage) {
		writeError(w, newAppError(http.StatusBadRequest, err.Error()))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if r.taskHub != nil {
		r.taskHub.PublishTask(*task)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": tasks.ToPublic(task)})
}

func (r *Router) requestTaskPublic(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID      string `json:"userId"`
		DisplayNote string `json:"displayNote"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	task, err := tasks.NewRepository(r.db).RequestPublic(ctx, strings.TrimSpace(id), strings.TrimSpace(input.UserID), input.DisplayNote)
	if errors.Is(err, sql.ErrNoRows) || task == nil {
		writeError(w, newAppError(http.StatusNotFound, "任务不存在"))
		return
	}
	if errors.Is(err, tasks.ErrForbiddenTask) {
		writeError(w, newAppError(http.StatusForbidden, err.Error()))
		return
	}
	if errors.Is(err, tasks.ErrNoResultImage) {
		writeError(w, newAppError(http.StatusBadRequest, err.Error()))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if r.taskHub != nil {
		r.taskHub.PublishTask(*task)
	}
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
	if cached, ok := imagecache.FindTaskImage(parts[0], index); ok {
		r.serveCachedTaskImage(w, req, cached, len(parts) >= 4 && parts[3] == "download")
		return
	}
	if len(parts) >= 4 && parts[3] == "download" {
		r.proxyTaskImageDownload(w, req, imageURL)
		return
	}
	if len(parts) >= 3 && strings.Contains(path, "/images/"+parts[2]+"/download") {
		r.proxyTaskImageDownload(w, req, imageURL)
		return
	}
	r.proxyTaskImageInline(w, req, imageURL)
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
	ctx, cancel := context.WithTimeout(req.Context(), 20*time.Second)
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
	if cached, ok := imagecache.FindTaskImage(parts[0], index); ok {
		r.serveCachedTaskImage(w, req, cached, false)
		return
	}
	if err := r.proxyCompressedThumbnail(w, req, ctx, imageURL); err != nil {
		r.proxyTaskImageInline(w, req, imageURL)
	}
}

func (r *Router) proxyCompressedThumbnail(w http.ResponseWriter, req *http.Request, ctx context.Context, imageURL string) error {
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("upstream image unavailable")
	}
	source, _, err := image.Decode(io.LimitReader(resp.Body, 24*1024*1024))
	if err != nil {
		return err
	}
	maxSide := queryInt(req, "w", 720)
	if maxSide < 160 {
		maxSide = 160
	}
	if maxSide > 1600 {
		maxSide = 1600
	}
	quality := queryInt(req, "q", 72)
	if quality < 40 {
		quality = 40
	}
	if quality > 92 {
		quality = 92
	}
	output := resizeImageNearest(source, maxSide)
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("X-Aipi-Thumbnail", "compressed")
	return jpeg.Encode(w, output, &jpeg.Options{Quality: quality})
}

func resizeImageNearest(source image.Image, maxSide int) image.Image {
	bounds := source.Bounds()
	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()
	if sourceWidth <= 0 || sourceHeight <= 0 || maxInt(sourceWidth, sourceHeight) <= maxSide {
		return source
	}
	targetWidth := maxSide
	targetHeight := maxSide
	if sourceWidth >= sourceHeight {
		targetHeight = maxInt(1, sourceHeight*maxSide/sourceWidth)
	} else {
		targetWidth = maxInt(1, sourceWidth*maxSide/sourceHeight)
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		sourceY := bounds.Min.Y + y*sourceHeight/targetHeight
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + x*sourceWidth/targetWidth
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return target
}

func (r *Router) proxyTaskImageDownload(w http.ResponseWriter, req *http.Request, imageURL string) {
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := strings.TrimSpace(req.URL.Query().Get("filename"))
	if filename == "" {
		filename = "aipi-image"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filename, `"`, "")+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}

func (r *Router) serveCachedTaskImage(w http.ResponseWriter, req *http.Request, cached imagecache.CachedImage, download bool) {
	if info, err := os.Stat(cached.Path); err != nil || info.IsDir() || info.Size() <= 0 {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	contentType := strings.TrimSpace(cached.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	if download {
		filename := strings.TrimSpace(req.URL.Query().Get("filename"))
		if filename == "" {
			filename = "aipi-image"
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filename, `"`, "")+`"`)
	}
	http.ServeFile(w, req, cached.Path)
}

func (r *Router) proxyTaskImageInline(w http.ResponseWriter, req *http.Request, imageURL string) {
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, newAppError(http.StatusNotFound, "图片跑丢了"))
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}

func probeImageURL(ctx context.Context, imageURL string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, imageURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return true
		}
		if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusForbidden {
			return false
		}
	}
	getReq, err := http.NewRequestWithContext(probeCtx, http.MethodGet, imageURL, nil)
	if err != nil {
		return false
	}
	getReq.Header.Set("Range", "bytes=0-32")
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		return false
	}
	defer getResp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(getResp.Body, 64))
	return getResp.StatusCode >= 200 && getResp.StatusCode < 400
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
