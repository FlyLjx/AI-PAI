package httpserver

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/database"
)

func TestWriteUsageWorkbookCreatesReadableXLSX(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT") + `(?s).*api_access_logs\.created_at.*ORDER BY api_access_logs\.created_at DESC`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"created_at", "endpoint", "task_id", "model", "size", "quantity", "charged_credits", "status", "error_message",
		}).AddRow(time.Date(2026, 8, 22, 10, 30, 0, 0, time.Local), "/v1/images/generations", "task-1", "gpt-image-1", "1k", 2, 0.25, "success", nil))

	path := t.TempDir() + string(os.PathSeparator) + "usage.xlsx"
	count, err := writeUsageWorkbook(context.Background(), path, apiaccess.NewRepository(database.Wrap(rawDB)), apiaccess.ListLogsInput{UserID: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("exported row count = %d, want 1", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	contents := make(map[string]string)
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents[file.Name] = string(data)
	}
	if !strings.Contains(contents["xl/_rels/workbook.xml.rels"], "worksheets/sheet1.xml") {
		t.Fatal("workbook sheet relationship is missing")
	}
	if !strings.Contains(contents["xl/worksheets/sheet1.xml"], "请求时间") || !strings.Contains(contents["xl/worksheets/sheet1.xml"], "task-1") {
		t.Fatal("workbook sheet does not contain the expected header and row")
	}
}
