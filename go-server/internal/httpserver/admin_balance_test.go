package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aipi-go/internal/auth"
	"aipi-go/internal/config"
	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAdminCanSetUserBalance(t *testing.T) {
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
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT credits FROM users WHERE id = \? FOR UPDATE`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(10.0))
	mock.ExpectExec(`UPDATE users`).
		WithArgs(25.5, "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO credit_logs`).
		WithArgs(sqlmock.AnyArg(), "user-1", 15.5, 25.5, "管理员 admin-1：补发余额").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectAdminBalanceUser(mock, "user-1", "user@example.com", 25.5, "user", now)

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "http://example.test/api/users/user-1/balance", strings.NewReader(`{"balance":25.5,"remark":"补发余额"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.userProfile(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Credits float64 `json:"credits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Credits != 25.5 {
		t.Fatalf("credits = %v, want 25.5", response.Data.Credits)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminBalanceRequiresPatch(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/users/user-1/balance", nil)
	recorder := httptest.NewRecorder()

	new(Router).userProfile(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func expectAdminBalanceUser(mock sqlmock.Sqlmock, id string, email string, credits float64, role string, now time.Time) {
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow(id, email, nil, nil, nil, "hash", credits, role, "active", now, now, now))
}
