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

func TestAdminAPIAccessLogsReturnsFilteredFullSummary(t *testing.T) {
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
	mock.ExpectQuery(`(?s)SELECT\s+api_access_logs\.id,\s+generation_tasks\.status.*LIMIT \?`).
		WithArgs(200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "quantity", "error_message"}))

	filterArgs := []driver.Value{"user-1", "key-1"}
	for range 15 {
		filterArgs = append(filterArgs, "%needle%")
	}
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM api_access_logs.*api_access_logs\.user_id = \?.*api_access_logs\.api_key_id = \?.*status IN \('success', 'succeeded'\).*users\.email`).
		WithArgs(filterArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(37))
	listArgs := append(append([]driver.Value{}, filterArgs...), 100, 0)
	mock.ExpectQuery(`(?s)SELECT\s+api_access_logs\.id,.*FROM api_access_logs.*status IN \('success', 'succeeded'\).*ORDER BY.*LIMIT \? OFFSET \?`).
		WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\) AS total,.*FROM api_access_logs.*api_access_logs\.user_id = \?.*api_access_logs\.api_key_id = \?.*status IN \('success', 'succeeded'\).*users\.email`).
		WithArgs(filterArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"total", "success", "failed", "image_count", "charged_credits", "model_cost_credits"}).AddRow(37, 31, 6, 52, 18.75, 9.25))

	router := &Router{db: database.Wrap(rawDB), tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/admin/api-access/logs?userId=user-1&apiKeyId=key-1&status=SUCCESS&keyword=Needle&page=-2&pageSize=500", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.adminAPIAccessLogs(recorder, req)

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
			Total            int     `json:"total"`
			Success          int     `json:"success"`
			Failed           int     `json:"failed"`
			ImageCount       int     `json:"imageCount"`
			ChargedCredits   float64 `json:"chargedCredits"`
			ModelCostCredits float64 `json:"modelCostCredits"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 0 {
		t.Fatalf("data length = %d, want an empty current page", len(response.Data))
	}
	if response.Pagination.Total != 37 || response.Pagination.Page != 1 || response.Pagination.PageSize != 100 {
		t.Fatalf("unexpected pagination: %+v", response.Pagination)
	}
	if response.Summary.Total != 37 || response.Summary.Success != 31 || response.Summary.Failed != 6 || response.Summary.ImageCount != 52 || response.Summary.ChargedCredits != 18.75 || response.Summary.ModelCostCredits != 9.25 {
		t.Fatalf("summary should describe all filtered rows, got %+v", response.Summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
