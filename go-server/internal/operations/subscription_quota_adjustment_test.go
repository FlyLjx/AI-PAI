package operations

import (
	"context"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAdjustSubscriptionQuotaSetsExactRemainingAmount(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	startedAt := now.AddDate(0, 0, -10)
	expiresAt := now.AddDate(0, 0, 20)
	snapshot, err := encodeSubscriptionPlanSnapshot(&SubscriptionPlan{
		ID: "plan-1", Name: "专业版", DurationDays: 30, QuotaImages: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	desiredRemaining := 25

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT plan_id, plan_snapshot, status, started_at, expires_at FROM user_subscriptions WHERE user_id=\? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"plan_id", "plan_snapshot", "status", "started_at", "expires_at"}).
			AddRow("plan-1", snapshot, "active", startedAt, expiresAt))
	mock.ExpectQuery(`(?s)SELECT generation_tasks\.status,.*result_image_count.*FROM generation_tasks`).
		WithArgs("user-1", startedAt, expiresAt).
		WillReturnRows(sqlmock.NewRows([]string{"status", "quantity", "subscription_quota_units", "result_image_count", "result_json_fallback"}).
			AddRow("processing", 2, 2, 0, nil).
			AddRow("success", 3, 3, 2, nil))
	mock.ExpectExec(`UPDATE user_subscriptions SET plan_snapshot=\?, started_at=\?, updated_at=CURRENT_TIMESTAMP WHERE user_id=\?`).
		WithArgs(subscriptionSnapshotMatcher{quota: 35, name: "专业版"}, startedAt, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = NewRepository(database.Wrap(rawDB)).AdjustSubscriptionQuota(context.Background(), "user-1", SubscriptionQuotaAdjustment{
		QuotaRemaining: &desiredRemaining,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdjustSubscriptionQuotaResetsUsageAndKeepsExpiry(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	startedAt := now.AddDate(0, 0, -10)
	expiresAt := now.AddDate(0, 0, 20)
	snapshot, err := encodeSubscriptionPlanSnapshot(&SubscriptionPlan{
		ID: "plan-1", Name: "专业版", DurationDays: 30, QuotaImages: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	resetRemaining := 80

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT plan_id, plan_snapshot, status, started_at, expires_at FROM user_subscriptions WHERE user_id=\? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"plan_id", "plan_snapshot", "status", "started_at", "expires_at"}).
			AddRow("plan-1", snapshot, "active", startedAt, expiresAt))
	mock.ExpectExec(`UPDATE user_subscriptions SET plan_snapshot=\?, started_at=\?, updated_at=CURRENT_TIMESTAMP WHERE user_id=\?`).
		WithArgs(subscriptionSnapshotMatcher{quota: 80, name: "专业版"}, sqlmock.AnyArg(), "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = NewRepository(database.Wrap(rawDB)).AdjustSubscriptionQuota(context.Background(), "user-1", SubscriptionQuotaAdjustment{
		QuotaRemaining: &resetRemaining,
		ResetUsage:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdjustSubscriptionQuotaRejectsMissingAdjustment(t *testing.T) {
	err := NewRepository(nil).AdjustSubscriptionQuota(context.Background(), "user-1", SubscriptionQuotaAdjustment{})
	if err != ErrInvalidSubscriptionQuota {
		t.Fatalf("error = %v, want ErrInvalidSubscriptionQuota", err)
	}
}
