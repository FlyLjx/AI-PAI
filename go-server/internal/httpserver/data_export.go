package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// exportTableNames intentionally contains business/configuration data only.
// Runtime history, request/mail logs, task images, and short-lived tokens are
// excluded so the backup remains suitable for restoring a fresh deployment.
var exportTableNames = []string{
	"system_settings",
	"users",
	"api_providers",
	"ai_models",
	"user_model_price_overrides",
	"subscription_plans",
	"user_subscriptions",
	"recharge_orders",
	"redeem_codes",
	"user_checkins",
	"user_invites",
	"invite_rebate_records",
	"announcements",
	"announcement_users",
	"announcement_receipts",
	"user_api_keys",
	"api_access_keys",
}

var excludedExportData = []string{
	"generation_tasks",
	"generation_result_images",
	"credit_logs",
	"api_access_logs",
	"email_delivery_logs",
	"http_request_logs",
	"user_registration_fingerprints",
	"user_ip_evidence",
	"registration_challenges",
	"user_email_tokens",
	"system log files and cached task images",
}

type dataExportDocument struct {
	FormatVersion string            `json:"formatVersion"`
	Product       string            `json:"product"`
	ExportedAt    string            `json:"exportedAt"`
	RestoreOrder  []string          `json:"restoreOrder"`
	Excluded      []string          `json:"excluded"`
	Tables        []dataExportTable `json:"tables"`
}

type dataExportTable struct {
	Name    string                   `json:"name"`
	Columns []dataExportColumn       `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

type dataExportColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (r *Router) adminDataExport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 90*time.Second)
	defer cancel()
	document := dataExportDocument{
		FormatVersion: "ai-pai-business-data/v1",
		Product:       "AI-PAI",
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		RestoreOrder:  append([]string(nil), exportTableNames...),
		Excluded:      append([]string(nil), excludedExportData...),
		Tables:        make([]dataExportTable, 0, len(exportTableNames)),
	}
	for _, tableName := range exportTableNames {
		table, err := exportTable(ctx, r.db, tableName)
		if err != nil {
			writeError(w, err)
			return
		}
		document.Tables = append(document.Tables, table)
	}

	body, err := json.Marshal(document)
	if err != nil {
		writeError(w, err)
		return
	}
	filename := "ai-pai-business-data-" + time.Now().Format("20060102-150405") + ".json"
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func exportTable(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}, tableName string) (dataExportTable, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+tableName)
	if err != nil {
		return dataExportTable{}, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return dataExportTable{}, err
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return dataExportTable{}, err
	}
	exportColumns := make([]dataExportColumn, len(columns))
	for index, column := range columns {
		exportColumns[index] = dataExportColumn{Name: column, Type: columnTypes[index].DatabaseTypeName()}
	}
	items := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		targets := make([]interface{}, len(columns))
		for index := range values {
			targets[index] = &values[index]
		}
		if err := rows.Scan(targets...); err != nil {
			return dataExportTable{}, err
		}
		item := make(map[string]interface{}, len(columns))
		for index, column := range columns {
			item[column] = exportValue(values[index], columnTypes[index].DatabaseTypeName())
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dataExportTable{}, err
	}
	return dataExportTable{Name: tableName, Columns: exportColumns, Rows: items}, nil
}

func exportValue(value interface{}, databaseType string) interface{} {
	switch item := value.(type) {
	case []byte:
		var decoded interface{}
		if strings.Contains(strings.ToUpper(databaseType), "JSON") && strings.TrimSpace(string(item)) != "" && json.Unmarshal(item, &decoded) == nil {
			return decoded
		}
		return string(item)
	case time.Time:
		return item.UTC().Format(time.RFC3339Nano)
	default:
		return value
	}
}
