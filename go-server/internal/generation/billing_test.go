package generation

import (
	"context"
	"errors"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRemainingBalanceAfterChargeRoundsToDatabasePrecision(t *testing.T) {
	remaining, ok := remainingBalanceAfterCharge(0.3, 0.1*3)
	if !ok {
		t.Fatal("expected the rounded balance to cover the rounded charge")
	}
	if remaining != 0 {
		t.Fatalf("remaining balance = %v, want 0", remaining)
	}
}

func TestRemainingBalanceAfterChargeRejectsInsufficientBalance(t *testing.T) {
	remaining, ok := remainingBalanceAfterCharge(0.2999, 0.3)
	if ok {
		t.Fatal("expected insufficient balance")
	}
	if remaining != 0.2999 {
		t.Fatalf("remaining balance = %v, want unchanged balance", remaining)
	}
}

func TestFinishSuccessWithBillingUsesReservedTaskCost(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	service := &Service{db: database.Wrap(rawDB)}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id, status, cost_credits FROM generation_tasks WHERE id = \? FOR UPDATE`).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "status", "cost_credits"}).AddRow("user-1", "processing", 0.3))
	mock.ExpectQuery(`SELECT credits FROM users WHERE id = \? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(0.3))
	mock.ExpectExec(`UPDATE users SET credits = \? WHERE id = \?`).
		WithArgs(0.0, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).
		WithArgs(sqlmock.AnyArg(), "user-1", 0.3, 0.0, "API 调用：测试模型").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE generation_tasks SET status = 'success'`).
		WithArgs(1, 0.3, 0.12, 0.0, 2.0, `{"data":[]}`, "task-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = service.finishSuccessWithBilling(context.Background(), BillingSuccessInput{
		TaskID:           "task-1",
		Quantity:         1,
		ModelCostCredits: 0.12,
		DurationSeconds:  2,
		Remark:           "API 调用：测试模型",
		Result:           map[string]any{"data": []any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinishSuccessWithBillingIsIdempotentForSuccessfulTask(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	service := &Service{db: database.Wrap(rawDB)}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id, status, cost_credits FROM generation_tasks WHERE id = \? FOR UPDATE`).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "status", "cost_credits"}).AddRow("user-1", "success", 0.3))
	mock.ExpectCommit()

	if err := service.finishSuccessWithBilling(context.Background(), BillingSuccessInput{TaskID: "task-1"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinishSuccessWithBillingRollsBackWhenBalanceChanged(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	service := &Service{db: database.Wrap(rawDB)}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT user_id, status, cost_credits FROM generation_tasks WHERE id = \? FOR UPDATE`).
		WithArgs("task-1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "status", "cost_credits"}).AddRow("user-1", "processing", 0.3))
	mock.ExpectQuery(`SELECT credits FROM users WHERE id = \? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(0.2))
	mock.ExpectRollback()

	err = service.finishSuccessWithBilling(context.Background(), BillingSuccessInput{
		TaskID: "task-1",
		Result: map[string]any{"data": []any{}},
	})
	if !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("error = %v, want ErrInsufficientCredits", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
