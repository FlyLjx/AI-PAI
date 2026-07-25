package users

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRecordIPEvidenceUpsertsCanonicalEvidence(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	sum := sha256.Sum256([]byte("ip:203.0.113.42"))
	expectedHash := hex.EncodeToString(sum[:])
	mock.ExpectExec(`INSERT INTO user_ip_evidence`).
		WithArgs("user-1", expectedHash, "203.0.113.42", IPSourceAPI, "key-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = NewRepository(database.Wrap(rawDB)).RecordIPEvidence(context.Background(), "user-1", IPSourceAPI, "203.0.113.42", "key-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecordIPEvidenceIgnoresIncompleteEvidence(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	repo := NewRepository(database.Wrap(rawDB))
	for _, test := range []struct {
		userID string
		source string
		ip     string
	}{
		{"", IPSourceLogin, "203.0.113.42"},
		{"user-1", "unknown", "203.0.113.42"},
		{"user-1", IPSourceLogin, ""},
	} {
		if err := repo.RecordIPEvidence(context.Background(), test.userID, test.source, test.ip, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
