package operations

import (
	"context"
	"errors"
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

func TestCompleteOrderRejectsClosedOrder(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	mock.ExpectBegin()
	expectOrder(mock, "closed", "recharge", nil, 10, time.Now())
	mock.ExpectRollback()

	order, changed, err := NewRepository(database.Wrap(rawDB)).CompleteOrder(context.Background(), "out-1", "trade-1")
	if !errors.Is(err, ErrRechargeOrderClosed) {
		t.Fatalf("error = %v, want ErrRechargeOrderClosed", err)
	}
	if changed || order == nil || order.Status != "closed" {
		t.Fatalf("unexpected result: changed=%v order=%#v", changed, order)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
