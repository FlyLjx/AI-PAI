package settings

import (
	"context"
	"errors"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpdateRejectsInvalidRechargeRate(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()
	_, err = NewRepository(database.Wrap(rawDB)).Update(context.Background(), Settings{"rechargeRate": float64(0)})
	if !errors.Is(err, ErrInvalidRechargeRate) {
		t.Fatalf("error = %v, want ErrInvalidRechargeRate", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestParseInvalidRechargeRateFallsBackToDefault(t *testing.T) {
	for _, value := range []string{"0", "-1", "invalid"} {
		if got := parseValue("rechargeRate", value); got != float64(10) {
			t.Fatalf("parseValue(rechargeRate, %q) = %v, want 10", value, got)
		}
	}
	if got := parseValue("rechargeRate", "12.5"); got != float64(12.5) {
		t.Fatalf("valid recharge rate = %v, want 12.5", got)
	}
}
