package apiaccess

import (
	"context"
	"math"
	"strings"
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

func TestAdminOperationsRankingOnlyQueriesHistoricalLogs(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.Local)
	startAt := now.AddDate(0, 0, -6)
	mock.ExpectQuery(`SELECT\s+api_access_logs.user_id`).
		WithArgs(startAt, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "billing_mode", "request_count", "success_count", "failed_count",
			"image_count", "credits_spent", "average_duration_seconds", "last_request_at",
		}).AddRow("user-1", "one@example.com", "balance", 8, 7, 1, 9, 3.5, 25.0, now.Add(-time.Minute)))

	snapshot, err := NewRepository(database.Wrap(rawDB)).AdminOperationsRanking(context.Background(), startAt, now, "7d", "images", 5)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Range != "7d" || snapshot.Metric != "images" || len(snapshot.TopUsers) != 1 || snapshot.TopUsers[0].ImageCount != 9 {
		t.Fatalf("unexpected ranking snapshot: %+v", snapshot)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminOperationsRankingUsesPersistedLogBillingAndDurationFallback(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("postgres")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.Local)
	startAt := now.AddDate(0, 0, -30)
	mock.ExpectQuery(`(?s)SUM\(CASE WHEN api_access_logs\.status IN \('success', 'succeeded'\) THEN api_access_logs\.charged_credits ELSE 0 END\).*generation_tasks\.duration_seconds <= 300.*EXTRACT\(EPOCH FROM \(api_access_logs\.finished_at - api_access_logs\.created_at\)\) <= 300`).
		WithArgs(startAt, 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "billing_mode", "request_count", "success_count", "failed_count",
			"image_count", "credits_spent", "average_duration_seconds", "last_request_at",
		}).AddRow("user-1", "one@example.com", "balance", 12, 10, 2, 15, 6.5, 42.5, now.Add(-time.Minute)))

	snapshot, err := NewRepository(database.Wrap(rawDB)).AdminOperationsRanking(context.Background(), startAt, now, "30d", "credits", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.TopUsers) != 1 || snapshot.TopUsers[0].CreditsSpent != 6.5 || snapshot.TopUsers[0].AverageDurationSeconds != 42.5 {
		t.Fatalf("unexpected persisted ranking values: %+v", snapshot.TopUsers)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminOperationsLiveOnlyQueriesCurrentTasks(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.Local)
	mock.ExpectQuery(`SELECT\s+api_access_logs.id`).
		WillReturnRows(sqlmock.NewRows([]string{
			"log_id", "task_id", "user_id", "email", "api_key_id", "key_name", "key_prefix", "billing_mode",
			"concurrency_limit", "model", "size_tier", "size", "quantity", "status", "created_at",
		}).AddRow("log-1", "task-1", "user-1", "one@example.com", "key-1", "main", "sk-aipai-1", "balance", 10, "Image One", "1k", "1024x1024", 1, "processing", now.Add(-30*time.Second)))

	snapshot, err := NewRepository(database.Wrap(rawDB)).AdminOperationsLive(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ActiveUsers != 1 || snapshot.ActiveRequests != 1 || snapshot.ProcessingRequests != 1 || snapshot.AverageElapsedSeconds != 30 {
		t.Fatalf("unexpected live snapshot: %+v", snapshot)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminOperationsTrendFillsSixtyMinuteWindow(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 18, 12, 0, 30, 0, time.Local)
	startAt := time.Date(2026, 7, 18, 11, 1, 0, 0, time.Local)
	endAt := time.Date(2026, 7, 18, 12, 1, 0, 0, time.Local)
	mock.ExpectQuery(`(?s)SELECT\s+DATE_FORMAT.*AS minute_bucket.*FROM api_access_logs`).
		WithArgs(startAt, endAt).
		WillReturnRows(sqlmock.NewRows([]string{"minute_bucket", "total", "success", "failed"}).
			AddRow("2026-07-18 11:01", 2, 1, 1).
			AddRow("2026-07-18 12:00", 3, 2, 0))

	snapshot, err := NewRepository(database.Wrap(rawDB)).AdminOperationsTrend(context.Background(), now, 60)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Minutes != 60 || len(snapshot.Points) != 60 {
		t.Fatalf("unexpected trend size: %+v", snapshot)
	}
	if first := snapshot.Points[0]; first.Timestamp != startAt.Format(time.RFC3339) || first.Total != 2 || first.Success != 1 || first.Failed != 1 {
		t.Fatalf("unexpected first trend point: %+v", first)
	}
	if middle := snapshot.Points[1]; middle.Total != 0 || middle.Success != 0 || middle.Failed != 0 {
		t.Fatalf("missing minute should be zero-filled: %+v", middle)
	}
	if last := snapshot.Points[59]; last.Timestamp != now.Truncate(time.Minute).Format(time.RFC3339) || last.Total != 3 || last.Success != 2 || last.Failed != 0 {
		t.Fatalf("unexpected last trend point: %+v", last)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminOperationsTrendUsesDialectSpecificMinuteBucket(t *testing.T) {
	previousDialect := database.CurrentDialect()
	defer database.SetDialect(string(previousDialect))

	database.SetDialect("mysql")
	if expression := adminOperationsMinuteBucketExpression(); !strings.HasPrefix(expression, "DATE_FORMAT") {
		t.Fatalf("mysql expression = %q", expression)
	}
	database.SetDialect("postgres")
	if expression := adminOperationsMinuteBucketExpression(); !strings.Contains(expression, "DATE_TRUNC") {
		t.Fatalf("postgres expression = %q", expression)
	}
}

func TestAdminOperationsDurationUsesDialectSpecificExpression(t *testing.T) {
	previousDialect := database.CurrentDialect()
	defer database.SetDialect(string(previousDialect))

	database.SetDialect("mysql")
	if expression := adminOperationsLogDurationSecondsExpression(); !strings.Contains(expression, "TIMESTAMPDIFF") {
		t.Fatalf("mysql expression = %q", expression)
	}
	database.SetDialect("postgres")
	if expression := adminOperationsLogDurationSecondsExpression(); !strings.Contains(expression, "EXTRACT(EPOCH") {
		t.Fatalf("postgres expression = %q", expression)
	}
}
