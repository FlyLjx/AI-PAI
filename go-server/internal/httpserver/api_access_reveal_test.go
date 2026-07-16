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

func TestRevealUserAPIAccessKey(t *testing.T) {
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
	mock.ExpectQuery(`SELECT key_plain FROM api_access_keys WHERE id = \? AND user_id = \? AND deleted_at IS NULL LIMIT 1`).
		WithArgs("key-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"key_plain"}).AddRow("sk-aipai-secret"))

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateUserToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/api-access/keys/key-1/reveal", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.userAPIAccessKeyByID(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
	if recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("Pragma = %q, want no-cache", recorder.Header().Get("Pragma"))
	}
	var response struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Key != "sk-aipai-secret" {
		t.Fatalf("data.key = %q, want revealed key", response.Data.Key)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRevealUserAPIAccessKeyReturnsNotFoundForUnownedKey(t *testing.T) {
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
	mock.ExpectQuery(`SELECT key_plain FROM api_access_keys WHERE id = \? AND user_id = \? AND deleted_at IS NULL LIMIT 1`).
		WithArgs("key-2", "user-1").
		WillReturnError(sql.ErrNoRows)

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateUserToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/api-access/keys/key-2/reveal", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.userAPIAccessKeyByID(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	if recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("Pragma = %q, want no-cache", recorder.Header().Get("Pragma"))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
