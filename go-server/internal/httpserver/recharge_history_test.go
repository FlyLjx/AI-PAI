package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aipi-go/internal/auth"
	"aipi-go/internal/config"
	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRechargeHistoryReturnsOnlyAuthenticatedUserOrders(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow("user-1", "user@example.com", nil, nil, nil, "hash", 0, "user", "active", now, now, now))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id WHERE recharge_orders.user_id=\?`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT recharge_orders.id, .* FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id WHERE recharge_orders.user_id=\? ORDER BY recharge_orders.created_at DESC, recharge_orders.id DESC LIMIT \? OFFSET \?`).
		WithArgs("user-1", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "email", "out_trade_no", "trade_no", "order_type", "subscription_plan_id",
			"amount", "credits", "status", "pay_url", "qr_code", "paid_at", "created_at", "updated_at",
		}).AddRow(
			"order-1", "user-1", "user@example.com", "AIPI001", nil, "recharge", nil,
			10.0, 10.0, "pending", "https://pay.example.test", "https://qr.example.test", nil, now, now,
		))

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateUserToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/recharge/history?page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.rechargeHistory(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Data []struct {
			ID     string `json:"id"`
			UserID string `json:"userId"`
		} `json:"data"`
		Pagination struct {
			Total    int `json:"total"`
			Page     int `json:"page"`
			PageSize int `json:"pageSize"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "order-1" || response.Data[0].UserID != "user-1" {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
	if response.Pagination.Total != 1 || response.Pagination.Page != 1 || response.Pagination.PageSize != 10 {
		t.Fatalf("unexpected pagination: %+v", response.Pagination)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRechargeHistoryRequiresFrontUser(t *testing.T) {
	router := &Router{tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/recharge/history", nil)
	recorder := httptest.NewRecorder()

	router.rechargeHistory(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}
