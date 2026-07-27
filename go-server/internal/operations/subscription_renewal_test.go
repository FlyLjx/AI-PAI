package operations

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGrantSubscriptionCarriesActiveRemainingQuota(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.Local)
	startedAt := now.AddDate(0, 0, -20)
	expiresAt := now.AddDate(0, 0, 10)
	oldSnapshot, err := encodeSubscriptionPlanSnapshot(&SubscriptionPlan{
		ID: "old-plan", Name: "旧套餐", DurationDays: 30, QuotaImages: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent,`).
		WithArgs("new-plan").
		WillReturnRows(testSubscriptionPlanRowsWithAccess(now, "new-plan", "新套餐", 30, 50, `["provider-new"]`, `["model-new"]`))
	mock.ExpectQuery(`SELECT plan_id, plan_snapshot, status, started_at, expires_at FROM user_subscriptions WHERE user_id=\? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"plan_id", "plan_snapshot", "status", "started_at", "expires_at"}).
			AddRow("old-plan", oldSnapshot, "active", startedAt, expiresAt))
	mock.ExpectQuery(`(?s)SELECT status, quantity, result_json, COALESCE\(subscription_quota_units, 1\).*FROM generation_tasks.*billing_mode = 'balance'`).
		WithArgs("user-1", startedAt, expiresAt).
		WillReturnRows(sqlmock.NewRows([]string{"status", "quantity", "result_json", "subscription_quota_units"}).
			AddRow("queued", 7, nil, 1).
			AddRow("success", 3, `{"data":[{"url":"https://cdn.example.test/one.png"},{"url":"https://cdn.example.test/two.png"}]}`, 1))
	mock.ExpectExec(`UPDATE user_subscriptions`).
		WithArgs("new-plan", subscriptionSnapshotMatcher{
			quota: 141, name: "新套餐", providerID: "provider-new", modelID: "model-new",
		}, now, expiresAt.AddDate(0, 0, 30), "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	db := database.Wrap(rawDB)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantSubscriptionInTx(context.Background(), tx, "user-1", "new-plan", 30, now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGrantSubscriptionDoesNotCarryExpiredQuota(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.Local)
	startedAt := now.AddDate(0, 0, -31)
	expiresAt := now.Add(-time.Hour)
	oldSnapshot, err := encodeSubscriptionPlanSnapshot(&SubscriptionPlan{
		ID: "old-plan", Name: "旧套餐", DurationDays: 30, QuotaImages: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent,`).
		WithArgs("new-plan").
		WillReturnRows(testSubscriptionPlanRowsWithAccess(now, "new-plan", "新套餐", 30, 50, `[]`, `[]`))
	mock.ExpectQuery(`SELECT plan_id, plan_snapshot, status, started_at, expires_at FROM user_subscriptions WHERE user_id=\? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"plan_id", "plan_snapshot", "status", "started_at", "expires_at"}).
			AddRow("old-plan", oldSnapshot, "active", startedAt, expiresAt))
	mock.ExpectExec(`UPDATE user_subscriptions`).
		WithArgs("new-plan", subscriptionSnapshotMatcher{quota: 50, name: "新套餐"}, now, now.AddDate(0, 0, 30), "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	db := database.Wrap(rawDB)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantSubscriptionInTx(context.Background(), tx, "user-1", "new-plan", 30, now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type subscriptionSnapshotMatcher struct {
	quota      int
	name       string
	providerID string
	modelID    string
}

func (matcher subscriptionSnapshotMatcher) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	var plan SubscriptionPlan
	if json.Unmarshal([]byte(text), &plan) != nil || plan.QuotaImages != matcher.quota || plan.Name != matcher.name {
		return false
	}
	if matcher.providerID != "" && !subscriptionSnapshotContains(plan.AllowedProviderIDs, matcher.providerID) {
		return false
	}
	return matcher.modelID == "" || subscriptionSnapshotContains(plan.AllowedModelIDs, matcher.modelID)
}

func subscriptionSnapshotContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func testSubscriptionPlanRowsWithAccess(now time.Time, id string, name string, durationDays int, quotaImages int, providerIDs string, modelIDs string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
		"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status", "created_at", "updated_at",
	}).AddRow(id, name, nil, 10, durationDays, quotaImages, 0, 0, providerIDs, modelIDs, nil, 0, "active", now, now)
}
