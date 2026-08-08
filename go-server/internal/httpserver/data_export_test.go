package httpserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestExportValuePreservesJSONAndTimeValues(t *testing.T) {
	decoded := exportValue([]byte(`{"enabled":true,"count":2}`), "JSONB")
	value, ok := decoded.(map[string]interface{})
	if !ok || value["enabled"] != true || value["count"] != float64(2) {
		t.Fatalf("decoded JSON export value = %#v", decoded)
	}

	exportedTime := exportValue(time.Date(2026, 8, 8, 12, 30, 0, 0, time.FixedZone("CST", 8*60*60)), "TIMESTAMP")
	if exportedTime != "2026-08-08T04:30:00Z" {
		t.Fatalf("exported time = %#v", exportedTime)
	}
}

func TestExportTableUsesColumnNames(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	previousDialect := database.CurrentDialect()
	defer database.SetDialect(string(previousDialect))
	database.SetDialect("mysql")
	mock.ExpectQuery(`SELECT \* FROM users`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "credits"}).AddRow("user-1", "user@example.com", 12.5))

	table, err := exportTable(context.Background(), database.Wrap(rawDB), "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Rows) != 1 || table.Rows[0]["id"] != "user-1" || table.Rows[0]["email"] != "user@example.com" {
		t.Fatalf("exported rows = %#v", table.Rows)
	}
	if _, ok := table.Rows[0]["credits"].(float64); !ok {
		t.Fatalf("credits should remain numeric, row = %#v", table.Rows)
	}
	if len(table.Columns) != 3 || table.Columns[0].Name != "id" {
		t.Fatalf("exported columns = %#v", table.Columns)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDataExportDocumentIsJSONSerializable(t *testing.T) {
	document := dataExportDocument{
		FormatVersion: "ai-pai-business-data/v1",
		Product:       "AI-PAI",
		ExportedAt:    "2026-08-08T00:00:00Z",
		RestoreOrder:  exportTableNames,
		Excluded:      excludedExportData,
		Tables: []dataExportTable{{
			Name:    "users",
			Columns: []dataExportColumn{{Name: "id", Type: "VARCHAR"}},
			Rows:    []map[string]interface{}{{"id": "user-1"}},
		}},
	}
	if _, err := json.Marshal(document); err != nil {
		t.Fatal(err)
	}
}
