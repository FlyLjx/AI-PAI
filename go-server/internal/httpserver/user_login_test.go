package httpserver

import (
	"encoding/json"
	"errors"
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

func TestUserLoginAllowsUnverifiedEmail(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectQuery(`SELECT .* FROM users WHERE email = \? LIMIT 1`).
		WithArgs("user@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow(
			"user-1", "user@example.com", nil, nil, nil, auth.HashPassword("password123"),
			0, "user", "active", nil, now, now,
		))
	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).
		WillReturnError(errors.New("subscription settings unavailable"))

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/users/login", strings.NewReader(`{"email":"user@example.com","password":"password123"}`))
	recorder := httptest.NewRecorder()
	router.userLogin(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Token           string  `json:"token"`
			EmailVerifiedAt *string `json:"emailVerifiedAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Token == "" {
		t.Fatal("login response is missing token")
	}
	if response.Data.EmailVerifiedAt != nil {
		t.Fatalf("emailVerifiedAt = %v, want nil", response.Data.EmailVerifiedAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
