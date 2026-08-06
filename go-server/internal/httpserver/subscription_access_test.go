package httpserver

import (
	"errors"
	"net/http"
	"testing"

	"aipi-go/internal/settings"
)

func TestSubscriptionAccessAllowedOnlyForConfiguredUser(t *testing.T) {
	values := settings.Settings{"subscriptionAccessUserId": "user-1"}
	if !subscriptionAccessAllowed(values, "user-1") {
		t.Fatal("configured user should have subscription access")
	}
	if subscriptionAccessAllowed(values, "user-2") {
		t.Fatal("another user should not have subscription access")
	}
	if subscriptionAccessAllowed(settings.Settings{}, "user-1") {
		t.Fatal("empty configuration should keep subscription access closed")
	}
}

func TestSubscriptionAccessSettingIsNotPublic(t *testing.T) {
	if _, ok := settings.Public(settings.Defaults)["subscriptionAccessUserId"]; ok {
		t.Fatal("subscription access user id must remain admin-only")
	}
}

func TestSubscriptionAccessErrorUsesForbiddenStatus(t *testing.T) {
	var appErr appError
	err := newAppError(http.StatusForbidden, "当前账号暂未开放订阅功能")
	if !errors.As(err, &appErr) || appErr.status != http.StatusForbidden {
		t.Fatalf("error = %v, want forbidden app error", err)
	}
}
