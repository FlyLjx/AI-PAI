package operations

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSavePlanIgnoresDiscountPercent(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	description := "API subscription"
	badge := "Popular"
	input := SubscriptionPlan{
		ID:                 "plan-1",
		Name:               "30 day plan",
		Description:        &description,
		Amount:             21,
		DurationDays:       30,
		QuotaImages:        9999,
		BonusCredits:       50,
		DiscountPercent:    35,
		AllowedProviderIDs: []string{"provider-1"},
		AllowedModelIDs:    []string{"model-1"},
		Badge:              &badge,
		SortOrder:          2,
		Status:             "active",
	}

	mock.ExpectExec(`INSERT INTO subscription_plans`).
		WithArgs(
			"plan-1", "30 day plan", &description, float64(21), 30, 9999, float64(0), float64(0),
			`["provider-1"]`, `["model-1"]`, &badge, 2, "active",
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	now := time.Now()
	mock.ExpectQuery(`SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent, allowed_provider_ids, allowed_model_ids, badge, sort_order, status, created_at, updated_at FROM subscription_plans WHERE id=\? LIMIT 1`).
		WithArgs("plan-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
			"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status", "created_at", "updated_at",
		}).AddRow(
			"plan-1", "30 day plan", description, 21, 30, 9999, 0, 0,
			`["provider-1"]`, `["model-1"]`, badge, 2, "active", now, now,
		))

	saved, err := NewRepository(database.Wrap(rawDB)).SavePlan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DiscountPercent != 0 {
		t.Fatalf("expected saved discount percent 0, got %v", saved.DiscountPercent)
	}
	if saved.BonusCredits != 0 {
		t.Fatalf("expected saved bonus credits 0, got %v", saved.BonusCredits)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeletePlanSoftDisablesProduct(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectExec(`UPDATE subscription_plans`).
		WithArgs("plan-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	changed, err := NewRepository(database.Wrap(rawDB)).DeletePlan(context.Background(), "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected the plan to be disabled")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentSubscriptionUsesSnapshotAfterPlanChanges(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	snapshotPlan := SubscriptionPlan{
		ID:                 "plan-1",
		Name:               "购买时专业版",
		DurationDays:       30,
		QuotaImages:        1200,
		AllowedProviderIDs: []string{"provider-original"},
		AllowedModelIDs:    []string{"model-original"},
		Status:             "active",
	}
	snapshot, err := json.Marshal(snapshotPlan)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT user_subscriptions.id, user_subscriptions.status, user_subscriptions.started_at, user_subscriptions.expires_at`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"subscription_id", "subscription_status", "started_at", "expires_at", "plan_snapshot",
			"plan_id", "plan_name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
			"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "plan_status", "created_at", "updated_at",
		}).AddRow(
			"subscription-1", "active", now, now.AddDate(0, 1, 0), string(snapshot),
			"plan-1", "修改后的套餐", nil, 99, 30, 10, 0, 0,
			`["provider-new"]`, `["model-new"]`, nil, 0, "disabled", now, now,
		))

	entitlement, err := NewRepository(database.Wrap(rawDB)).currentPaidSubscription(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if entitlement.PlanName != "购买时专业版" || entitlement.plan.QuotaImages != 1200 {
		t.Fatalf("snapshot was not preserved: plan=%q quota=%d", entitlement.PlanName, entitlement.plan.QuotaImages)
	}
	if len(entitlement.AllowedModelIDs) != 1 || entitlement.AllowedModelIDs[0] != "model-original" {
		t.Fatalf("models = %#v, want original snapshot", entitlement.AllowedModelIDs)
	}
	if len(entitlement.AllowedProviderIDs) != 1 || entitlement.AllowedProviderIDs[0] != "provider-original" {
		t.Fatalf("providers = %#v, want original snapshot", entitlement.AllowedProviderIDs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindPlanMasksLegacyDiscountPercent(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectQuery(`SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent, allowed_provider_ids, allowed_model_ids, badge, sort_order, status, created_at, updated_at FROM subscription_plans WHERE id=\? LIMIT 1`).
		WithArgs("legacy-plan").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent",
			"allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status", "created_at", "updated_at",
		}).AddRow(
			"legacy-plan", "Legacy", nil, 9.9, 30, 100, 0, 60,
			`[]`, `[]`, nil, 1, "active", now, now,
		))

	plan, err := NewRepository(database.Wrap(rawDB)).FindPlan(context.Background(), "legacy-plan")
	if err != nil {
		t.Fatal(err)
	}
	if plan.DiscountPercent != 0 {
		t.Fatalf("expected legacy discount to be masked, got %v", plan.DiscountPercent)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
