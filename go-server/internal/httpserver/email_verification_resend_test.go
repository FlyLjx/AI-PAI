package httpserver

import (
	"database/sql"
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

func TestUserCanResendEmailVerification(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	expectResendVerificationUser(mock, nil, now)
	expectResendVerificationUser(mock, nil, now)
	mock.ExpectQuery(`SELECT created_at\s+FROM user_email_tokens`).
		WithArgs("user-1", "verify_email").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).
		WillReturnRows(sqlmock.NewRows([]string{"setting_key", "setting_value"}))
	mock.ExpectExec(`INSERT INTO user_email_tokens`).
		WithArgs(sqlmock.AnyArg(), "user-1", "verify_email", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateUserToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/users/user-1/resend-verification", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.userProfile(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Data struct {
			VerificationRequired bool   `json:"verificationRequired"`
			Email                string `json:"email"`
			VerificationURL      string `json:"verificationUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Data.VerificationRequired || response.Data.Email != "user@example.com" || response.Data.VerificationURL == "" {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResendEmailVerificationEnforcesCooldown(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	expectResendVerificationUser(mock, nil, now)
	expectResendVerificationUser(mock, nil, now)
	mock.ExpectQuery(`SELECT created_at\s+FROM user_email_tokens`).
		WithArgs("user-1", "verify_email").
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateUserToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/users/user-1/resend-verification", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.userProfile(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusTooManyRequests, recorder.Body.String())
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is missing")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectResendVerificationUser(mock sqlmock.Sqlmock, verifiedAt any, now time.Time) {
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow("user-1", "user@example.com", nil, nil, nil, "hash", 0, "user", "active", verifiedAt, now, now))
}
