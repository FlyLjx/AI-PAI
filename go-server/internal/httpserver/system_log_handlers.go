package httpserver

import (
	"errors"
	"net/http"
	"os"

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

func (r *Router) imageCleanupStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	if r.cleanupTracker == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"state": "idle"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": r.cleanupTracker.Snapshot()})
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
