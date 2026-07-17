package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCompatBalanceReturnsAuthenticatedUserBalance(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	rawKey := "sk-aipai-balance-test-secret"
	prefix := apiaccess.KeyPrefix(rawKey)
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.Local)
	mock.ExpectQuery(`FROM api_access_keys`).
		WithArgs(prefix).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_email", "name", "key_prefix", "key_hash", "key_plain",
			"status", "concurrency_limit", "billing_mode", "last_used_at", "deleted_at",
			"created_at", "updated_at", "request_count", "success_count", "failed_count", "image_count", "last_error",
		}).AddRow(
			"key-1", "user-1", "user@example.com", "default", prefix, apiaccess.HashKey(rawKey), rawKey,
			"active", 10, apiaccess.BillingModeBalance, nil, nil,
			now, now, 0, 0, 0, 0, nil,
		))
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow("user-1", "user@example.com", nil, nil, nil, "hash", 128.75, "user", "active", now, now, now))
	mock.ExpectExec(`UPDATE api_access_keys SET last_used_at`).
		WithArgs("key-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	router := &Router{db: database.Wrap(rawDB)}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/v1/balance", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	recorder := httptest.NewRecorder()

	router.compatBalance(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Object      string  `json:"object"`
		Balance     float64 `json:"balance"`
		Unit        string  `json:"unit"`
		BillingMode string  `json:"billing_mode"`
		UpdatedAt   string  `json:"updated_at"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Object != "balance" || response.Balance != 128.75 || response.Unit != "credits" || response.BillingMode != apiaccess.BillingModeBalance {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.UpdatedAt == "" {
		t.Fatal("updated_at is empty")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompatBalanceRequiresGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.test/v1/balance", nil)
	recorder := httptest.NewRecorder()

	new(Router).compatBalance(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}
