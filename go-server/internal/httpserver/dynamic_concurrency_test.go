package httpserver

import (
	"database/sql/driver"
	"testing"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

type timeWithin struct {
	min time.Time
	max time.Time
}

func (matcher timeWithin) Match(value driver.Value) bool {
	actual, ok := value.(time.Time)
	return ok && !actual.Before(matcher.min) && !actual.After(matcher.max)
}

func TestDynamicAPIKeyConcurrencyLimitUsesDefaultRule(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).
		WillReturnRows(sqlmock.NewRows([]string{"setting_key", "setting_value"}))
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

func TestDynamicAPIKeyConcurrencyLimitUsesMinuteWindowAndCustomStep(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	settingsRows := sqlmock.NewRows([]string{"setting_key", "setting_value"}).
		AddRow("dynamicConcurrencyEnabled", "true").
		AddRow("dynamicConcurrencyWindowValue", "10").
		AddRow("dynamicConcurrencyWindowUnit", "minute").
		AddRow("dynamicConcurrencyRequestStep", "20").
		AddRow("dynamicConcurrencyIncrement", "3")
	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).WillReturnRows(settingsRows)
	startedAt := time.Now()
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).
		WithArgs("key-1", timeWithin{min: startedAt.Add(-10*time.Minute - time.Second), max: startedAt.Add(-10*time.Minute + time.Second)}).
		WillReturnRows(sqlmock.NewRows([]string{"request_count"}).AddRow(40))

	router := &Router{db: database.Wrap(rawDB)}
	if got := router.dynamicAPIKeyConcurrencyLimit(apiaccess.AccessKey{ID: "key-1", ConcurrencyLimit: 10}); got != 16 {
		t.Fatalf("dynamic concurrency = %d, want 16", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDynamicAPIKeyConcurrencyLimitCanBeDisabled(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).
		WillReturnRows(sqlmock.NewRows([]string{"setting_key", "setting_value"}).AddRow("dynamicConcurrencyEnabled", "false"))

	router := &Router{db: database.Wrap(rawDB)}
	if got := router.dynamicAPIKeyConcurrencyLimit(apiaccess.AccessKey{ID: "key-1", ConcurrencyLimit: 10}); got != 10 {
		t.Fatalf("dynamic concurrency = %d, want 10", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
