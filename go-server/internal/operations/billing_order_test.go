package operations

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCompleteBalanceOrderCreditsUserExactlyOnce(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	repo := NewRepository(database.Wrap(rawDB))
	now := time.Now()

	mock.ExpectBegin()
	expectOrder(mock, "pending", "recharge", nil, 10, now)
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
		t.Fatalf("unexpected completion result: changed=%v order=%#v", changed, order)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteBalanceOrderSkipsAlreadyPaidOrder(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	repo := NewRepository(database.Wrap(rawDB))

	mock.ExpectBegin()
	expectOrder(mock, "paid", "recharge", nil, 10, time.Now())
	mock.ExpectCommit()

	_, changed, err := repo.CompleteOrder(context.Background(), "out-1", "trade-1")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected an already paid order not to credit the user again")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteSubscriptionOrderGrantsQuotaAndBonusBalance(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	repo := NewRepository(database.Wrap(rawDB))
	now := time.Now()

	mock.ExpectBegin()
	expectOrder(mock, "pending", "subscription", "plan-1", 2, now)
	mock.ExpectQuery(`SELECT credits FROM users WHERE id=\? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(5.0))
	mock.ExpectQuery(`SELECT name, duration_days FROM subscription_plans WHERE id=\?`).
		WithArgs("plan-1").
		WillReturnRows(sqlmock.NewRows([]string{"name", "duration_days"}).AddRow("专业版", 30))
	mock.ExpectQuery(`SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent,`).
		WithArgs("plan-1").
		WillReturnRows(testSubscriptionPlanRows(now, "plan-1", "专业版", 30, 1000, "active"))
	mock.ExpectQuery(`SELECT expires_at FROM user_subscriptions WHERE user_id=\? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO user_subscriptions`).
		WithArgs(sqlmock.AnyArg(), "user-1", "plan-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE users SET credits=\? WHERE id=\?`).
		WithArgs(7.0, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).
		WithArgs(sqlmock.AnyArg(), "user-1", 2.0, 7.0, "订阅套餐赠送 专业版").
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
		t.Fatalf("unexpected completion result: changed=%v order=%#v", changed, order)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func testSubscriptionPlanRows(now time.Time, id string, name string, durationDays int, quotaImages int, status string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
		"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status", "created_at", "updated_at",
	}).AddRow(id, name, nil, 10, durationDays, quotaImages, 0, 0, `[]`, `[]`, nil, 0, status, now, now)
}

func expectOrder(mock sqlmock.Sqlmock, status string, orderType string, planID any, credits float64, created time.Time) {
	mock.ExpectQuery(`SELECT id, user_id, NULL, out_trade_no, trade_no, order_type, subscription_plan_id, amount, credits, status, pay_url, qr_code, paid_at, created_at, updated_at FROM recharge_orders WHERE out_trade_no=\? FOR UPDATE`).
		WithArgs("out-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_email", "out_trade_no", "trade_no", "order_type", "subscription_plan_id",
			"amount", "credits", "status", "pay_url", "qr_code", "paid_at", "created_at", "updated_at",
		}).AddRow("order-1", "user-1", nil, "out-1", nil, orderType, planID, 10.0, credits, status, nil, nil, nil, created, created))
}
