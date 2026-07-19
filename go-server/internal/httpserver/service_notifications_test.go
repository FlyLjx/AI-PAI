package httpserver

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestServiceNotificationReserveHonorsCooldown(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	manager := &serviceNotificationManager{now: func() time.Time { return now }, sent: map[string]time.Time{}}
	if !manager.reserve("balance:user-1", 24*time.Hour) {
		t.Fatal("first reminder should be reserved")
	}
	if manager.reserve("balance:user-1", 24*time.Hour) {
		t.Fatal("duplicate reminder should be suppressed during cooldown")
	}
	now = now.Add(24*time.Hour + time.Second)
	if !manager.reserve("balance:user-1", 24*time.Hour) {
		t.Fatal("reminder should be allowed after cooldown")
	}
}

func TestNotificationActionURLUsesDeploymentOriginForLegacyLocalSetting(t *testing.T) {
	t.Setenv("APP_PUBLIC_ORIGIN", "https://ai.yccc.me/")
	got := notificationActionURL("http://localhost:5173", "/sys-admins/upstream-apis")
	if got != "https://ai.yccc.me/sys-admins/upstream-apis" {
		t.Fatalf("notificationActionURL() = %q", got)
	}
}

func TestNotificationActionURLKeepsExplicitPublicOrigin(t *testing.T) {
	t.Setenv("APP_PUBLIC_ORIGIN", "https://environment.example.com")
	got := notificationActionURL("https://portal.example.com/", "/recharge")
	if got != "https://portal.example.com/recharge" {
		t.Fatalf("notificationActionURL() = %q", got)
	}
}

func TestNotificationActionURLFallsBackToProductionDomain(t *testing.T) {
	t.Setenv("APP_PUBLIC_ORIGIN", "")
	got := notificationActionURL("http://localhost:5173", "/subscriptions")
	if got != "https://ai.yccc.me/subscriptions" {
		t.Fatalf("notificationActionURL() = %q", got)
	}
}

func TestNotificationActionURLRejectsLocalDeploymentOrigin(t *testing.T) {
	t.Setenv("APP_PUBLIC_ORIGIN", "http://127.0.0.1:6985")
	got := notificationActionURL("http://127.0.0.1:3000", "/sys-admins/recharges")
	if got != "https://ai.yccc.me/sys-admins/recharges" {
		t.Fatalf("notificationActionURL() = %q", got)
	}
}

func TestReminderAlreadySentReadsPersistentState(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	manager := newServiceNotificationManager(database.Wrap(rawDB), nil)
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	manager.now = func() time.Time { return now }

	mock.ExpectQuery(`SELECT setting_value`).
		WithArgs("notify.balance.user-1").
		WillReturnRows(sqlmock.NewRows([]string{"setting_value"}).AddRow(now.Add(-time.Hour).Format(time.RFC3339Nano)))
	alreadySent, err := manager.reminderAlreadySent(context.Background(), "notify.balance.user-1", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !alreadySent {
		t.Fatal("recent persistent reminder should suppress a duplicate")
	}

	mock.ExpectQuery(`SELECT setting_value`).
		WithArgs("notify.balance.user-1").
		WillReturnRows(sqlmock.NewRows([]string{"setting_value"}).AddRow(now.Add(-25 * time.Hour).Format(time.RFC3339Nano)))
	alreadySent, err = manager.reminderAlreadySent(context.Background(), "notify.balance.user-1", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if alreadySent {
		t.Fatal("expired persistent reminder should allow another email")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSendBalanceReminderUsesExistingSMTPSettings(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	db := database.Wrap(rawDB)
	manager := newServiceNotificationManager(db, nil)

	mock.ExpectQuery(`SELECT setting_value`).
		WithArgs("notify.balance.user-1").
		WillReturnRows(sqlmock.NewRows([]string{"setting_value"}))
	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).
		WillReturnRows(notificationSettingRows("https://portal.example.com"))
	mock.ExpectQuery(`SELECT email, credits, status`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"email", "credits", "status"}).AddRow("user@example.com", 0.25, "active"))

	var recipient, subject, body string
	var action mailAction
	manager.sendMail = func(_ smtpSettings, to string, title string, text string, actions ...mailAction) error {
		recipient, subject, body = to, title, text
		if len(actions) > 0 {
			action = actions[0]
		}
		return nil
	}
	expectTrackedMailSuccess(mock, "balance_reminder", "user@example.com")
	mock.ExpectExec(`INSERT INTO system_settings`).
		WithArgs("notify.balance.user-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := manager.sendBalanceReminder(context.Background(), "user-1"); err != nil {
		t.Fatal(err)
	}
	if recipient != "user@example.com" || !strings.Contains(subject, "余额不足") || !strings.Contains(body, "0.25") {
		t.Fatalf("unexpected reminder: recipient=%q subject=%q body=%q", recipient, subject, body)
	}
	if action.Text != "立即充值" || action.URL != "https://portal.example.com/recharge" {
		t.Fatalf("unexpected action: %#v", action)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSendSubscriptionExpiryRemindersUsesPlanSnapshot(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	db := database.Wrap(rawDB)
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	expiresAt := now.Add(48 * time.Hour)
	manager := newServiceNotificationManager(db, nil)
	manager.now = func() time.Time { return now }

	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).
		WillReturnRows(notificationSettingRows("https://portal.example.com/"))
	mock.ExpectQuery(`SELECT user_subscriptions.id, user_subscriptions.user_id, users.email`).
		WithArgs(now, now.Add(subscriptionExpiryWindow)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "email", "expires_at", "plan_snapshot", "plan_name"}).
			AddRow("subscription-1", "user-1", "user@example.com", expiresAt, `{"name":"专业订阅"}`, "已改名套餐"))
	persistentKey := fmt.Sprintf("notify.subscription.%s.%d", "subscription-1", expiresAt.Unix())
	mock.ExpectQuery(`SELECT setting_value`).
		WithArgs(persistentKey).
		WillReturnRows(sqlmock.NewRows([]string{"setting_value"}))

	var subject, body string
	var action mailAction
	manager.sendMail = func(_ smtpSettings, _ string, title string, text string, actions ...mailAction) error {
		subject, body = title, text
		if len(actions) > 0 {
			action = actions[0]
		}
		return nil
	}
	expectTrackedMailSuccess(mock, "subscription_expiry", "user@example.com")
	mock.ExpectExec(`INSERT INTO system_settings`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := manager.sendSubscriptionExpiryReminders(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(subject, "订阅即将到期") || !strings.Contains(body, "专业订阅") || !strings.Contains(body, "2 天内") {
		t.Fatalf("unexpected reminder: subject=%q body=%q", subject, body)
	}
	if action.Text != "查看订阅" || action.URL != "https://portal.example.com/subscriptions" {
		t.Fatalf("unexpected action: %#v", action)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func notificationSettingRows(frontendURL string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"setting_key", "setting_value"}).
		AddRow("siteName", "AI-PAI").
		AddRow("frontendUrl", frontendURL).
		AddRow("emailEnabled", "true").
		AddRow("emailHost", "smtp.example.com").
		AddRow("emailPort", "465").
		AddRow("emailSecure", "true").
		AddRow("emailUser", "sender@example.com").
		AddRow("emailPassword", "secret").
		AddRow("emailFromName", "AI-PAI").
		AddRow("emailFromAddress", "sender@example.com")
}

func expectTrackedMailSuccess(mock sqlmock.Sqlmock, category string, recipient string) {
	mock.ExpectExec(`INSERT INTO email_delivery_logs`).
		WithArgs(sqlmock.AnyArg(), category, "sender@example.com", recipient, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE email_delivery_logs`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
}
