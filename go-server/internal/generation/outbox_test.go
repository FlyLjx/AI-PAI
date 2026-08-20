package generation

import (
	"context"
	"database/sql"
	"log/slog"
	"regexp"
	"testing"

	"aipi-go/internal/database"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestInsertOutboxWithTxPersistsCompleteJob(t *testing.T) {
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	db := database.Wrap(raw)
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO generation_outbox")).
		WithArgs("task-1", "task-1", taskShard("task-1", 64), "api-key:key-1", 1000, "url", "high", 100).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = InsertOutboxWithTx(context.Background(), tx, Job{
		TaskID: "task-1", ConcurrencyScope: "api-key:key-1", ConcurrencyLimit: 1000,
		ImageResponseFormat: "url", ImageQuality: "high", QueuePriority: 100,
	}, 64)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMarkOutboxPublishedUpdatesBatchInOneStatement(t *testing.T) {
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	db := database.Wrap(raw)
	queue := &Queue{service: NewService(db, slog.Default())}
	mock.ExpectExec(regexp.QuoteMeta("UPDATE generation_outbox")).
		WithArgs("task-1", "1-0", "task-2", "2-0", "task-1", "task-2").
		WillReturnResult(sqlmock.NewResult(0, 2))
	if err := queue.markOutboxPublished(context.Background(), []publishedOutboxRow{
		{TaskID: "task-1", MessageID: "1-0"},
		{TaskID: "task-2", MessageID: "2-0"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueueCompletionNotificationIsMultiplexed(t *testing.T) {
	queue := &Queue{waiters: map[string]map[chan struct{}]struct{}{}}
	completed, unsubscribe := queue.Subscribe("task-1")
	defer unsubscribe()
	queue.notifyWaiters("task-1")
	select {
	case <-completed:
	default:
		t.Fatal("completion notification was not delivered")
	}
}
