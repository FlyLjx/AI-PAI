package tasks

import (
	"context"
	"testing"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFailTimedOutMarksActiveTasksFailed(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 19, 10, 0, 0, 0, appclock.ConfigureDefault())
	cutoff := now.Add(-5 * time.Minute)
	createdAt := now.Add(-6 * time.Minute)
	mock.ExpectQuery(`(?s)SELECT id, created_at.*status IN \('queued', 'pending', 'processing'\).*created_at <= \?.*LIMIT \?`).
		WithArgs(cutoff, 500).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("task-1", createdAt))
	mock.ExpectExec(`(?s)UPDATE generation_tasks.*status = 'failed'.*error_message = \?.*duration_seconds = \?.*WHERE id = \?.*created_at <= \?`).
		WithArgs("任务处理超时（超过 5 分钟）", 360.0, "task-1", cutoff).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ids, err := NewRepository(database.Wrap(rawDB)).FailTimedOut(context.Background(), cutoff, now, "任务处理超时（超过 5 分钟）", 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "task-1" {
		t.Fatalf("timed out task ids = %#v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
