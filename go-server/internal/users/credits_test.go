package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSetCreditsUpdatesBalanceAndWritesAuditLog(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT credits FROM users WHERE id = \? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(10.0))
	mock.ExpectExec(`UPDATE users`).
		WithArgs(25.5, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).
		WithArgs(sqlmock.AnyArg(), "user-1", 15.5, 25.5, "管理员 admin-1：补发余额").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow("user-1", "user@example.com", nil, nil, nil, "hash", 25.5, "user", "active", now, now, now))

	updated, err := NewRepository(database.Wrap(rawDB)).SetCredits(context.Background(), "user-1", 25.5, "管理员 admin-1：补发余额")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Credits != 25.5 {
		t.Fatalf("credits = %v, want 25.5", updated.Credits)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSetCreditsRejectsInvalidBalance(t *testing.T) {
	rawDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	_, err = NewRepository(database.Wrap(rawDB)).SetCredits(context.Background(), "user-1", -0.0001, "invalid")
	if !errors.Is(err, ErrInvalidCredits) {
		t.Fatalf("error = %v, want ErrInvalidCredits", err)
	}
}
