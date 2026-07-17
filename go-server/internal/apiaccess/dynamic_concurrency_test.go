package apiaccess

import (
	"context"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHourlyRequestCountsGroupsByAPIKey(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	since := time.Date(2026, 7, 17, 10, 0, 0, 0, time.Local)
	mock.ExpectQuery(`SELECT api_key_id, COUNT\(\*\)`).
		WithArgs(since, "key-1", "key-2").
		WillReturnRows(sqlmock.NewRows([]string{"api_key_id", "request_count"}).
			AddRow("key-1", 50).
			AddRow("key-2", 101))

	counts, err := NewRepository(database.Wrap(rawDB)).HourlyRequestCounts(context.Background(), []string{"key-1", "key-2", "key-1", ""}, since)
	if err != nil {
		t.Fatal(err)
	}
	if counts["key-1"] != 50 || counts["key-2"] != 101 {
		t.Fatalf("counts = %#v", counts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
