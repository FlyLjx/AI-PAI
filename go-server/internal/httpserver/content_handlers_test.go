package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aipi-go/internal/content"
)

func TestNormalizeAnnouncementInput(t *testing.T) {
	input := content.Announcement{
		Title:      "  维护通知  ",
		Content:    "  服务将在今晚维护。  ",
		TargetType: "users",
		UserIDs:    []string{" user-1 ", "user-1", "", "user-2"},
	}
	if err := normalizeAnnouncementInput(&input); err != nil {
		t.Fatalf("normalize announcement: %v", err)
	}
	if input.Title != "维护通知" || input.Content != "服务将在今晚维护。" {
		t.Fatalf("unexpected trimmed content: %#v", input)
	}
	if input.DisplayMode != "popup" || input.Status != "active" {
		t.Fatalf("unexpected defaults: %#v", input)
	}
	if len(input.UserIDs) != 2 || input.UserIDs[0] != "user-1" || input.UserIDs[1] != "user-2" {
		t.Fatalf("unexpected user ids: %#v", input.UserIDs)
	}
}

func TestNormalizeAnnouncementInputRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		input content.Announcement
	}{
		{name: "empty title", input: content.Announcement{Content: "内容"}},
		{name: "empty content", input: content.Announcement{Title: "标题"}},
		{name: "invalid display mode", input: content.Announcement{Title: "标题", Content: "内容", DisplayMode: "toast"}},
		{name: "invalid target type", input: content.Announcement{Title: "标题", Content: "内容", TargetType: "group"}},
		{name: "invalid status", input: content.Announcement{Title: "标题", Content: "内容", Status: "draft"}},
		{name: "missing target users", input: content.Announcement{Title: "标题", Content: "内容", TargetType: "users"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := normalizeAnnouncementInput(&test.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestAnnouncementRoutesRequireAuthentication(t *testing.T) {
	router := &Router{mux: http.NewServeMux()}
	router.routes()

	tests := []struct {
		path string
		want int
	}{
		{path: "/api/announcements", want: http.StatusUnauthorized},
		{path: "/api/announcements/public?userId=user-1", want: http.StatusUnauthorized},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		router.mux.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("%s returned %d, want %d", test.path, response.Code, test.want)
		}
	}
}
