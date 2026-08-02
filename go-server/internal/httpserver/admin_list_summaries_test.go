package httpserver

import (
	"database/sql/driver"
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

func TestRechargeOrdersReturnsFilteredFullSummary(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now().UTC()
	expectAdminBalanceUser(mock, "admin-1", "admin@example.com", 0, "admin", now)
	filterArgs := []driver.Value{"paid", "subscription"}
	for range 7 {
		filterArgs = append(filterArgs, "%customer@example.com%")
	}
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) FROM recharge_orders.*WHERE recharge_orders\.status=\? AND recharge_orders\.order_type=\?.*users\.email`).
		WithArgs(filterArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))
	listArgs := append(append([]driver.Value{}, filterArgs...), 30, 30)
	mock.ExpectQuery(`(?s)SELECT recharge_orders\.id,.*FROM recharge_orders.*WHERE recharge_orders\.status=\? AND recharge_orders\.order_type=\?.*users\.email.*LIMIT \? OFFSET \?`).
		WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\) AS total,.*FROM recharge_orders.*WHERE recharge_orders\.status=\? AND recharge_orders\.order_type=\?.*users\.email`).
		WithArgs(filterArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"total", "paid_amount", "paid_count", "pending_count", "subscription_count"}).
			AddRow(42, 230.5, 38, 0, 42))

	router := &Router{db: database.Wrap(rawDB), tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/recharge/orders?page=2&pageSize=30&keyword=Customer%40Example.COM&status=PAID&orderType=SUBSCRIPTION", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.rechargeOrders(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data       []json.RawMessage `json:"data"`
		Pagination struct {
			Total    int `json:"total"`
			Page     int `json:"page"`
			PageSize int `json:"pageSize"`
		} `json:"pagination"`
		Summary struct {
			Total             int     `json:"total"`
			PaidAmount        float64 `json:"paidAmount"`
			PaidCount         int     `json:"paidCount"`
			PendingCount      int     `json:"pendingCount"`
			SubscriptionCount int     `json:"subscriptionCount"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 0 || response.Pagination.Total != 42 || response.Pagination.Page != 2 || response.Pagination.PageSize != 30 {
		t.Fatalf("unexpected paginated response: data=%d pagination=%+v", len(response.Data), response.Pagination)
	}
	if response.Summary.Total != 42 || response.Summary.PaidAmount != 230.5 || response.Summary.PaidCount != 38 || response.Summary.PendingCount != 0 || response.Summary.SubscriptionCount != 42 {
		t.Fatalf("summary should describe all filtered rows: %+v", response.Summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInvitesReturnsFullSummary(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now().UTC()
	expectAdminBalanceUser(mock, "admin-1", "admin@example.com", 0, "admin", now)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_invites`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(70))
	mock.ExpectQuery(`(?s)SELECT user_invites\.id,.*FROM user_invites.*ORDER BY user_invites\.created_at DESC LIMIT \? OFFSET \?`).
		WithArgs(30, 30).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\) AS total,.*AS rewarded,.*AS pending,.*AS review,.*AS blocked.*FROM user_invites`).
		WillReturnRows(sqlmock.NewRows([]string{"total", "rewarded", "pending", "review", "blocked"}).AddRow(70, 51, 12, 4, 7))

	router := &Router{db: database.Wrap(rawDB), tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/invites?page=2&pageSize=30", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.invites(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data       []json.RawMessage `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
		Summary struct {
			Total    int `json:"total"`
			Rewarded int `json:"rewarded"`
			Pending  int `json:"pending"`
			Review   int `json:"review"`
			Blocked  int `json:"blocked"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Pagination.Total != 70 || response.Summary.Total != 70 || response.Summary.Rewarded != 51 || response.Summary.Pending != 12 || response.Summary.Review != 4 || response.Summary.Blocked != 7 {
		t.Fatalf("unexpected invite response: pagination=%+v summary=%+v", response.Pagination, response.Summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
