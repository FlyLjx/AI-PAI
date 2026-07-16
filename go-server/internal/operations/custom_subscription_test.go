package operations

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGrantCustomSubscriptionCreatesEntitlementWithoutOrder(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	userID := "user-1"
	planID := adminCustomSubscriptionPlanID(userID)
	description := "管理员直接发放，不关联支付订单"
	badge := adminCustomSubscriptionBadge
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM users WHERE id=\? FOR UPDATE`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	mock.ExpectExec(`INSERT INTO subscription_plans`).
		WithArgs(planID, "合作客户额度", &description, 45, 1234, `[]`, `[]`, &badge).
		WillReturnResult(sqlmock.NewResult(1, 1))
	now := time.Now()
	mock.ExpectQuery(`SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent,`).
		WithArgs(planID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
			"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status", "created_at", "updated_at",
		}).AddRow(planID, "合作客户额度", description, 0, 45, 1234, 0, 0, `[]`, `[]`, badge, 0, "active", now, now))
	mock.ExpectQuery(`SELECT expires_at FROM user_subscriptions WHERE user_id=\? FOR UPDATE`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO user_subscriptions`).
		WithArgs(sqlmock.AnyArg(), userID, planID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = NewRepository(database.Wrap(rawDB)).GrantCustomSubscription(context.Background(), userID, CustomSubscriptionGrant{
		Name: "合作客户额度", DurationDays: 45, QuotaImages: 1234,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantCustomSubscriptionRejectsInvalidQuota(t *testing.T) {
	rawDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	err = NewRepository(database.Wrap(rawDB)).GrantCustomSubscription(context.Background(), "user-1", CustomSubscriptionGrant{
		DurationDays: 30,
		QuotaImages:  0,
	})
	if err != ErrInvalidCustomSubscription {
		t.Fatalf("error = %v, want ErrInvalidCustomSubscription", err)
	}
}

func TestPlansExcludeAdminCustomEntitlements(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(`SELECT .* FROM subscription_plans WHERE \(badge IS NULL OR badge <> \?\) AND status='active'`).
		WithArgs(adminCustomSubscriptionBadge).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
			"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status", "created_at", "updated_at",
		}))

	plans, err := NewRepository(database.Wrap(rawDB)).Plans(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 0 {
		t.Fatalf("plans = %d, want 0", len(plans))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentSubscriptionMarksAdminCustomSource(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectQuery(`SELECT user_subscriptions.id, user_subscriptions.status, user_subscriptions.started_at, user_subscriptions.expires_at`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"subscription_id", "subscription_status", "started_at", "expires_at",
			"plan_snapshot",
			"plan_id", "plan_name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
			"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "plan_status", "created_at", "updated_at",
		}).AddRow(
			"subscription-1", "active", now, now.AddDate(0, 1, 0),
			nil,
			"plan-custom", "合作客户额度", nil, 0, 30, 1000, 0, 0,
			`[]`, `[]`, adminCustomSubscriptionBadge, 0, "active", now, now,
		))

	entitlement, err := NewRepository(database.Wrap(rawDB)).currentPaidSubscription(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if entitlement.Source != "admin_custom" || entitlement.Tier != "custom" {
		t.Fatalf("source=%q tier=%q, want admin_custom/custom", entitlement.Source, entitlement.Tier)
	}
	if entitlement.plan.Badge != nil {
		t.Fatal("internal custom badge must not be exposed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
