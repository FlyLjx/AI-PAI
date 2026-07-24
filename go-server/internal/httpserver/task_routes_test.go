package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskMetadataRequiresAdmin(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/tasks/task-1", nil)
	response := httptest.NewRecorder()

	router.taskByID(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestRemovedTaskActionsAreNotDispatched(t *testing.T) {
	for _, item := range []struct {
		method string
		path   string
	}{
		{http.MethodPatch, "/api/tasks/task-1/favorite"},
		{http.MethodPost, "/api/tasks/task-1/public-request"},
		{http.MethodPatch, "/api/tasks/task-1/display"},
		{http.MethodPatch, "/api/tasks/task-1/public-review"},
	} {
		t.Run(item.path, func(t *testing.T) {
			req := httptest.NewRequest(item.method, "http://example.test"+item.path, nil)
			response := httptest.NewRecorder()
			(&Router{}).taskByID(response, req)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestTaskCancelRouteRequiresAdmin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/tasks/task-1/cancel", nil)
	response := httptest.NewRecorder()

	(&Router{}).taskByID(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestTaskImagePathRemainsPublic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/tasks/task-1/images/not-an-index", nil)
	response := httptest.NewRecorder()

	(&Router{}).taskByID(response, req)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want public image handler response %d", response.Code, http.StatusNotFound)
	}
}

func TestProxyTaskImageInlineStreamsUpstreamImage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/tasks/task-1/images/0", nil)
	response := httptest.NewRecorder()

	(&Router{}).proxyTaskImageInline(response, req, upstream.URL+"/generated/out.png")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", contentType)
	}
	if body := response.Body.String(); body != "png-bytes" {
		t.Fatalf("body = %q, want upstream image body", body)
	}
}
