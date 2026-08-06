package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendBarkNotificationPostsPushPayload(t *testing.T) {
	var received barkPushRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/push" {
			t.Fatalf("request = %s %s, want POST /push", req.Method, req.URL.Path)
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", req.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(req.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	defer server.Close()

	err := sendBarkNotification(context.Background(), barkSettings{
		Enabled:        true,
		ServerURL:      server.URL,
		DeviceKey:      "device-key",
		Group:          "AI-PAI",
		Sound:          "minuet",
		NotifyRecharge: true,
	}, "充值成功通知", "订单已到账", "https://ai.yccc.me/sys-admins/recharges")
	if err != nil {
		t.Fatal(err)
	}
	if received.DeviceKey != "device-key" || received.Title != "充值成功通知" || received.Body != "订单已到账" || received.Group != "AI-PAI" || received.Sound != "minuet" || received.Level != "active" || received.URL == "" {
		t.Fatalf("unexpected Bark payload: %#v", received)
	}
}

func TestAdminNotificationSendsBarkWhenSMTPIsDisabled(t *testing.T) {
	called := false
	manager := &serviceNotificationManager{
		sendBark: func(_ context.Context, config barkSettings, title string, body string, actionURL string) error {
			called = config.DeviceKey == "device-key" && title == "充值成功" && body == "订单内容" && actionURL == "https://example.test/recharges"
			return nil
		},
	}

	err := manager.sendAdminNotification(
		context.Background(),
		smtpSettings{},
		barkSettings{Enabled: true, DeviceKey: "device-key", ServerURL: "https://api.day.app", NotifyRecharge: true},
		"",
		"recharge_success",
		"充值成功",
		"订单内容",
		mailAction{Text: "查看", URL: "https://example.test/recharges"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("Bark sender was not called with the notification payload")
	}
}

func TestAdminNotificationContinuesEmailWhenBarkFails(t *testing.T) {
	barkCalled := false
	mailCalled := false
	manager := &serviceNotificationManager{
		sendBark: func(context.Context, barkSettings, string, string, string) error {
			barkCalled = true
			return errors.New("Bark unavailable")
		},
		sendMail: func(smtpSettings, string, string, string, ...mailAction) error {
			mailCalled = true
			return nil
		},
	}

	err := manager.sendAdminNotification(
		context.Background(),
		smtpSettings{Enabled: true, Host: "smtp.example.com", User: "sender@example.com", Password: "secret"},
		barkSettings{Enabled: true, DeviceKey: "device-key", ServerURL: "https://api.day.app", NotifyRecharge: true},
		"admin@example.com",
		"recharge_success",
		"充值成功",
		"订单内容",
	)
	if err == nil || !strings.Contains(err.Error(), "Bark unavailable") {
		t.Fatalf("error = %v, want Bark failure", err)
	}
	if !barkCalled || !mailCalled {
		t.Fatalf("channel calls: bark=%v mail=%v", barkCalled, mailCalled)
	}
}
