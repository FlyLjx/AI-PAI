package operations

import (
	"context"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCloseExpiredPendingOrdersUsesThirtyMinuteCutoff(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.Local)
	mock.ExpectExec(`UPDATE recharge_orders SET status='closed', pay_url=NULL, qr_code=NULL, updated_at=CURRENT_TIMESTAMP WHERE status='pending' AND created_at <= \?`).
		WithArgs(now.Add(-30 * time.Minute)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	closed, err := NewRepository(database.Wrap(rawDB)).CloseExpiredPendingOrders(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if closed != 2 {
		t.Fatalf("closed = %d, want 2", closed)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteOrderCreditsClosedPaidOrder(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	repo := NewRepository(database.Wrap(rawDB))
	now := time.Now()

	mock.ExpectBegin()
	expectOrder(mock, "closed", "recharge", nil, 10, now)
	mock.ExpectQuery(`SELECT credits FROM users WHERE id=\? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(5.0))
	mock.ExpectExec(`UPDATE users SET credits=\? WHERE id=\?`).
		WithArgs(15.0, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).
		WithArgs(sqlmock.AnyArg(), "user-1", 10.0, 15.0, "支付宝充值 out-1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE recharge_orders SET status='paid', trade_no=\?, paid_at=\?, updated_at=CURRENT_TIMESTAMP WHERE id=\?`).
		WithArgs("trade-1", sqlmock.AnyArg(), "order-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	order, changed, err := repo.CompleteOrder(context.Background(), "out-1", "trade-1")
	if err != nil {
		t.Fatal(err)
	}
	if !changed || order == nil || order.Status != "paid" {
		t.Fatalf("unexpected result: changed=%v order=%#v", changed, order)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
