package httpserver

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (r *Router) openNanaPrompts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	target, _ := url.Parse("https://api.opennana.com/api/prompts")
	query := target.Query()
	for key, values := range req.URL.Query() {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	if query.Get("limit") == "" {
		query.Set("limit", "20")
	}
	if query.Get("sort") == "" {
		query.Set("sort", "reviewed_at")
	}
	if query.Get("order") == "" {
		query.Set("order", "DESC")
	}
	if query.Get("model") == "" {
		query.Set("model", "ChatGPT")
	}
	target.RawQuery = query.Encode()
	proxyJSON(w, req, target.String(), 300)
}

func (r *Router) openNanaPrompt(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	slug := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/prompt-library/opennana/"), "/")
	if slug == "" {
		writeError(w, newAppError(http.StatusNotFound, "提示词不存在"))
		return
	}
	proxyJSON(w, req, "https://api.opennana.com/api/prompts/"+url.PathEscape(slug), 300)
}

func proxyJSON(w http.ResponseWriter, req *http.Request, target string, maxAge int) {
	client := http.Client{Timeout: 15 * time.Second}
	upstreamReq, err := http.NewRequestWithContext(req.Context(), http.MethodGet, target, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		writeError(w, newAppError(http.StatusBadGateway, "提示词库连接失败："+err.Error()))
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(maxAge))
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}
