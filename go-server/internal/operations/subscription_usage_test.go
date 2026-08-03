package operations

import (
	"context"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGenerationUsageExcludesBalanceAPIKeyTasks(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	mock.ExpectQuery(`(?s)SELECT generation_tasks\.status,.*result_image_count.*NOT EXISTS.*api_access_logs.*api_access_keys.*billing_mode = 'balance'`).
		WithArgs("user-1", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"status", "quantity", "subscription_quota_units", "result_image_count", "result_json_fallback"}).
			AddRow("processing", 2, 1, 0, nil).
			AddRow("success", 3, 1, 2, nil))

	repo := NewRepository(database.Wrap(rawDB))
	used, err := repo.GenerationUsage(context.Background(), "user-1", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if used != 4 {
		t.Fatalf("subscription usage = %d, want 4", used)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationUsageAppliesStoredSubscriptionQuotaUnits(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	mock.ExpectQuery(`(?s)SELECT generation_tasks\.status,.*result_image_count.*FROM generation_tasks`).
		WithArgs("user-1", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"status", "quantity", "subscription_quota_units", "result_image_count", "result_json_fallback"}).
			AddRow("processing", 2, 2, 0, nil).
			AddRow("success", 3, 3, 2, nil))

	used, err := NewRepository(database.Wrap(rawDB)).GenerationUsage(context.Background(), "user-1", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if used != 10 {
		t.Fatalf("subscription usage = %d, want 10", used)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
