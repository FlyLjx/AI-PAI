package database

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCleanupLegacyGenerationImageDataRunsOnceAndPreservesURLs(t *testing.T) {
	previousDialect := CurrentDialect()
	SetDialect("mysql")
	defer SetDialect(string(previousDialect))

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT setting_value FROM system_settings WHERE setting_key = ?")).
		WithArgs(imageDataCleanupMigrationKey).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT id, reference_image_url, result_json.*created_at < \?.*LIMIT \?`).
		WithArgs(sqlmock.AnyArg(), "", imageDataCleanupBatchSize).
		WillReturnRows(sqlmock.NewRows([]string{"id", "reference_image_url", "result_json"}).AddRow(
			"task-1",
			`["data:image/png;base64,REFERENCE","https://example.test/reference.png"]`,
			`{"data":[{"url":"https://example.test/result.png","b64_json":"RESULT"}],"usage":{"total_tokens":10}}`,
		))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE generation_tasks SET reference_image_url = ?, result_json = ? WHERE id = ?")).
		WithArgs(
			`["https://example.test/reference.png"]`,
			`{"data":[{"url":"https://example.test/result.png"}],"usage":{"total_tokens":10}}`,
			"task-1",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)SELECT id, reference_image_url, result_json.*id > \?.*LIMIT \?`).
		WithArgs(sqlmock.AnyArg(), "task-1", imageDataCleanupBatchSize).
		WillReturnRows(sqlmock.NewRows([]string{"id", "reference_image_url", "result_json"}))
	mock.ExpectExec(`INSERT INTO system_settings`).
		WithArgs(imageDataCleanupMigrationKey).
		WillReturnResult(sqlmock.NewResult(1, 1))

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if err := cleanupLegacyGenerationImageData(context.Background(), db, now, 0); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupLegacyGenerationImageDataSkipsCompletedMigration(t *testing.T) {
	previousDialect := CurrentDialect()
	SetDialect("mysql")
	defer SetDialect(string(previousDialect))

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT setting_value FROM system_settings WHERE setting_key = ?")).
		WithArgs(imageDataCleanupMigrationKey).
		WillReturnRows(sqlmock.NewRows([]string{"setting_value"}).AddRow("complete"))

	if err := cleanupLegacyGenerationImageData(context.Background(), db, time.Now(), 0); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
