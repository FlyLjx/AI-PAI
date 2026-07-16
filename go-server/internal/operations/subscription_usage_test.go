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
	mock.ExpectQuery(`(?s)SELECT status, quantity, result_json.*NOT EXISTS.*api_access_logs.*api_access_keys.*billing_mode = 'balance'`).
		WithArgs("user-1", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"status", "quantity", "result_json"}).
			AddRow("processing", 2, nil).
			AddRow("success", 3, `{"data":[{"url":"https://cdn.example.test/one.png"},{"url":"https://cdn.example.test/two.png"}]}`))

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
