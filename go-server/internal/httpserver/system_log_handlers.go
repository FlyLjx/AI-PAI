package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"aipi-go/internal/systemlogs"
)

func (r *Router) listSystemLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	files, err := systemlogs.New(r.cfg.LogDir).List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": files})
}

func (r *Router) systemLogDetail(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	detail, err := systemlogs.New(r.cfg.LogDir).Read(req.URL.Query().Get("name"), int64(queryInt(req, "maxBytes", 300000)))
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, newAppError(http.StatusNotFound, "日志文件不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (r *Router) deleteSystemLog(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	name := req.PathValue("name")
	if name == "" {
		name = req.URL.Path[len("/api/system-logs/"):]
	}
	result, err := systemlogs.New(r.cfg.LogDir).Delete(name)
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, newAppError(http.StatusNotFound, "日志文件不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (r *Router) systemLogStream(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, newAppError(http.StatusInternalServerError, "当前服务不支持日志流"))
		return
	}
	service := systemlogs.New(r.cfg.LogDir)
	name := req.URL.Query().Get("name")
	offset, _ := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	detail, err := service.ReadSince(name, offset, 200000)
	if errors.Is(err, os.ErrNotExist) {
		writeSSE(w, "ready", map[string]any{"name": name, "size": 0, "offset": 0})
		flusher.Flush()
		return
	}
	if err != nil {
		writeSSE(w, "error", map[string]any{"message": err.Error()})
		flusher.Flush()
		return
	}
	offset = detail.Offset
	writeSSE(w, "ready", map[string]any{"name": detail.Name, "size": detail.Size, "offset": offset})
	if detail.Content != "" {
		writeSSE(w, "append", detail)
	}
	flusher.Flush()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-req.Context().Done():
			return
		case <-ticker.C:
			next, err := service.ReadSince(name, offset, 200000)
			if err == nil && (next.Offset != offset || next.Content != "") {
				offset = next.Offset
				writeSSE(w, "append", next)
			} else {
				writeSSE(w, "ping", map[string]any{"offset": offset, "at": time.Now().Format(time.RFC3339)})
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, value any) {
	bytes, _ := json.Marshal(value)
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
}
