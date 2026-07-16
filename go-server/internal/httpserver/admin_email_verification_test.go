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

func TestAdminCanVerifyUserEmail(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now().UTC()
	expectAdminVerificationUser(mock, "admin-1", "admin@example.com", "admin", now, now)
	mock.ExpectExec(`UPDATE users SET email_verified_at = COALESCE\(email_verified_at, NOW\(\)\) WHERE id = \?`).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectAdminVerificationUser(mock, "user-1", "user@example.com", "user", now, now)
	mock.ExpectQuery(`SELECT setting_key, setting_value FROM system_settings`).WillReturnError(sql.ErrConnDone)

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/users/user-1/verify-email", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.userProfile(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Data struct {
			ID              string  `json:"id"`
			EmailVerifiedAt *string `json:"emailVerifiedAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.ID != "user-1" || response.Data.EmailVerifiedAt == nil {
		t.Fatalf("unexpected response data: %+v", response.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminVerifyUserEmailReturnsNotFound(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now().UTC()
	expectAdminVerificationUser(mock, "admin-1", "admin@example.com", "admin", now, now)
	mock.ExpectExec(`UPDATE users SET email_verified_at = COALESCE\(email_verified_at, NOW\(\)\) WHERE id = \?`).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/users/missing/verify-email", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.userProfile(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminVerifyUserEmailRequiresAdmin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/users/user-1/verify-email", nil)
	recorder := httptest.NewRecorder()

	new(Router).userProfile(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}

func TestAdminVerifyUserEmailRequiresPOST(t *testing.T) {
	req := httptest.NewRequest(http.MethodPatch, "http://example.test/api/users/user-1/verify-email", nil)
	recorder := httptest.NewRecorder()

	new(Router).userProfile(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusMethodNotAllowed, recorder.Body.String())
	}
}

func expectAdminVerificationUser(mock sqlmock.Sqlmock, id string, email string, role string, createdAt time.Time, verifiedAt time.Time) {
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow(id, email, nil, nil, nil, "hash", 0, role, "active", verifiedAt, createdAt, createdAt))
}
