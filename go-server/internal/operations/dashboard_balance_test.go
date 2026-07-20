package operations

import (
	"context"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDashboardBalanceMetrics(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(`(?s)SELECT.*type='deduct'.*FROM users WHERE role <> 'admin'.*FROM credit_logs`).
		WillReturnRows(sqlmock.NewRows([]string{"today_consumed", "yesterday_consumed", "total_balance"}).
			AddRow(18.75, 12.5, 436.25))

	metrics, err := NewRepository(database.Wrap(rawDB)).dashboardBalanceMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if metrics.TodayConsumed != 18.75 || metrics.YesterdayConsumed != 12.5 || metrics.TotalBalance != 436.25 {
		t.Fatalf("unexpected balance metrics: %+v", metrics)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
