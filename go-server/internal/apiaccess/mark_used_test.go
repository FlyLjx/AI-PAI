package apiaccess

import (
	"context"
	"sync"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMarkUsedCoalescesConcurrentWritesForSameKey(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	keyID := "mark-used-concurrent-key"
	markUsedWrites.Lock()
	delete(markUsedWrites.last, keyID)
	markUsedWrites.Unlock()
	mock.ExpectExec(`UPDATE api_access_keys SET last_used_at`).
		WithArgs(keyID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repository := NewRepository(database.Wrap(rawDB))
	start := make(chan struct{})
	errors := make(chan error, 100)
	var workers sync.WaitGroup
	for range 100 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errors <- repository.MarkUsed(context.Background(), keyID)
		}()
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
