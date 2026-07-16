package operations

import (
	"context"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDashboardTaskTrendFillsMissingDatesAndGroupsRunningStates(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	today := time.Now().In(time.Local)
	firstDate := today.AddDate(0, 0, -2).Format("2006-01-02")
	lastDate := today.Format("2006-01-02")
	mock.ExpectQuery(`SELECT DATE\(created_at\) AS task_date`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"task_date", "total", "queued", "pending", "processing", "success", "failed", "canceled",
		}).
			AddRow(firstDate, 8, 1, 1, 2, 3, 1, 0).
			AddRow(lastDate, 5, 0, 1, 0, 2, 1, 1))

	points, err := NewRepository(database.Wrap(rawDB)).DashboardTaskTrend(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 {
		t.Fatalf("points = %d, want 3", len(points))
	}
	if points[0].Date != firstDate || points[0].Running != 4 || points[0].Success != 3 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if points[1].Total != 0 || points[1].Running != 0 {
		t.Fatalf("missing date was not zero-filled: %+v", points[1])
	}
	if points[2].Date != lastDate || points[2].Canceled != 1 {
		t.Fatalf("unexpected last point: %+v", points[2])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
