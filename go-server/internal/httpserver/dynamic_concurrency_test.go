package httpserver

import (
	"testing"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDynamicAPIKeyConcurrencyLimitUsesRollingHourCount(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WithArgs("key-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"request_count"}).AddRow(100))

	router := &Router{db: database.Wrap(rawDB)}
	if got := router.dynamicAPIKeyConcurrencyLimit(apiaccess.AccessKey{ID: "key-1", ConcurrencyLimit: 10}); got != 20 {
		t.Fatalf("dynamic concurrency = %d, want 20", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
