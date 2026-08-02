package operations

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBuildOrderWhereSupportsAdminFilters(t *testing.T) {
	where, args := buildOrderWhere(PageInput{
		UserID:    " user-1 ",
		Status:    " PAID ",
		OrderType: " SUBSCRIPTION ",
		Keyword:   " Customer@Example.COM ",
	})
	for _, fragment := range []string{
		"recharge_orders.user_id=?",
		"recharge_orders.status=?",
		"recharge_orders.order_type=?",
		"recharge_orders.id",
		"users.email",
		"recharge_orders.out_trade_no",
		"recharge_orders.trade_no",
		"recharge_orders.subscription_plan_id",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where clause omitted %q: %s", fragment, where)
		}
	}
	if len(args) != 10 {
		t.Fatalf("args length = %d, want 10: %#v", len(args), args)
	}
	if args[0] != "user-1" || args[1] != "paid" || args[2] != "subscription" {
		t.Fatalf("exact filter args were not normalized: %#v", args[:3])
	}
	for _, arg := range args[3:] {
		if arg != "%customer@example.com%" {
			t.Fatalf("keyword pattern = %#v, want lowercase substring", arg)
		}
	}
}

func TestOrderSummaryUsesAllFilteredRowsWithoutPagination(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	args := []driver.Value{"paid", "subscription"}
	for range 7 {
		args = append(args, "%needle%")
	}
	mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\) AS total,.*paid_amount.*subscription_count.*FROM recharge_orders.*WHERE recharge_orders\.status=\? AND recharge_orders\.order_type=\?.*users\.email`).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"total", "paid_amount", "paid_count", "pending_count", "subscription_count"}).
			AddRow(42, 230.5, 38, 0, 42))

	summary, err := NewRepository(database.Wrap(rawDB)).OrderSummary(context.Background(), PageInput{
		Page: 999, PageSize: 1, Status: "paid", OrderType: "subscription", Keyword: "Needle",
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 42 || summary.PaidAmount != 230.5 || summary.PaidCount != 38 || summary.PendingCount != 0 || summary.SubscriptionCount != 42 {
		t.Fatalf("unexpected order summary: %+v", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminInviteSummaryUsesAllRowsWithoutPagination(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\) AS total,.*AS rewarded,.*AS pending,.*AS review,.*AS blocked.*FROM user_invites`).
		WillReturnRows(sqlmock.NewRows([]string{"total", "rewarded", "pending", "review", "blocked"}).AddRow(70, 51, 12, 4, 7))
	summary, err := NewRepository(database.Wrap(rawDB)).AdminInviteSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 70 || summary.Rewarded != 51 || summary.Pending != 12 || summary.Review != 4 || summary.Blocked != 7 {
		t.Fatalf("unexpected invite summary: %+v", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
