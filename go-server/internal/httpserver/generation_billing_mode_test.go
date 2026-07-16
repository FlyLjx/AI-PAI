package httpserver

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"aipi-go/internal/database"
	"aipi-go/internal/generation"
	"aipi-go/internal/models"
	"aipi-go/internal/operations"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNormalizeGenerationBillingMode(t *testing.T) {
	tests := map[string]string{
		"":             generationBillingModeAuto,
		" auto ":       generationBillingModeAuto,
		"SUBSCRIPTION": generationBillingModeSubscription,
		"balance":      generationBillingModeBalance,
		"other":        "",
	}
	for input, want := range tests {
		if got := normalizeGenerationBillingMode(input); got != want {
			t.Fatalf("normalizeGenerationBillingMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRequiredSubscriptionDoesNotFallBackWhenExpired(t *testing.T) {
	handled, err := generationSubscriptionBillingQuote(&operations.SubscriptionEntitlement{IsPaid: false}, models.Model{}, 1, true)
	if !handled {
		t.Fatal("expected subscription mode to handle an expired entitlement")
	}
	assertBillingAppErrorStatus(t, err, http.StatusPaymentRequired)
	if err.Error() != "订阅已到期或未开通，请续费后再调用" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestRequiredSubscriptionKeepsPermissionErrorAndMapsQuotaToPaymentRequired(t *testing.T) {
	model := models.Model{ID: "model-2", ProviderID: "provider-1"}
	entitlement := &operations.SubscriptionEntitlement{
		IsPaid:             true,
		QuotaRemaining:     1,
		AllowedProviderIDs: []string{"provider-1"},
		AllowedModelIDs:    []string{"model-1"},
	}
	handled, err := generationSubscriptionBillingQuote(entitlement, model, 1, true)
	if !handled {
		t.Fatal("expected subscription mode to handle the entitlement")
	}
	assertBillingAppErrorStatus(t, err, http.StatusForbidden)
	if err.Error() != "当前订阅套餐不支持该模型" {
		t.Fatalf("specific subscription error was lost: %q", err.Error())
	}

	entitlement.AllowedModelIDs = []string{"model-2"}
	_, err = generationSubscriptionBillingQuote(entitlement, model, 2, true)
	assertBillingAppErrorStatus(t, err, http.StatusPaymentRequired)
	if err.Error() != "本周期生成额度不足，请续费或升级订阅" {
		t.Fatalf("specific quota error was lost: %q", err.Error())
	}
}

func TestBalanceModeInsufficientMessageDoesNotSuggestSubscription(t *testing.T) {
	if got := generationBalanceInsufficientMessage(generationBillingModeBalance); got != "账户余额不足，请先充值" {
		t.Fatalf("balance mode message = %q", got)
	}
	if got := generationBalanceInsufficientMessage(generationBillingModeAuto); got != "账户余额不足，请充值或开通订阅" {
		t.Fatalf("auto mode message = %q", got)
	}
}

func TestBalanceBillingQuoteUsesOnlyAccountBalance(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	db := database.Wrap(rawDB)
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(`SELECT credits FROM users WHERE id = \?`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(10.0))
	mock.ExpectQuery(`(?s)SELECT COALESCE\(SUM\(cost_credits\), 0\).*FROM generation_tasks`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"reserved"}).AddRow(1.0))
	mock.ExpectRollback()

	router := &Router{db: db}
	cost, err := router.generationBillingQuote(
		context.Background(),
		tx,
		"user-1",
		models.Model{Price1K: 1.25},
		"1k",
		2,
		generationBillingModeBalance,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cost != 2.5 {
		t.Fatalf("balance quote = %v, want 2.5", cost)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAutoBillingKeepsLegacySubscriptionPermissionStatus(t *testing.T) {
	entitlement := &operations.SubscriptionEntitlement{
		IsPaid:             true,
		QuotaRemaining:     1,
		AllowedProviderIDs: []string{"provider-1"},
		AllowedModelIDs:    []string{"model-1"},
	}
	model := models.Model{ID: "model-2", ProviderID: "provider-1"}
	handled, err := generationSubscriptionBillingQuote(entitlement, model, 1, false)
	if !handled {
		t.Fatal("expected auto mode to use an active paid subscription")
	}
	assertBillingAppErrorStatus(t, err, http.StatusForbidden)
}

func TestCompatSettlementInsufficientCreditsReturnsQuotaError(t *testing.T) {
	status, errorType := compatTaskFailureResponse("任务结算失败：" + generation.ErrInsufficientCredits.Error())
	if status != http.StatusPaymentRequired || errorType != "insufficient_quota" {
		t.Fatalf("got status=%d type=%q", status, errorType)
	}
}

func assertBillingAppErrorStatus(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var appErr appError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected appError, got %T: %v", err, err)
	}
	if appErr.status != want {
		t.Fatalf("status = %d, want %d", appErr.status, want)
	}
}
