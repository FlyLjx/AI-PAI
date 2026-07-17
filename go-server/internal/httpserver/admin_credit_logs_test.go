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

func TestAdminCanListUserCreditLogs(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now().UTC().Truncate(time.Second)
	expectAdminBalanceUser(mock, "admin-1", "admin@example.com", 0, "admin", now)
	expectAdminBalanceUser(mock, "user-1", "user@example.com", 8.5, "user", now)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM credit_logs WHERE user_id = \? AND type = \?`).
		WithArgs("user-1", "deduct").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT id, user_id, type, amount, balance_after, COALESCE\(remark, ''\), created_at`).
		WithArgs("user-1", "deduct", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "type", "amount", "balance_after", "remark", "created_at"}).
			AddRow("log-1", "user-1", "deduct", 1.5, 8.5, "API 调用：测试模型", now))

	router := &Router{db: database.Wrap(rawDB), tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/users/user-1/credit-logs?type=deduct&page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.userProfile(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data []struct {
			ID           string  `json:"id"`
			Type         string  `json:"type"`
			Amount       float64 `json:"amount"`
			BalanceAfter float64 `json:"balanceAfter"`
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
	if len(response.Data) != 1 || response.Data[0].ID != "log-1" || response.Data[0].Type != "deduct" || response.Data[0].Amount != 1.5 || response.Data[0].BalanceAfter != 8.5 {
		t.Fatalf("unexpected data: %+v", response.Data)
	}
	if response.Pagination.Total != 1 || response.Pagination.Page != 1 || response.Pagination.PageSize != 10 {
		t.Fatalf("unexpected pagination: %+v", response.Pagination)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
