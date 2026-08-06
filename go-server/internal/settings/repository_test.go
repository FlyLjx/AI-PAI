package settings

import (
	"context"
	"errors"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpdateRejectsInvalidRechargeRate(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()
	_, err = NewRepository(database.Wrap(rawDB)).Update(context.Background(), Settings{"rechargeRate": float64(0)})
	if !errors.Is(err, ErrInvalidRechargeRate) {
		t.Fatalf("error = %v, want ErrInvalidRechargeRate", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRejectsInvalidTaskTimeout(t *testing.T) {
	for _, value := range []float64{0, 1.5, 121} {
		rawDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		mock.ExpectRollback()
		_, err = NewRepository(database.Wrap(rawDB)).Update(context.Background(), Settings{"taskTimeoutMinutes": value})
		if !errors.Is(err, ErrInvalidTaskTimeout) {
			t.Fatalf("value %v error = %v, want ErrInvalidTaskTimeout", value, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		rawDB.Close()
	}
}

func TestParseTaskTimeoutFallsBackToDefault(t *testing.T) {
	for _, value := range []string{"0", "1.5", "121", "invalid"} {
		if got := parseValue("taskTimeoutMinutes", value); got != Defaults["taskTimeoutMinutes"] {
			t.Fatalf("parseValue(taskTimeoutMinutes, %q) = %v, want %v", value, got, Defaults["taskTimeoutMinutes"])
		}
	}
	if got := parseValue("taskTimeoutMinutes", "10"); got != float64(10) {
		t.Fatalf("valid task timeout = %v, want 10", got)
	}
}

func TestParseInvalidRechargeRateFallsBackToDefault(t *testing.T) {
	for _, value := range []string{"0", "-1", "invalid"} {
		if got := parseValue("rechargeRate", value); got != float64(10) {
			t.Fatalf("parseValue(rechargeRate, %q) = %v, want 10", value, got)
		}
	}
	if got := parseValue("rechargeRate", "12.5"); got != float64(12.5) {
		t.Fatalf("valid recharge rate = %v, want 12.5", got)
	}
}

func TestUpdateRejectsInvalidDynamicConcurrencySettings(t *testing.T) {
	tests := []Settings{
		{"dynamicConcurrencyWindowValue": float64(0)},
		{"dynamicConcurrencyWindowValue": float64(1.5)},
		{"dynamicConcurrencyWindowUnit": "day"},
		{"dynamicConcurrencyRequestStep": float64(-1)},
		{"dynamicConcurrencyIncrement": float64(0)},
		{"dynamicConcurrencyEnabled": "true"},
	}
	for _, input := range tests {
		rawDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		mock.ExpectRollback()
		_, err = NewRepository(database.Wrap(rawDB)).Update(context.Background(), input)
		if !errors.Is(err, ErrInvalidDynamicConcurrency) {
			t.Fatalf("input %#v error = %v, want ErrInvalidDynamicConcurrency", input, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		rawDB.Close()
	}
}

func TestParseInvalidDynamicConcurrencyFallsBackToDefaults(t *testing.T) {
	for key, value := range map[string]string{
		"dynamicConcurrencyWindowValue": "0",
		"dynamicConcurrencyWindowUnit":  "day",
		"dynamicConcurrencyRequestStep": "1.5",
		"dynamicConcurrencyIncrement":   "invalid",
	} {
		if got := parseValue(key, value); got != Defaults[key] {
			t.Fatalf("parseValue(%s, %q) = %v, want %v", key, value, got, Defaults[key])
		}
	}
}

func TestUpdateRejectsInvalidInviteSettings(t *testing.T) {
	tests := []Settings{
		{"inviteInviterRewardType": "coupon"},
		{"inviteInviteeRewardCredits": float64(-1)},
		{"inviteRechargeRebatePercent": float64(0)},
		{"inviteRechargeRebatePercent": float64(101)},
		{"inviteRechargeRebateEnabled": "true"},
		{"inviteRebateIncludeSubscriptions": float64(1)},
		{"inviteRiskMaxPerIP24h": float64(0)},
		{"registrationChallengeMinSeconds": float64(1.5)},
	}
	for _, input := range tests {
		rawDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		mock.ExpectRollback()
		_, err = NewRepository(database.Wrap(rawDB)).Update(context.Background(), input)
		if !errors.Is(err, ErrInvalidInviteSettings) {
			t.Fatalf("input %#v error = %v, want ErrInvalidInviteSettings", input, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		rawDB.Close()
	}
}

func TestParseInvalidInviteRechargeRebatePercentFallsBackToDefault(t *testing.T) {
	for _, value := range []string{"0", "101", "invalid"} {
		if got := parseValue("inviteRechargeRebatePercent", value); got != Defaults["inviteRechargeRebatePercent"] {
			t.Fatalf("parseValue(inviteRechargeRebatePercent, %q) = %v, want %v", value, got, Defaults["inviteRechargeRebatePercent"])
		}
	}
}

func TestUpdateRejectsInvalidAdminNotificationSettings(t *testing.T) {
	tests := []Settings{
		{"adminUpstreamCheckIntervalMinutes": float64(0)},
		{"adminUpstreamCheckIntervalMinutes": float64(1.5)},
		{"adminUpstreamCheckIntervalMinutes": float64(1441)},
		{"adminRechargeNotificationEnabled": "true"},
		{"adminUpstreamNotificationEnabled": float64(1)},
		{"barkEnabled": "true"},
		{"barkNotifyError": "true"},
		{"barkServerUrl": "ftp://bark.example.com"},
	}
	for _, input := range tests {
		rawDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		mock.ExpectRollback()
		_, err = NewRepository(database.Wrap(rawDB)).Update(context.Background(), input)
		if !errors.Is(err, ErrInvalidAdminNotification) {
			t.Fatalf("input %#v error = %v, want ErrInvalidAdminNotification", input, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		rawDB.Close()
	}
}

func TestParseBarkServerFallsBackToDefault(t *testing.T) {
	if got := parseValue("barkServerUrl", ""); got != Defaults["barkServerUrl"] {
		t.Fatalf("empty Bark server = %v, want %v", got, Defaults["barkServerUrl"])
	}
	if got := parseValue("barkServerUrl", "https://bark.example.com/"); got != "https://bark.example.com/" {
		t.Fatalf("Bark server = %v", got)
	}
}

func TestParseInvalidAdminNotificationIntervalFallsBackToDefault(t *testing.T) {
	for _, value := range []string{"0", "1.5", "1441", "invalid"} {
		if got := parseValue("adminUpstreamCheckIntervalMinutes", value); got != Defaults["adminUpstreamCheckIntervalMinutes"] {
			t.Fatalf("parseValue(adminUpstreamCheckIntervalMinutes, %q) = %v, want %v", value, got, Defaults["adminUpstreamCheckIntervalMinutes"])
		}
	}
}

func TestNormalizeNotificationEmails(t *testing.T) {
	got, err := NormalizeNotificationEmails("admin@example.com; finance@example.com\nADMIN@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "admin@example.com, finance@example.com" {
		t.Fatalf("normalized recipients = %q", got)
	}
	recipients, err := ParseNotificationEmails(got)
	if err != nil || len(recipients) != 2 || recipients[1] != "finance@example.com" {
		t.Fatalf("parsed recipients = %#v, error = %v", recipients, err)
	}
}

func TestNormalizeNotificationEmailsRejectsInvalidAddress(t *testing.T) {
	for _, value := range []string{"admin", "admin@example.com, not-an-email", "Admin <admin@example.com>"} {
		if _, err := NormalizeNotificationEmails(value); !errors.Is(err, ErrInvalidAdminNotification) {
			t.Fatalf("value %q error = %v, want ErrInvalidAdminNotification", value, err)
		}
	}
}

func TestUpdateRejectsInvalidSystemLogCleanupSettings(t *testing.T) {
	tests := []Settings{
		{"systemLogAutoCleanupEnabled": "true"},
		{"systemLogRetentionDays": float64(0)},
		{"systemLogRetentionDays": float64(1.5)},
		{"systemLogRetentionDays": float64(3651)},
		{"taskImageAutoCleanupEnabled": "true"},
		{"taskImageRetentionDays": float64(0)},
		{"taskImageRetentionDays": float64(1.5)},
		{"taskImageRetentionDays": float64(3651)},
	}
	for _, input := range tests {
		rawDB, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectBegin()
		mock.ExpectRollback()
		_, err = NewRepository(database.Wrap(rawDB)).Update(context.Background(), input)
		if !errors.Is(err, ErrInvalidSystemLogCleanup) {
			t.Fatalf("input %#v error = %v, want ErrInvalidSystemLogCleanup", input, err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		rawDB.Close()
	}
}

func TestParseInvalidSystemLogRetentionDaysFallsBackToDefault(t *testing.T) {
	for _, value := range []string{"0", "1.5", "3651", "invalid"} {
		if got := parseValue("systemLogRetentionDays", value); got != Defaults["systemLogRetentionDays"] {
			t.Fatalf("parseValue(systemLogRetentionDays, %q) = %v, want %v", value, got, Defaults["systemLogRetentionDays"])
		}
	}
	if got := parseValue("systemLogRetentionDays", "90"); got != float64(90) {
		t.Fatalf("valid retention days = %v, want 90", got)
	}
}

func TestParseInvalidTaskImageRetentionDaysFallsBackToDefault(t *testing.T) {
	for _, value := range []string{"0", "1.5", "3651", "invalid"} {
		if got := parseValue("taskImageRetentionDays", value); got != Defaults["taskImageRetentionDays"] {
			t.Fatalf("parseValue(taskImageRetentionDays, %q) = %v, want %v", value, got, Defaults["taskImageRetentionDays"])
		}
	}
	if got := parseValue("taskImageRetentionDays", "7"); got != float64(7) {
		t.Fatalf("valid task image retention days = %v, want 7", got)
	}
}
