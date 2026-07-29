package users

import (
	"context"
	"strings"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListCreditLogsResolvesAndRedactsLegacyAdminIdentifiers(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	const knownAdminID = "11111111-1111-4111-8111-111111111111"
	const missingAdminID = "99999999-9999-4999-8999-999999999999"
	now := time.Now().UTC().Truncate(time.Second)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM credit_logs WHERE user_id = \?`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`SELECT id, user_id, type, amount, balance_after, COALESCE\(remark, ''\), created_at`).
		WithArgs("user-1", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "type", "amount", "balance_after", "remark", "created_at"}).
			AddRow("known", "user-1", "manual_adjust", 5, 15, "管理员 "+knownAdminID+"：补发余额", now).
			AddRow("missing", "user-1", "manual_adjust", 2, 10, "管理员 "+missingAdminID+"：活动补贴", now).
			AddRow("deduct", "user-1", "deduct", 1, 8, "API 调用：gpt-image-2", now))
	mock.ExpectQuery(`SELECT id, email\s+FROM users\s+WHERE id IN \(\?, \?\)`).
		WithArgs(knownAdminID, missingAdminID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(knownAdminID, "admin@example.com"))

	items, total, err := NewRepository(database.Wrap(rawDB)).ListCreditLogs(context.Background(), "user-1", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("total=%d len=%d, want 3", total, len(items))
	}
	if items[0].Remark != "管理员 admin@example.com：补发余额" {
		t.Fatalf("resolved remark = %q", items[0].Remark)
	}
	if items[1].Remark != "系统管理员：活动补贴" {
		t.Fatalf("fallback remark = %q", items[1].Remark)
	}
	if items[2].Remark != "API 调用：gpt-image-2" {
		t.Fatalf("ordinary remark = %q", items[2].Remark)
	}
	for _, item := range items {
		if strings.Contains(item.Remark, knownAdminID) || strings.Contains(item.Remark, missingAdminID) {
			t.Fatalf("remark exposes internal identifier: %q", item.Remark)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSplitLegacyAdminCreditRemarkLeavesEmailUntouched(t *testing.T) {
	if _, _, ok := splitLegacyAdminCreditRemark("管理员 admin@example.com：补发余额"); ok {
		t.Fatal("email-based remark must not be treated as a legacy identifier")
	}
}
