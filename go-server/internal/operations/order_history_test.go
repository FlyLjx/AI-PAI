package operations

import (
	"context"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOrdersFiltersByAuthenticatedUserAndSortsNewestFirst(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id WHERE recharge_orders.user_id=\? AND recharge_orders.status=\?`).
		WithArgs("user-1", "paid").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	now := time.Now()
	mock.ExpectQuery(`SELECT recharge_orders.id, .* FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id WHERE recharge_orders.user_id=\? AND recharge_orders.status=\? ORDER BY recharge_orders.created_at DESC, recharge_orders.id DESC LIMIT \? OFFSET \?`).
		WithArgs("user-1", "paid", 2, 2).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "email", "out_trade_no", "trade_no", "order_type", "subscription_plan_id",
			"amount", "credits", "status", "pay_url", "qr_code", "paid_at", "created_at", "updated_at",
		}).AddRow(
			"order-1", "user-1", "user@example.com", "AIPI001", "TRADE001", "subscription", "plan-1",
			19.9, 0, "paid", nil, nil, now, now, now,
		))

	items, total, err := NewRepository(database.Wrap(rawDB)).Orders(context.Background(), PageInput{
		Page: 2, PageSize: 2, Status: "paid", UserID: " user-1 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(items) != 1 || items[0].ID != "order-1" || items[0].UserID != "user-1" {
		t.Fatalf("unexpected orders: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
