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
		{http.MethodPost, "/api/tasks/task-1/cancel"},
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

func TestTaskImagePathRemainsPublic(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/tasks/task-1/images/not-an-index", nil)
	response := httptest.NewRecorder()

	(&Router{}).taskByID(response, req)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want public image handler response %d", response.Code, http.StatusNotFound)
	}
}
