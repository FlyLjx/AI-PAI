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

func TestProviderModelsByIDUsesSavedCredentialsAndFiltersCapability(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer saved-secret" {
			t.Errorf("Authorization = %q, want saved provider credential", req.Header.Get("Authorization"))
		}
		switch req.URL.Path {
		case "/v1/models":
			writeJSON(w, http.StatusOK, map[string]any{"data": []any{
				map[string]any{"id": " gpt-image-z ", "cost_1k": 1, "cost_2k": 2, "cost_4k": 4},
				map[string]any{"id": "gpt-5-5", "cost_1k": 9},
				map[string]any{"id": "gpt-image-a"},
				map[string]any{"id": "gpt-image-a", "pricing": map[string]any{"1k": 3, "2k": 5, "4k": 7}},
				map[string]any{"id": "   "},
			}})
		case "/api/ratio_config":
			writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
				"model_price": map[string]any{},
				"model_ratio": map[string]any{},
			}})
		case "/api/pricing":
			writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
		default:
			http.NotFound(w, req)
		}
	}))
	defer upstream.Close()

	now := time.Now()
	expectAdminUser(mock, now)
	mock.ExpectQuery(`SELECT id, name, type, capability, base_url, api_key, status, created_at, updated_at FROM api_providers WHERE id = \? LIMIT 1`).
		WithArgs("provider-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "type", "capability", "base_url", "api_key", "status", "created_at", "updated_at",
		}).AddRow("provider-1", "AI-PAI", "newapi", "chat_image", upstream.URL, "Bearer saved-secret", "active", now, now))

	router := &Router{
		db:     database.Wrap(rawDB),
		tokens: auth.NewTokenManager(config.DatabaseConfig{}),
	}
	token, err := router.tokens.CreateAdminToken("admin-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/api-providers/provider-1/models", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.providerByID(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var response struct {
		Data []remoteModel `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 2 {
		t.Fatalf("data length = %d, want 2; data = %+v", len(response.Data), response.Data)
	}
	if response.Data[0].Name != "gpt-image-a" || response.Data[1].Name != "gpt-image-z" {
		t.Fatalf("models are not trimmed, filtered, deduplicated, and sorted: %+v", response.Data)
	}
	if response.Data[0].Cost1K != 3 || response.Data[0].Cost2K != 5 || response.Data[0].Cost4K != 7 {
		t.Fatalf("first model costs = %+v, want 3/5/7", response.Data[0].remoteModelPrice)
	}
	if response.Data[1].Cost1K != 1 || response.Data[1].Cost2K != 2 || response.Data[1].Cost4K != 4 {
		t.Fatalf("second model costs = %+v, want 1/2/4", response.Data[1].remoteModelPrice)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderModelsByIDReturnsNotFoundForMissingProvider(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	expectAdminUser(mock, now)
	mock.ExpectQuery(`SELECT id, name, type, capability, base_url, api_key, status, created_at, updated_at FROM api_providers WHERE id = \? LIMIT 1`).
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
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/api-providers/missing/models", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.providerByID(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderModelsByIDRequiresGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/api-providers/provider-1/models", nil)
	recorder := httptest.NewRecorder()

	new(Router).providerByID(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusMethodNotAllowed, recorder.Body.String())
	}
}

func expectAdminUser(mock sqlmock.Sqlmock, now time.Time) {
	mock.ExpectQuery(`SELECT .* FROM users WHERE id = \? LIMIT 1`).
		WithArgs("admin-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "invite_code", "invited_by", "invited_ip", "password_hash",
			"credits", "role", "status", "email_verified_at", "created_at", "updated_at",
		}).AddRow("admin-1", "admin@example.com", nil, nil, nil, "hash", 0, "admin", "active", now, now, now))
}
