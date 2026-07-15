package tasks

import (
	"context"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestClaimForProcessingClaimsQueuedTaskOnce(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	repo := NewRepository(database.Wrap(rawDB))

	mock.ExpectExec(`UPDATE generation_tasks SET status = 'processing' WHERE id = \? AND status IN \('queued', 'pending'\)`).
		WithArgs("task-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	claimed, err := repo.ClaimForProcessing(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("expected queued task to be claimed")
	}

	mock.ExpectExec(`UPDATE generation_tasks SET status = 'processing' WHERE id = \? AND status IN \('queued', 'pending'\)`).
		WithArgs("task-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	claimed, err = repo.ClaimForProcessing(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		t.Fatal("expected an already claimed task to be skipped")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
