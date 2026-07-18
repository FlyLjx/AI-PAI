package apiaccess

import (
	"context"
	"math"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAdminOperationsAggregatesRankingAndActiveCalls(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.Local)
	startAt := time.Date(2026, 7, 18, 0, 0, 0, 0, time.Local)
	mock.ExpectQuery(`SELECT\s+api_access_logs.user_id`).
		WithArgs(startAt, 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "billing_mode", "request_count", "success_count", "failed_count",
			"image_count", "credits_spent", "average_duration_seconds", "last_request_at",
		}).AddRow("user-1", "one@example.com", "balance", 12, 10, 2, 15, 6.5, 42.5, now.Add(-time.Minute)))
	mock.ExpectQuery(`SELECT\s+api_access_logs.id`).
		WillReturnRows(sqlmock.NewRows([]string{
			"log_id", "task_id", "user_id", "email", "api_key_id", "key_name", "key_prefix", "billing_mode",
			"concurrency_limit", "model", "size_tier", "size", "quantity", "status", "created_at",
		}).
			AddRow("log-1", "task-1", "user-1", "one@example.com", "key-1", "main", "sk-aipai-1", "balance", 10, "Image One", "1k", "1024x1024", 1, "processing", now.Add(-130*time.Second)).
			AddRow("log-2", "task-2", "user-1", "one@example.com", "key-1", "main", "sk-aipai-1", "balance", 10, "Image Two", "2k", "2048x2048", 1, "queued", now.Add(-10*time.Second)))

	snapshot, err := NewRepository(database.Wrap(rawDB)).AdminOperations(context.Background(), startAt, now, "today", "requests", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.TopUsers) != 1 || snapshot.TopUsers[0].RequestCount != 12 || math.Abs(snapshot.TopUsers[0].SuccessRate-1000.0/12.0) > 0.0001 {
		t.Fatalf("unexpected top users: %+v", snapshot.TopUsers)
	}
	if snapshot.ActiveUsers != 1 || snapshot.ActiveRequests != 2 || snapshot.ProcessingRequests != 1 || snapshot.QueuedRequests != 1 || snapshot.SlowRequests != 1 {
		t.Fatalf("unexpected active summary: %+v", snapshot)
	}
	if snapshot.ActiveCalls[0].ActiveForKey != 2 || snapshot.ActiveCalls[0].ElapsedSeconds != 130 {
		t.Fatalf("unexpected active call: %+v", snapshot.ActiveCalls[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeAdminOperationsMetric(t *testing.T) {
	if got := normalizeAdminOperationsMetric("credits"); got != "credits" {
		t.Fatalf("metric = %q, want credits", got)
	}
	if got := normalizeAdminOperationsMetric("unknown"); got != "requests" {
		t.Fatalf("metric = %q, want requests", got)
	}
}
