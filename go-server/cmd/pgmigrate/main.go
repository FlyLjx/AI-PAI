package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/config"
	"aipi-go/internal/database"
	"aipi-go/internal/settings"

	mysql "github.com/go-sql-driver/mysql"
)

type tableSpec struct {
	name       string
	columns    []string
	transforms map[string]func(any) any
}

func main() {
	appclock.ConfigureDefault()
	cfg := config.Load()
	mysqlDB, err := openMySQLFromConfig(cfg.Database)
	if err != nil {
		log.Fatalf("open mysql failed: %v", err)
	}
	defer mysqlDB.Close()

	pgRaw, err := database.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open target db failed: %v", err)
	}
	defer pgRaw.Close()
	pg := database.Wrap(pgRaw)

	if database.CurrentDialect() != database.DialectPostgres {
		log.Fatal("target database must be postgres; set DB_DRIVER=postgres")
	}
	if err := database.EnsureSchema(pg.Raw()); err != nil {
		log.Fatalf("ensure postgres schema failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	specs := migrationSpecs()
	for _, spec := range specs {
		if err := migrateTable(ctx, mysqlDB, pg, spec); err != nil {
			log.Fatalf("migrate %s failed: %v", spec.name, err)
		}
	}

	if err := normalizeImportedSettings(ctx, pg); err != nil {
		log.Fatalf("normalize settings failed: %v", err)
	}

	fmt.Println("postgres migration ok")
}

func openMySQLFromConfig(cfg config.DatabaseConfig) (*sql.DB, error) {
	host := strings.TrimSpace(os.Getenv("MYSQL_HOST"))
	if host == "" {
		host = cfg.Host
	}
	port := cfg.Port
	if value := strings.TrimSpace(os.Getenv("MYSQL_PORT")); value != "" {
		fmt.Sscanf(value, "%d", &port)
	}
	user := strings.TrimSpace(os.Getenv("MYSQL_USER"))
	if user == "" {
		user = cfg.User
	}
	password := os.Getenv("MYSQL_PASSWORD")
	if password == "" {
		password = cfg.Password
	}
	name := strings.TrimSpace(os.Getenv("MYSQL_DATABASE"))
	if name == "" {
		name = cfg.Name
	}
	mysqlConfig := mysql.NewConfig()
	mysqlConfig.User = user
	mysqlConfig.Passwd = password
	mysqlConfig.Net = "tcp"
	mysqlConfig.Addr = fmt.Sprintf("%s:%d", host, port)
	mysqlConfig.DBName = name
	mysqlConfig.ParseTime = true
	mysqlConfig.Loc = appclock.ConfigureDefault()
	mysqlConfig.Params = map[string]string{
		"charset":   "utf8mb4,utf8",
		"time_zone": "'" + appclock.DefaultDatabaseTimeZone + "'",
	}
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	return db, db.Ping()
}

func migrationSpecs() []tableSpec {
	jsonString := func(v any) any {
		if v == nil {
			return nil
		}
		switch item := v.(type) {
		case []byte:
			text := strings.TrimSpace(string(item))
			if text == "" {
				return nil
			}
			return text
		case string:
			text := strings.TrimSpace(item)
			if text == "" {
				return nil
			}
			return text
		default:
			bytes, _ := json.Marshal(item)
			return string(bytes)
		}
	}
	clearHeavyTaskField := func(v any) any {
		return nil
	}
	boolValue := func(v any) any {
		if v == nil {
			return nil
		}
		switch item := v.(type) {
		case bool:
			return item
		case int64:
			return item != 0
		case int32:
			return item != 0
		case int:
			return item != 0
		case []byte:
			text := strings.TrimSpace(string(item))
			return text == "1" || strings.EqualFold(text, "true")
		case string:
			text := strings.TrimSpace(item)
			return text == "1" || strings.EqualFold(text, "true")
		default:
			return v
		}
	}
	return []tableSpec{
		{name: "users", columns: []string{"id", "email", "password_hash", "role", "status", "email_verified_at", "created_at", "updated_at", "credits"}},
		{name: "api_providers", columns: []string{"id", "name", "type", "capability", "base_url", "api_key", "status", "created_at", "updated_at"}},
		{name: "ai_models", columns: []string{"id", "provider_id", "model_name", "display_name", "capability", "status", "created_at", "updated_at", "price_1k", "price_2k", "price_4k", "append_size_to_prompt", "enabled_size_tiers", "sort_order", "cost_1k", "cost_2k", "cost_4k", "markup_percent", "price_change_percent"}, transforms: map[string]func(any) any{"append_size_to_prompt": boolValue, "enabled_size_tiers": jsonString}},
		{name: "generation_tasks", columns: []string{"id", "user_id", "model_id", "provider_id", "capability", "prompt", "reference_image_url", "size_tier", "size", "output_format", "transparent_background", "quantity", "user_ip", "cost_credits", "model_cost_credits", "remaining_credits", "duration_seconds", "status", "error_message", "result_json", "favorite_enabled", "public_status", "public_requested_at", "public_reviewed_at", "display_enabled", "display_note", "created_at", "updated_at"}, transforms: map[string]func(any) any{"reference_image_url": clearHeavyTaskField, "transparent_background": boolValue, "result_json": clearHeavyTaskField, "favorite_enabled": boolValue, "display_enabled": boolValue}},
		{name: "credit_logs", columns: []string{"id", "user_id", "type", "amount", "balance_after", "remark", "created_at"}},
		{name: "system_settings", columns: []string{"setting_key", "setting_value", "updated_at"}},
		{name: "recharge_orders", columns: []string{"id", "user_id", "out_trade_no", "trade_no", "order_type", "subscription_plan_id", "amount", "credits", "status", "pay_url", "qr_code", "paid_at", "created_at", "updated_at"}},
		{name: "subscription_plans", columns: []string{"id", "name", "description", "amount", "duration_days", "quota_images", "bonus_credits", "discount_percent", "allowed_provider_ids", "allowed_model_ids", "badge", "sort_order", "status", "created_at", "updated_at"}, transforms: map[string]func(any) any{"allowed_provider_ids": jsonString, "allowed_model_ids": jsonString}},
		{name: "user_subscriptions", columns: []string{"id", "user_id", "plan_id", "status", "started_at", "expires_at", "created_at", "updated_at"}},
		{name: "redeem_codes", columns: []string{"id", "code", "credits", "status", "remark", "user_id", "used_at", "expires_at", "created_at", "updated_at"}},
		{name: "user_checkins", columns: []string{"id", "user_id", "reward_credits", "checkin_date", "user_ip", "created_at"}},
		{name: "user_invites", columns: []string{"id", "inviter_id", "invitee_id", "reward_credits", "reward_type", "reward_plan_id", "reward_label", "invitee_ip", "created_at"}},
		{name: "announcements", columns: []string{"id", "title", "content", "display_mode", "target_type", "status", "sort_order", "created_at", "updated_at"}},
		{name: "announcement_users", columns: []string{"announcement_id", "user_id", "created_at"}},
		{name: "announcement_receipts", columns: []string{"announcement_id", "user_id", "signed_at"}},
		{name: "oauth_authorization_codes", columns: []string{"code", "client_id", "user_id", "redirect_uri", "scope", "expires_at", "used_at", "created_at"}},
		{name: "oauth_access_tokens", columns: []string{"token_hash", "client_id", "user_id", "scope", "expires_at", "created_at"}},
		{name: "user_email_tokens", columns: []string{"token_hash", "user_id", "purpose", "expires_at", "used_at", "created_at"}},
		{name: "user_api_keys", columns: []string{"id", "user_id", "name", "key_prefix", "key_hash", "key_plain", "encrypted_key", "status", "last_used_at", "deleted_at", "created_at", "updated_at"}},
	}
}

func migrateTable(ctx context.Context, src *sql.DB, dst *database.DB, spec tableSpec) error {
	selectSQL := fmt.Sprintf("SELECT %s FROM %s", strings.Join(spec.columns, ", "), spec.name)
	rows, err := src.QueryContext(ctx, selectSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	if _, err := dst.ExecContext(ctx, "TRUNCATE TABLE "+spec.name+" RESTART IDENTITY CASCADE"); err != nil {
		return err
	}

	placeholders := make([]string, 0, len(spec.columns))
	updateCols := make([]string, 0, len(spec.columns))
	for index, column := range spec.columns {
		placeholders = append(placeholders, fmt.Sprintf("$%d", index+1))
		updateCols = append(updateCols, fmt.Sprintf("%s = EXCLUDED.%s", column, column))
	}
	conflict := conflictColumns(spec.name)
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s",
		spec.name,
		strings.Join(spec.columns, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(conflict, ", "),
		strings.Join(updateCols, ", "),
	)

	count := 0
	for rows.Next() {
		values := make([]any, len(spec.columns))
		scanTargets := make([]any, len(spec.columns))
		for index := range values {
			scanTargets[index] = &values[index]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return err
		}
		for index, column := range spec.columns {
			if transform := spec.transforms[column]; transform != nil {
				values[index] = transform(values[index])
			}
		}
		if _, err := dst.Raw().ExecContext(ctx, insertSQL, values...); err != nil {
			return fmt.Errorf("insert row into %s: %w", spec.name, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	log.Printf("migrated %-24s %d rows", spec.name, count)
	return nil
}

func conflictColumns(table string) []string {
	switch table {
	case "system_settings":
		return []string{"setting_key"}
	case "announcement_users", "announcement_receipts":
		return []string{"announcement_id", "user_id"}
	case "oauth_authorization_codes":
		return []string{"code"}
	case "oauth_access_tokens", "user_email_tokens":
		return []string{"token_hash"}
	default:
		return []string{"id"}
	}
}

func normalizeImportedSettings(ctx context.Context, db *database.DB) error {
	values, err := settings.NewRepository(db).Get(ctx)
	if err != nil {
		return err
	}
	_, err = settings.NewRepository(db).Update(ctx, values)
	return err
}
