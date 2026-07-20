package httpserver

import (
	"encoding/json"
	"testing"

	"aipi-go/internal/content"
	"aipi-go/internal/users"
)

func TestNormalizeMailBroadcastInput(t *testing.T) {
	input, err := normalizeMailBroadcastInput(mailBroadcastInput{
		Subject:    "  系统通知  ",
		Content:    "  正文内容  ",
		ActionText: "  查看详情  ",
		ActionURL:  "  https://portal.example.com/dashboard  ",
		TargetType: "specific",
		UserIDs:    []string{"user-1", " user-1 ", "", "user-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.Subject != "系统通知" || input.Content != "正文内容" {
		t.Fatalf("unexpected normalized content: %#v", input)
	}
	if len(input.UserIDs) != 2 || input.UserIDs[0] != "user-1" || input.UserIDs[1] != "user-2" {
		t.Fatalf("unexpected user ids: %#v", input.UserIDs)
	}
}

func TestNormalizeMailBroadcastInputRejectsIncompleteAction(t *testing.T) {
	_, err := normalizeMailBroadcastInput(mailBroadcastInput{
		Subject: "系统通知", Content: "正文内容", TargetType: "all", ActionText: "查看详情",
	})
	if err == nil {
		t.Fatal("expected incomplete action to be rejected")
	}
}

func TestValidateMailAudienceRejectsAdminLinkInBroadcast(t *testing.T) {
	err := validateMailAudience("broadcast", "用户通知", []mailAction{{
		Text: "查看详情", URL: "https://portal.example.com/sys-admins/users",
	}})
	if err == nil {
		t.Fatal("expected broadcast with admin link to be rejected")
	}
}

func TestSelectMailRecipientsAppliesAudienceAndRole(t *testing.T) {
	items := []users.User{
		{ID: "active", Email: "active@example.com", Role: "user", Status: "active"},
		{ID: "disabled", Email: "disabled@example.com", Role: "user", Status: "disabled"},
		{ID: "admin", Email: "admin@example.com", Role: "admin", Status: "active"},
		{ID: "duplicate", Email: "ACTIVE@example.com", Role: "user", Status: "active"},
	}
	active := selectMailRecipients(items, "active", nil)
	if len(active) != 1 || active[0] != "active@example.com" {
		t.Fatalf("unexpected active recipients: %#v", active)
	}
	specific := selectMailRecipients(items, "specific", []string{"disabled", "admin"})
	if len(specific) != 1 || specific[0] != "disabled@example.com" {
		t.Fatalf("unexpected specific recipients: %#v", specific)
	}
}

func TestAnnouncementMutationInputDecodesSendEmail(t *testing.T) {
	var input announcementMutationInput
	if err := json.Unmarshal([]byte(`{"title":"维护通知","content":"今晚维护","displayMode":"banner","targetType":"all","status":"active","sortOrder":10,"userIds":[],"sendEmail":true}`), &input); err != nil {
		t.Fatal(err)
	}
	if !input.SendEmail || input.Title != "维护通知" || input.DisplayMode != "banner" {
		t.Fatalf("unexpected announcement input: %#v", input)
	}
}

func TestAnnouncementMailBroadcastInputUsesSelectedUsers(t *testing.T) {
	input := announcementMailBroadcastInput(content.Announcement{
		Title: "维护通知", Content: "今晚维护", TargetType: "users", UserIDs: []string{"user-1"},
	})
	if input.Category != "announcement" || input.TargetType != "specific" || len(input.UserIDs) != 1 {
		t.Fatalf("unexpected announcement mail input: %#v", input)
	}
	if input.ActionPath != "/dashboard" || input.ActionText != "查看公告" {
		t.Fatalf("unexpected announcement action: %#v", input)
	}
}
