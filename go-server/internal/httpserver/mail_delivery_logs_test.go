package httpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDeliverTrackedMailRecordsSuccess(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	expectTrackedMailSuccess(mock, "smtp_test", "admin@example.com")
	called := false
	sender := func(_ smtpSettings, to string, subject string, content string, actions ...mailAction) error {
		called = to == "admin@example.com" && subject == "测试邮件" && content == "正文" && len(actions) == 1
		return nil
	}
	err = deliverTrackedMail(
		context.Background(),
		database.Wrap(rawDB),
		sender,
		smtpSettings{User: "sender@example.com"},
		"smtp_test",
		"admin@example.com",
		"测试邮件",
		"正文",
		mailAction{Text: "查看", URL: "https://example.com"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("mail sender did not receive the expected payload")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeliverTrackedMailRecordsFailure(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectExec(`INSERT INTO email_delivery_logs`).
		WithArgs(sqlmock.AnyArg(), "upstream_alert", "sender@example.com", "admin@example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE email_delivery_logs`).
		WithArgs("smtp unavailable", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	wantErr := errors.New("smtp unavailable")
	err = deliverTrackedMail(
		context.Background(),
		database.Wrap(rawDB),
		func(smtpSettings, string, string, string, ...mailAction) error { return wantErr },
		smtpSettings{User: "sender@example.com"},
		"upstream_alert",
		"admin@example.com",
		"上游异常",
		"连接失败",
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeliverTrackedMailBlocksAdminRouteInUserMail(t *testing.T) {
	called := false
	err := deliverTrackedMail(
		context.Background(),
		nil,
		func(smtpSettings, string, string, string, ...mailAction) error {
			called = true
			return nil
		},
		smtpSettings{User: "sender@example.com"},
		"balance_reminder",
		"user@example.com",
		"余额不足",
		"请及时充值。",
		mailAction{Text: "查看", URL: "https://ai.yccc.me/sys-admins/recharges"},
	)
	if err == nil {
		t.Fatal("expected user mail with admin route to be rejected")
	}
	if called {
		t.Fatal("mail sender should not be called for a rejected user mail")
	}
}

func TestDeliverTrackedMailAllowsAdminRouteInAdminNotification(t *testing.T) {
	called := false
	err := deliverTrackedMail(
		context.Background(),
		nil,
		func(smtpSettings, string, string, string, ...mailAction) error {
			called = true
			return nil
		},
		smtpSettings{User: "sender@example.com"},
		"recharge_success",
		"admin@example.com",
		"充值成功",
		"系统收到一笔订单。",
		mailAction{Text: "查看充值流水", URL: "https://ai.yccc.me/sys-admins/recharges"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("admin notification should be sent")
	}
}

func TestContainsAdminRouteDetectsEncodedPath(t *testing.T) {
	if !containsAdminRoute("https://ai.yccc.me/redirect?next=%2Fsys-admins%2Fusers") {
		t.Fatal("encoded admin route should be detected")
	}
}

func TestUpstreamSnapshotHealth(t *testing.T) {
	tests := []struct {
		name     string
		snapshot map[string]any
		healthy  bool
	}{
		{name: "idle is healthy", snapshot: map[string]any{"reachable": true, "status": "idle", "stability_percent": float64(100)}, healthy: true},
		{name: "low stability is unhealthy", snapshot: map[string]any{"reachable": true, "status": "unstable", "stability_percent": float64(89)}, healthy: false},
		{name: "unreachable is unhealthy", snapshot: map[string]any{"reachable": false, "status": "unreachable", "stability_percent": float64(100)}, healthy: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			healthy, label := upstreamSnapshotHealth(test.snapshot)
			if healthy != test.healthy {
				t.Fatalf("healthy = %v, want %v; label = %q", healthy, test.healthy, label)
			}
			if strings.TrimSpace(label) == "" {
				t.Fatal("health label should not be empty")
			}
		})
	}
}
