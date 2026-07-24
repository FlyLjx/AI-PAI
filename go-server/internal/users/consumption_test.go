package users

import (
	"context"
	"strings"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestConsumptionRankingOrdersByCreditsSpent(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now().UTC().Truncate(time.Second)
	mock.ExpectQuery(`(?s)SELECT\s+credit_logs\.user_id,.*FROM credit_logs.*WHERE credit_logs\.type='deduct'.*COALESCE\(users\.role, 'user'\) <> 'admin'.*GROUP BY credit_logs\.user_id, users\.email, users\.status.*ORDER BY credits_spent DESC, deduct_count DESC, last_deduct_at DESC, credit_logs\.user_id ASC\s+LIMIT \?`).
		WithArgs(sqlmock.AnyArg(), 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "user_email", "user_status", "deduct_count", "credits_spent", "last_deduct_at",
		}).AddRow("user-1", "user@example.com", "active", 3, 9.5, now))

	items, err := NewRepository(database.Wrap(rawDB)).ConsumptionRanking(context.Background(), 30, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	item := items[0]
	if item.Rank != 1 || item.UserID != "user-1" || item.UserEmail != "user@example.com" || item.UserStatus != "active" || item.DeductCount != 3 || item.CreditsSpent != 9.5 || item.WindowDays != 30 {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.LastDeductAt == nil || !strings.HasSuffix(*item.LastDeductAt, "+08:00") {
		t.Fatalf("lastDeductAt = %+v, want +08:00 suffix", item.LastDeductAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
