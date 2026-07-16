package apiaccess

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

const revealKeyQuery = `SELECT key_plain FROM api_access_keys WHERE id = \? AND user_id = \? AND deleted_at IS NULL LIMIT 1`

func TestRevealUserKeyReturnsOwnedNonDeletedKey(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(revealKeyQuery).
		WithArgs("key-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"key_plain"}).AddRow("sk-aipai-secret"))

	service := NewService(NewRepository(database.Wrap(rawDB)), nil)
	key, err := service.RevealUserKey(context.Background(), "key-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if key != "sk-aipai-secret" {
		t.Fatalf("key = %q, want owned key", key)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRevealUserKeyHidesMissingDeletedAndOtherUsersKeys(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(revealKeyQuery).
		WithArgs("key-1", "user-2").
		WillReturnError(sql.ErrNoRows)

	service := NewService(NewRepository(database.Wrap(rawDB)), nil)
	_, err = service.RevealUserKey(context.Background(), "key-1", "user-2")
	if !errors.Is(err, ErrAccessKeyNotFound) {
		t.Fatalf("error = %v, want ErrAccessKeyNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRevealUserKeyRejectsMissingPlaintext(t *testing.T) {
	database.SetDialect("mysql")
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(revealKeyQuery).
		WithArgs("key-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"key_plain"}).AddRow(nil))

	service := NewService(NewRepository(database.Wrap(rawDB)), nil)
	_, err = service.RevealUserKey(context.Background(), "key-1", "user-1")
	if !errors.Is(err, ErrKeyPlainUnavailable) {
		t.Fatalf("error = %v, want ErrKeyPlainUnavailable", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPublicAccessKeyDoesNotExposeStoredPlaintext(t *testing.T) {
	plain := "sk-aipai-secret"
	encoded, err := json.Marshal(ToPublicKey(AccessKey{KeyPlain: &plain}))
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	if strings.Contains(body, "keyPlain") || strings.Contains(body, plain) {
		t.Fatalf("public key response exposed stored plaintext: %s", body)
	}
}
