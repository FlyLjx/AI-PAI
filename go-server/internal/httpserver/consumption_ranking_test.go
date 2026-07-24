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

func TestAdminCanListConsumptionRanking(t *testing.T) {
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
	mock.ExpectQuery(`(?s)SELECT\s+credit_logs\.user_id,.*FROM credit_logs.*WHERE credit_logs\.type='deduct'.*GROUP BY credit_logs\.user_id, users\.email, users\.status.*LIMIT \?`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "user_email", "user_status", "deduct_count", "credits_spent", "last_deduct_at",
		}).AddRow("user-1", "user@example.com", "active", 2, 6.5, now))

	router := &Router{db: database.Wrap(rawDB), tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/admin/users/consumption-ranking?days=0&limit=5", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.adminConsumptionRanking(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data []struct {
			UserID       string  `json:"userId"`
			UserEmail    string  `json:"userEmail"`
			UserStatus   string  `json:"userStatus"`
			DeductCount  int     `json:"deductCount"`
			CreditsSpent float64 `json:"creditsSpent"`
			WindowDays   int     `json:"windowDays"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 || response.Data[0].UserID != "user-1" || response.Data[0].CreditsSpent != 6.5 || response.Data[0].WindowDays != 0 {
		t.Fatalf("unexpected data: %+v", response.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
