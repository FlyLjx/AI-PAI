package models

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"aipi-go/internal/database"
)

func TestFindActiveByNameOrDisplayNamePrefersExactPublicModelID(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectQuery(`(?s)FROM ai_models.*ai_models\.display_name = \?`).
		WithArgs("gpt-image-2").
		WillReturnRows(modelLookupRows().
			AddRow("model-public", "provider-public", "Provider Public", "openai", "active", "gpt-image-2", "gpt-image-2", "chat_image", 0.1, 0.2, 0.4, 0.0, 0.0, 0.1, 0.2, 0.4, false, `["1k","2k"]`, 10, "active", now, now))

	model, err := NewRepository(database.Wrap(rawDB)).FindActiveByNameOrDisplayName(context.Background(), "gpt-image-2")
	if err != nil {
		t.Fatal(err)
	}
	if model.ProviderID != "provider-public" || model.DisplayName != "gpt-image-2" {
		t.Fatalf("selected provider=%q display=%q, want exact public model", model.ProviderID, model.DisplayName)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindActiveByNameOrDisplayNameRejectsAmbiguousUpstreamModelName(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectQuery(`(?s)FROM ai_models.*ai_models\.display_name = \?`).
		WithArgs("gpt-image-2").
		WillReturnRows(modelLookupRows())
	mock.ExpectQuery(`(?s)FROM ai_models.*ai_models\.model_name = \?`).
		WithArgs("gpt-image-2").
		WillReturnRows(modelLookupRows().
			AddRow("model-a", "provider-a", "Provider A", "openai", "active", "gpt-image-2", "gpt-image-2-1k", "chat_image", 0.1, 0.2, 0.4, 0.0, 0.0, 0.1, 0.2, 0.4, false, `["1k"]`, 10, "active", now, now).
			AddRow("model-b", "provider-b", "Provider B", "openai", "active", "gpt-image-2", "gpt-image-2-4k", "chat_image", 0.1, 0.2, 0.4, 0.0, 0.0, 0.1, 0.2, 0.4, false, `["4k"]`, 20, "active", now, now))

	_, err = NewRepository(database.Wrap(rawDB)).FindActiveByNameOrDisplayName(context.Background(), "gpt-image-2")
	if !errors.Is(err, ErrAmbiguousModelName) {
		t.Fatalf("err=%v, want ErrAmbiguousModelName", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindActiveByNameOrDisplayNameSkipsHiddenModels(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(`(?s)FROM ai_models.*ai_models\.deleted_at IS NULL.*ai_models\.display_name = \?`).
		WithArgs("gpt-image-2").
		WillReturnRows(modelLookupRows())
	mock.ExpectQuery(`(?s)FROM ai_models.*ai_models\.deleted_at IS NULL.*ai_models\.model_name = \?`).
		WithArgs("gpt-image-2").
		WillReturnRows(modelLookupRows())

	_, err = NewRepository(database.Wrap(rawDB)).FindActiveByNameOrDisplayName(context.Background(), "gpt-image-2")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err=%v, want sql.ErrNoRows", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureUniqueDisplayNameRejectsDuplicatePublicModelID(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(`SELECT id\s+FROM ai_models`).
		WithArgs("chat_image", "gpt-image-2", "model-current", "model-current").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("model-other"))

	err = NewRepository(database.Wrap(rawDB)).ensureUniqueDisplayName(context.Background(), "gpt-image-2", "chat_image", "model-current")
	if !errors.Is(err, ErrDuplicateModelDisplayName) {
		t.Fatalf("err=%v, want ErrDuplicateModelDisplayName", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindAllExcludesHiddenModels(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	now := time.Now()
	mock.ExpectQuery(`(?s)FROM ai_models.*WHERE ai_models\.deleted_at IS NULL.*ORDER BY`).
		WillReturnRows(modelLookupRows().
			AddRow("model-visible", "provider-visible", "Provider Visible", "openai", "active", "gpt-image-2", "gpt-image-2", "chat_image", 0.1, 0.2, 0.4, 0.0, 0.0, 0.1, 0.2, 0.4, false, `["1k","2k"]`, 10, "active", now, now))

	items, err := NewRepository(database.Wrap(rawDB)).FindAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "model-visible" {
		t.Fatalf("items=%v, want only visible model", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteSoftDeletesModel(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectExec(`(?s)UPDATE ai_models.*SET status = 'disabled'.*deleted_at = COALESCE\(deleted_at, CURRENT_TIMESTAMP\).*WHERE id = \?`).
		WithArgs("model-delete").
		WillReturnResult(sqlmock.NewResult(0, 1))

	deleted, err := NewRepository(database.Wrap(rawDB)).Delete(context.Background(), "model-delete")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("deleted=false, want true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureUniqueDisplayNameAllowsCurrentModel(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(`SELECT id\s+FROM ai_models`).
		WithArgs("chat_image", "gpt-image-2", "model-current", "model-current").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if err := NewRepository(database.Wrap(rawDB)).ensureUniqueDisplayName(context.Background(), "gpt-image-2", "chat_image", "model-current"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func modelLookupRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"provider_id",
		"provider_name",
		"provider_type",
		"provider_status",
		"model_name",
		"display_name",
		"capability",
		"cost_1k",
		"cost_2k",
		"cost_4k",
		"markup_percent",
		"price_change_percent",
		"price_1k",
		"price_2k",
		"price_4k",
		"append_size_to_prompt",
		"enabled_size_tiers",
		"sort_order",
		"status",
		"created_at",
		"updated_at",
	})
}
