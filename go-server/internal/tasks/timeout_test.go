package tasks

import (
	"context"
	"testing"
	"time"
	"unicode/utf8"

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
	mock.ExpectQuery(`(?s)SELECT id, created_at, reference_image_url.*status IN \('queued', 'pending', 'processing'\).*updated_at <= \?.*LIMIT \?`).
		WithArgs(cutoff, 500).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "reference_image_url"}).AddRow("task-1", createdAt, nil))
	mock.ExpectExec(`(?s)UPDATE generation_tasks.*status = 'failed'.*error_message = \?.*duration_seconds = \?.*WHERE id = \?.*updated_at <= \?`).
		WithArgs("任务处理超时（超过 5 分钟）", 360.0, nil, "task-1", cutoff).
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

func TestFailTimedOutProcessingOnlyIgnoresWaitingTasks(t *testing.T) {
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
	mock.ExpectQuery(`(?s)SELECT id, created_at, reference_image_url.*status = 'processing'.*updated_at <= \?.*LIMIT \?`).
		WithArgs(cutoff, 500).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "reference_image_url"}))

	ids, err := NewRepository(database.Wrap(rawDB)).FailTimedOutProcessing(context.Background(), cutoff, now, "任务处理超时（超过 5 分钟）", 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("processing-only timeout ids = %#v", ids)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTruncateUTF8PreservesCompleteRunes(t *testing.T) {
	value := "上游返回错误"
	result := truncateUTF8(value, 5)
	if result != "上" {
		t.Fatalf("truncated value = %q, want %q", result, "上")
	}
	if !utf8.ValidString(result) {
		t.Fatalf("truncated value is not valid UTF-8: %q", result)
	}

	invalid := string([]byte{0xe6, 0xaf, 0x2e})
	if result := truncateUTF8(invalid, 8); !utf8.ValidString(result) {
		t.Fatalf("sanitized value is not valid UTF-8: %q", result)
	}
}
