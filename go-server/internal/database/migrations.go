package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	migrationInviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	migrationInviteCodeLength   = 8
)

func EnsureSchema(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	statements := schemaBootstrapStatements()
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, Rebind(statement)); err != nil {
			return err
		}
	}
	if err := addColumnIfMissing(ctx, db, "generation_tasks", "output_format", "VARCHAR(20) NULL", "size"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "users", "invite_code", "VARCHAR(16) NULL", "email"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "users", "invited_by", "VARCHAR(36) NULL", "invite_code"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "users", "invited_ip", "VARCHAR(64) NULL", "invited_by"); err != nil {
		return err
	}
	if err := backfillUserInviteCodes(ctx, db); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "subscription_plans", "quota_images", "INTEGER NOT NULL DEFAULT 100", "duration_days"); err != nil {
		return err
	}
	if err := backfillSubscriptionPlanQuotas(ctx, db); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_subscriptions", "plan_snapshot", JSONTextType()+" NULL", "plan_id"); err != nil {
		return err
	}
	if err := backfillUserSubscriptionPlanSnapshots(ctx, db); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "announcements", "display_mode", "VARCHAR(20) NOT NULL DEFAULT 'popup'", "content"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "reward_type", "VARCHAR(20) NOT NULL DEFAULT 'credits'", "reward_credits"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "reward_plan_id", "VARCHAR(36) NULL", "reward_type"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "reward_label", "VARCHAR(120) NULL", "reward_plan_id"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "reward_plan_snapshot", JSONTextType()+" NULL", "reward_label"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "invitee_reward_type", "VARCHAR(20) NOT NULL DEFAULT 'none'", "reward_plan_snapshot"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "invitee_reward_credits", "NUMERIC(12,4) NOT NULL DEFAULT 0", "invitee_reward_type"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "invitee_reward_plan_id", "VARCHAR(36) NULL", "invitee_reward_credits"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "invitee_reward_label", "VARCHAR(120) NULL", "invitee_reward_plan_id"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "invitee_reward_plan_snapshot", JSONTextType()+" NULL", "invitee_reward_label"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "status", "VARCHAR(20) NOT NULL DEFAULT 'rewarded'", "invitee_reward_plan_snapshot"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "ip_hash", "VARCHAR(64) NULL", "status"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "device_hash", "VARCHAR(64) NULL", "ip_hash"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "risk_reason", "VARCHAR(255) NULL", "device_hash"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "verified_at", "TIMESTAMP NULL", "risk_reason"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "user_invites", "rewarded_at", "TIMESTAMP NULL", "verified_at"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "subscription_lottery_prizes", "prize_type", "VARCHAR(20) NOT NULL DEFAULT 'subscription'", "name"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "subscription_lottery_prizes", "monthly_stock", "INTEGER NOT NULL DEFAULT 0", "daily_stock"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "subscription_lottery_records", "prize_type", "VARCHAR(20) NOT NULL DEFAULT 'subscription'", "prize_id"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "api_access_keys", "key_plain", "VARCHAR(255) NULL", "key_hash"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "api_access_keys", "concurrency_limit", "INTEGER NOT NULL DEFAULT 10", "status"); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, db, "api_access_keys", "billing_mode", "VARCHAR(16) NULL", "concurrency_limit"); err != nil {
		return err
	}
	if err := normalizeAPIAccessKeyConcurrencyDefaults(ctx, db); err != nil {
		return err
	}
	indexes := []struct {
		name      string
		statement string
	}{
		{"idx_generation_tasks_status_created_at", `CREATE INDEX idx_generation_tasks_status_created_at ON generation_tasks (status, created_at)`},
		{"idx_generation_tasks_user_id_created_at", `CREATE INDEX idx_generation_tasks_user_id_created_at ON generation_tasks (user_id, created_at)`},
		{"idx_generation_tasks_created_at_user_id", `CREATE INDEX idx_generation_tasks_created_at_user_id ON generation_tasks (created_at, user_id)`},
		{"idx_generation_tasks_public_status_display_enabled_created_at", `CREATE INDEX idx_generation_tasks_public_status_display_enabled_created_at ON generation_tasks (public_status, display_enabled, created_at)`},
		{"idx_recharge_orders_status_created_at", `CREATE INDEX idx_recharge_orders_status_created_at ON recharge_orders (status, created_at)`},
		{"idx_recharge_orders_user_id_created_at", `CREATE INDEX idx_recharge_orders_user_id_created_at ON recharge_orders (user_id, created_at)`},
		{"idx_credit_logs_user_id_created_at", `CREATE INDEX idx_credit_logs_user_id_created_at ON credit_logs (user_id, created_at)`},
		{"idx_credit_logs_type_created_at", `CREATE INDEX idx_credit_logs_type_created_at ON credit_logs (type, created_at)`},
		{"uq_users_invite_code", `CREATE UNIQUE INDEX uq_users_invite_code ON users (invite_code)`},
		{"idx_users_invited_by", `CREATE INDEX idx_users_invited_by ON users (invited_by)`},
		{"idx_user_checkins_user_id_checkin_date", `CREATE INDEX idx_user_checkins_user_id_checkin_date ON user_checkins (user_id, checkin_date)`},
		{"idx_user_invites_invitee_id", `CREATE INDEX idx_user_invites_invitee_id ON user_invites (invitee_id)`},
		{"idx_user_invites_inviter_id", `CREATE INDEX idx_user_invites_inviter_id ON user_invites (inviter_id)`},
		{"idx_user_invites_status_created", `CREATE INDEX idx_user_invites_status_created ON user_invites (status, created_at)`},
		{"idx_user_invites_device_created", `CREATE INDEX idx_user_invites_device_created ON user_invites (device_hash, created_at)`},
		{"idx_invite_rebate_records_inviter_created", `CREATE INDEX idx_invite_rebate_records_inviter_created ON invite_rebate_records (inviter_id, created_at)`},
		{"idx_invite_rebate_records_invitee_created", `CREATE INDEX idx_invite_rebate_records_invitee_created ON invite_rebate_records (invitee_id, created_at)`},
		{"idx_invite_rebate_records_invite_created", `CREATE INDEX idx_invite_rebate_records_invite_created ON invite_rebate_records (invite_id, created_at)`},
		{"idx_registration_fingerprints_ip_created", `CREATE INDEX idx_registration_fingerprints_ip_created ON user_registration_fingerprints (ip_hash, created_at)`},
		{"idx_registration_fingerprints_device_created", `CREATE INDEX idx_registration_fingerprints_device_created ON user_registration_fingerprints (device_hash, created_at)`},
		{"idx_registration_challenges_ip_created", `CREATE INDEX idx_registration_challenges_ip_created ON registration_challenges (ip_hash, created_at)`},
		{"idx_email_delivery_logs_created_at", `CREATE INDEX idx_email_delivery_logs_created_at ON email_delivery_logs (created_at)`},
		{"idx_email_delivery_logs_status_created", `CREATE INDEX idx_email_delivery_logs_status_created ON email_delivery_logs (status, created_at)`},
		{"idx_email_delivery_logs_category_created", `CREATE INDEX idx_email_delivery_logs_category_created ON email_delivery_logs (category, created_at)`},
		{"idx_email_delivery_logs_recipient_created", `CREATE INDEX idx_email_delivery_logs_recipient_created ON email_delivery_logs (recipient, created_at)`},
		{"idx_api_access_logs_task_id", `CREATE INDEX idx_api_access_logs_task_id ON api_access_logs (task_id)`},
		{"idx_subscription_lottery_prizes_status_sort", `CREATE INDEX idx_subscription_lottery_prizes_status_sort ON subscription_lottery_prizes (status, sort_order)`},
		{"idx_subscription_lottery_records_user_created", `CREATE INDEX idx_subscription_lottery_records_user_created ON subscription_lottery_records (user_id, created_at)`},
		{"idx_subscription_lottery_records_prize_date", `CREATE INDEX idx_subscription_lottery_records_prize_date ON subscription_lottery_records (prize_id, draw_date)`},
		{"uq_subscription_lottery_user_date", `CREATE UNIQUE INDEX uq_subscription_lottery_user_date ON subscription_lottery_records (user_id, draw_date)`},
	}
	for _, index := range indexes {
		if err := addIndexIfMissing(ctx, db, index.name, index.statement); err != nil {
			return err
		}
	}
	return nil
}

func backfillSubscriptionPlanQuotas(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, Rebind(`
		UPDATE subscription_plans
		SET quota_images = CASE
			WHEN duration_days <= 1 THEN 20
			WHEN duration_days <= 31 THEN 300
			WHEN duration_days <= 92 THEN 1000
			ELSE 100
		END
		WHERE COALESCE(quota_images, 0) <= 0
			OR (quota_images = 100 AND duration_days <= 31)
	`))
	return err
}

type migrationSubscriptionPlanSnapshot struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Description        *string  `json:"description"`
	Amount             float64  `json:"amount"`
	DurationDays       int      `json:"durationDays"`
	QuotaImages        int      `json:"quotaImages"`
	BonusCredits       float64  `json:"bonusCredits"`
	DiscountPercent    float64  `json:"discountPercent"`
	AllowedProviderIDs []string `json:"allowedProviderIds"`
	AllowedModelIDs    []string `json:"allowedModelIds"`
	Badge              *string  `json:"badge"`
	SortOrder          int      `json:"sortOrder"`
	Status             string   `json:"status"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}

func backfillUserSubscriptionPlanSnapshots(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, Rebind(`
		SELECT user_subscriptions.id,
			subscription_plans.id, subscription_plans.name, subscription_plans.description, subscription_plans.amount,
			subscription_plans.duration_days, subscription_plans.quota_images, subscription_plans.bonus_credits,
			subscription_plans.allowed_provider_ids, subscription_plans.allowed_model_ids, subscription_plans.badge,
			subscription_plans.sort_order, subscription_plans.status, subscription_plans.created_at, subscription_plans.updated_at
		FROM user_subscriptions
		INNER JOIN subscription_plans ON subscription_plans.id = user_subscriptions.plan_id
		WHERE user_subscriptions.plan_snapshot IS NULL
	`))
	if err != nil {
		return err
	}
	type pendingSnapshot struct {
		subscriptionID string
		payload        string
	}
	pending := make([]pendingSnapshot, 0)
	for rows.Next() {
		var subscriptionID string
		var item migrationSubscriptionPlanSnapshot
		var description, providers, models, badge sql.NullString
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&subscriptionID,
			&item.ID, &item.Name, &description, &item.Amount,
			&item.DurationDays, &item.QuotaImages, &item.BonusCredits,
			&providers, &models, &badge,
			&item.SortOrder, &item.Status, &createdAt, &updatedAt,
		); err != nil {
			rows.Close()
			return err
		}
		item.Description = migrationNullString(description)
		item.AllowedProviderIDs = migrationJSONStringList(providers.String)
		item.AllowedModelIDs = migrationJSONStringList(models.String)
		item.Badge = migrationNullString(badge)
		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		payload, err := json.Marshal(item)
		if err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, pendingSnapshot{subscriptionID: subscriptionID, payload: string(payload)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range pending {
		if _, err := db.ExecContext(ctx, Rebind(`
			UPDATE user_subscriptions
			SET plan_snapshot=?
			WHERE id=? AND plan_snapshot IS NULL
		`), item.payload, item.subscriptionID); err != nil {
			return err
		}
	}
	return nil
}

func migrationNullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func migrationJSONStringList(value string) []string {
	items := []string{}
	if strings.TrimSpace(value) == "" {
		return items
	}
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return []string{}
	}
	return items
}

func backfillUserInviteCodes(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, Rebind(`SELECT id, invite_code FROM users ORDER BY created_at ASC, id ASC`))
	if err != nil {
		return err
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	missingUserIDs := []string{}
	for rows.Next() {
		var id string
		var current sql.NullString
		if err := rows.Scan(&id, &current); err != nil {
			return err
		}
		code := normalizeMigrationInviteCode(current.String)
		if !current.Valid || code == "" {
			missingUserIDs = append(missingUserIDs, id)
			continue
		}
		if _, exists := seen[code]; exists {
			missingUserIDs = append(missingUserIDs, id)
			continue
		}
		if code != strings.TrimSpace(current.String) {
			missingUserIDs = append(missingUserIDs, id)
			continue
		}
		seen[code] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, userID := range missingUserIDs {
		code, err := uniqueMigrationInviteCode(seen)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, Rebind(`UPDATE users SET invite_code = ? WHERE id = ?`), code, userID); err != nil {
			return err
		}
		seen[code] = struct{}{}
	}
	return nil
}

func normalizeMigrationInviteCode(value string) string {
	code := strings.ToUpper(strings.TrimSpace(value))
	if len(code) < 6 || len(code) > 16 {
		return ""
	}
	for _, ch := range code {
		if !strings.ContainsRune(migrationInviteCodeAlphabet, ch) {
			return ""
		}
	}
	return code
}

func uniqueMigrationInviteCode(seen map[string]struct{}) (string, error) {
	for attempts := 0; attempts < 64; attempts++ {
		code, err := randomMigrationInviteCode()
		if err != nil {
			return "", err
		}
		if _, exists := seen[code]; !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique invite code")
}

func randomMigrationInviteCode() (string, error) {
	max := big.NewInt(int64(len(migrationInviteCodeAlphabet)))
	var builder strings.Builder
	builder.Grow(migrationInviteCodeLength)
	for builder.Len() < migrationInviteCodeLength {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(migrationInviteCodeAlphabet[index.Int64()])
	}
	return builder.String(), nil
}

func addColumnIfMissing(ctx context.Context, db *sql.DB, table string, column string, definition string, afterColumn string) error {
	var exists int
	err := db.QueryRowContext(ctx, Rebind(columnExistsSQL()), columnExistsArgs(table, column)...).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	statement := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
	if CurrentDialect() == DialectMySQL && strings.TrimSpace(afterColumn) != "" {
		statement += " AFTER " + afterColumn
	}
	_, err = db.ExecContext(ctx, statement)
	return err
}

func normalizeAPIAccessKeyConcurrencyDefaults(ctx context.Context, db *sql.DB) error {
	defaultStatement := `ALTER TABLE api_access_keys MODIFY COLUMN concurrency_limit INTEGER NOT NULL DEFAULT 10`
	if CurrentDialect() == DialectPostgres {
		defaultStatement = `ALTER TABLE api_access_keys ALTER COLUMN concurrency_limit SET DEFAULT 10`
	}
	if _, err := db.ExecContext(ctx, defaultStatement); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, Rebind(`
		UPDATE api_access_keys
		SET concurrency_limit = 10
		WHERE concurrency_limit IS NULL OR concurrency_limit = 1
	`))
	return err
}

func addIndexIfMissing(ctx context.Context, db *sql.DB, indexName string, statement string) error {
	var exists int
	err := db.QueryRowContext(ctx, Rebind(indexExistsSQL()), indexExistsArgs(indexName)...).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	_, err = db.ExecContext(ctx, statement)
	return err
}

func schemaBootstrapStatements() []string {
	if CurrentDialect() == DialectPostgres {
		return []string{
			`CREATE TABLE IF NOT EXISTS users (
				id VARCHAR(36) PRIMARY KEY,
				email VARCHAR(120) NOT NULL UNIQUE,
				invite_code VARCHAR(16) NULL,
				invited_by VARCHAR(36) NULL,
				invited_ip VARCHAR(64) NULL,
				password_hash VARCHAR(255) NOT NULL,
				role VARCHAR(16) NOT NULL DEFAULT 'user',
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				email_verified_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				credits NUMERIC(12,4) NOT NULL DEFAULT 0.0000
			)`,
			`CREATE TABLE IF NOT EXISTS api_providers (
				id VARCHAR(36) PRIMARY KEY,
				name VARCHAR(80) NOT NULL,
				type VARCHAR(32) NOT NULL,
				capability VARCHAR(32) NOT NULL DEFAULT 'chat_image',
				base_url VARCHAR(255) NOT NULL,
				api_key VARCHAR(255) NOT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS ai_models (
				id VARCHAR(36) PRIMARY KEY,
				provider_id VARCHAR(36) NOT NULL,
				model_name VARCHAR(120) NOT NULL,
				display_name VARCHAR(120) NOT NULL,
				capability VARCHAR(32) NOT NULL DEFAULT 'image',
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				price_1k NUMERIC(10,4) NOT NULL DEFAULT 0.0000,
				price_2k NUMERIC(10,4) NOT NULL DEFAULT 0.0000,
				price_4k NUMERIC(10,4) NOT NULL DEFAULT 0.0000,
				append_size_to_prompt BOOLEAN NOT NULL DEFAULT FALSE,
				enabled_size_tiers JSONB NULL,
				sort_order INTEGER NOT NULL DEFAULT 100,
				cost_1k NUMERIC(10,4) NOT NULL DEFAULT 0.0000,
				cost_2k NUMERIC(10,4) NOT NULL DEFAULT 0.0000,
				cost_4k NUMERIC(10,4) NOT NULL DEFAULT 0.0000,
				markup_percent NUMERIC(8,2) NOT NULL DEFAULT 0.00,
				price_change_percent NUMERIC(8,2) NOT NULL DEFAULT 0.00,
				CONSTRAINT uq_ai_models_provider_model_capability UNIQUE (provider_id, model_name, capability)
			)`,
			`CREATE TABLE IF NOT EXISTS generation_tasks (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				model_id VARCHAR(36) NOT NULL,
				provider_id VARCHAR(36) NOT NULL,
				capability VARCHAR(32) NOT NULL,
				prompt TEXT NOT NULL,
				reference_image_url TEXT NULL,
				size_tier VARCHAR(8) NOT NULL DEFAULT '1k',
				size VARCHAR(30) NULL,
				output_format VARCHAR(20) NOT NULL DEFAULT 'jpeg',
				transparent_background BOOLEAN NOT NULL DEFAULT FALSE,
				quantity INTEGER NOT NULL DEFAULT 1,
				user_ip VARCHAR(64) NOT NULL,
				cost_credits NUMERIC(12,4) NOT NULL DEFAULT 0.0000,
				model_cost_credits NUMERIC(12,4) NOT NULL DEFAULT 0.0000,
				remaining_credits NUMERIC(12,4) NOT NULL DEFAULT 0.0000,
				duration_seconds NUMERIC(10,3) NOT NULL DEFAULT 0.000,
				status VARCHAR(16) NOT NULL DEFAULT 'queued',
				error_message TEXT NULL,
				result_json JSONB NULL,
				favorite_enabled BOOLEAN NOT NULL DEFAULT FALSE,
				public_status VARCHAR(16) NOT NULL DEFAULT 'private',
				public_requested_at TIMESTAMP NULL,
				public_reviewed_at TIMESTAMP NULL,
				display_enabled BOOLEAN NOT NULL DEFAULT FALSE,
				display_note VARCHAR(500) NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS generation_result_images (
				id VARCHAR(36) PRIMARY KEY,
				task_id VARCHAR(36) NOT NULL,
				provider_id VARCHAR(36) NOT NULL,
				url_hash CHAR(64) NOT NULL,
				image_url TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT uq_generation_result_images_provider_hash UNIQUE (provider_id, url_hash)
			)`,
			`CREATE TABLE IF NOT EXISTS credit_logs (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				type VARCHAR(16) NOT NULL,
				amount NUMERIC(12,4) NOT NULL,
				balance_after NUMERIC(12,4) NOT NULL,
				remark VARCHAR(200) NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS system_settings (
				setting_key VARCHAR(80) PRIMARY KEY,
				setting_value TEXT NOT NULL,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS email_delivery_logs (
				id VARCHAR(36) PRIMARY KEY,
				category VARCHAR(40) NOT NULL,
				from_address VARCHAR(160) NOT NULL,
				recipient VARCHAR(160) NOT NULL,
				subject VARCHAR(255) NOT NULL,
				content TEXT NOT NULL,
				action_url TEXT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'sending',
				error_message TEXT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				sent_at TIMESTAMP NULL
			)`,
			`CREATE TABLE IF NOT EXISTS recharge_orders (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				out_trade_no VARCHAR(64) NOT NULL UNIQUE,
				trade_no VARCHAR(80) NULL,
				order_type VARCHAR(24) NOT NULL DEFAULT 'recharge',
				subscription_plan_id VARCHAR(36) NULL,
				amount NUMERIC(12,2) NOT NULL,
				credits NUMERIC(12,4) NOT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'pending',
				pay_url TEXT NULL,
				qr_code TEXT NULL,
				paid_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS subscription_plans (
				id VARCHAR(36) PRIMARY KEY,
				name VARCHAR(80) NOT NULL,
				description VARCHAR(300) NULL,
				amount NUMERIC(12,2) NOT NULL,
				duration_days INTEGER NOT NULL,
				quota_images INTEGER NOT NULL DEFAULT 100,
				bonus_credits NUMERIC(12,4) NOT NULL DEFAULT 0.0000,
				discount_percent NUMERIC(5,2) NOT NULL DEFAULT 0.00,
				allowed_provider_ids JSONB NULL,
				allowed_model_ids JSONB NULL,
				badge VARCHAR(40) NULL,
				sort_order INTEGER NOT NULL DEFAULT 0,
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS user_subscriptions (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL UNIQUE,
				plan_id VARCHAR(36) NOT NULL,
				plan_snapshot JSONB NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				started_at TIMESTAMP NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS redeem_codes (
				id VARCHAR(36) PRIMARY KEY,
				code VARCHAR(80) NOT NULL UNIQUE,
				credits NUMERIC(12,4) NOT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				remark VARCHAR(200) NULL,
				user_id VARCHAR(36) NULL,
				used_at TIMESTAMP NULL,
				expires_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS user_checkins (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				reward_credits NUMERIC(12,4) NOT NULL,
				checkin_date DATE NOT NULL,
				user_ip VARCHAR(64) NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				CONSTRAINT uq_user_checkins_user_date UNIQUE (user_id, checkin_date)
			)`,
			`CREATE TABLE IF NOT EXISTS user_invites (
				id VARCHAR(36) PRIMARY KEY,
				inviter_id VARCHAR(36) NOT NULL,
				invitee_id VARCHAR(36) NOT NULL UNIQUE,
				reward_credits NUMERIC(12,4) NOT NULL,
				reward_type VARCHAR(20) NOT NULL DEFAULT 'credits',
				reward_plan_id VARCHAR(36) NULL,
				reward_label VARCHAR(120) NULL,
				reward_plan_snapshot ` + JSONTextType() + ` NULL,
				invitee_reward_type VARCHAR(20) NOT NULL DEFAULT 'none',
				invitee_reward_credits NUMERIC(12,4) NOT NULL DEFAULT 0,
				invitee_reward_plan_id VARCHAR(36) NULL,
				invitee_reward_label VARCHAR(120) NULL,
				invitee_reward_plan_snapshot ` + JSONTextType() + ` NULL,
				status VARCHAR(20) NOT NULL DEFAULT 'pending',
				ip_hash VARCHAR(64) NULL,
				device_hash VARCHAR(64) NULL,
				risk_reason VARCHAR(255) NULL,
				invitee_ip VARCHAR(64) NULL,
				verified_at TIMESTAMP NULL,
				rewarded_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS invite_rebate_records (
				id VARCHAR(36) PRIMARY KEY,
				invite_id VARCHAR(36) NOT NULL,
				order_id VARCHAR(36) NOT NULL UNIQUE,
				inviter_id VARCHAR(36) NOT NULL,
				invitee_id VARCHAR(36) NOT NULL,
				order_type VARCHAR(24) NOT NULL,
				order_amount NUMERIC(12,2) NOT NULL,
				recharge_rate NUMERIC(12,4) NOT NULL,
				rebate_percent NUMERIC(6,2) NOT NULL,
				rebate_credits NUMERIC(12,4) NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS user_registration_fingerprints (
				user_id VARCHAR(36) PRIMARY KEY,
				ip_hash VARCHAR(64) NOT NULL,
				device_hash VARCHAR(64) NULL,
				user_agent_hash VARCHAR(64) NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS registration_challenges (
				token_hash VARCHAR(64) PRIMARY KEY,
				ip_hash VARCHAR(64) NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				used_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS subscription_lottery_prizes (
				id VARCHAR(36) PRIMARY KEY,
				name VARCHAR(80) NOT NULL,
				prize_type VARCHAR(20) NOT NULL DEFAULT 'subscription',
				plan_id VARCHAR(36) NOT NULL,
				weight INTEGER NOT NULL DEFAULT 1,
				daily_stock INTEGER NOT NULL DEFAULT 0,
				monthly_stock INTEGER NOT NULL DEFAULT 0,
				sort_order INTEGER NOT NULL DEFAULT 0,
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS subscription_lottery_records (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				prize_id VARCHAR(36) NOT NULL,
				prize_type VARCHAR(20) NOT NULL DEFAULT 'subscription',
				plan_id VARCHAR(36) NOT NULL,
				draw_date DATE NOT NULL,
				user_ip VARCHAR(64) NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS announcements (
				id VARCHAR(36) PRIMARY KEY,
				title VARCHAR(120) NOT NULL,
				content TEXT NOT NULL,
				display_mode VARCHAR(20) NOT NULL DEFAULT 'popup',
				target_type VARCHAR(20) NOT NULL DEFAULT 'all',
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				sort_order INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS announcement_users (
				announcement_id VARCHAR(36) NOT NULL,
				user_id VARCHAR(36) NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (announcement_id, user_id)
			)`,
			`CREATE TABLE IF NOT EXISTS announcement_receipts (
				announcement_id VARCHAR(36) NOT NULL,
				user_id VARCHAR(36) NOT NULL,
				signed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (announcement_id, user_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_announcement_receipts_user_id ON announcement_receipts (user_id)`,
			`CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
				code VARCHAR(120) PRIMARY KEY,
				client_id VARCHAR(120) NOT NULL,
				user_id VARCHAR(36) NOT NULL,
				redirect_uri VARCHAR(500) NOT NULL,
				scope VARCHAR(200) NULL,
				expires_at TIMESTAMP NOT NULL,
				used_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_oauth_codes_client_user ON oauth_authorization_codes (client_id, user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_oauth_codes_expires_at ON oauth_authorization_codes (expires_at)`,
			`CREATE TABLE IF NOT EXISTS oauth_access_tokens (
				token_hash CHAR(64) PRIMARY KEY,
				client_id VARCHAR(120) NOT NULL,
				user_id VARCHAR(36) NOT NULL,
				scope VARCHAR(200) NULL,
				expires_at TIMESTAMP NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_oauth_tokens_user_id ON oauth_access_tokens (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_oauth_tokens_expires_at ON oauth_access_tokens (expires_at)`,
			`CREATE TABLE IF NOT EXISTS user_email_tokens (
				token_hash CHAR(64) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				purpose VARCHAR(40) NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				used_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_user_email_tokens_user_id ON user_email_tokens (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_user_email_tokens_purpose ON user_email_tokens (purpose)`,
			`CREATE INDEX IF NOT EXISTS idx_user_email_tokens_expires_at ON user_email_tokens (expires_at)`,
			`CREATE TABLE IF NOT EXISTS user_api_keys (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				name VARCHAR(80) NOT NULL,
				key_prefix VARCHAR(20) NOT NULL,
				key_hash VARCHAR(64) NOT NULL UNIQUE,
				key_plain VARCHAR(255) NULL,
				encrypted_key TEXT NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				last_used_at TIMESTAMP NULL,
				deleted_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS api_access_keys (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				name VARCHAR(80) NOT NULL,
				key_prefix VARCHAR(24) NOT NULL,
				key_hash VARCHAR(64) NOT NULL UNIQUE,
				key_plain VARCHAR(255) NULL,
				status VARCHAR(16) NOT NULL DEFAULT 'active',
				concurrency_limit INTEGER NOT NULL DEFAULT 10,
				billing_mode VARCHAR(16) NULL,
				last_used_at TIMESTAMP NULL,
				deleted_at TIMESTAMP NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS api_access_logs (
				id VARCHAR(36) PRIMARY KEY,
				user_id VARCHAR(36) NOT NULL,
				api_key_id VARCHAR(36) NOT NULL,
				task_id VARCHAR(36) NULL,
				endpoint VARCHAR(80) NOT NULL,
				model VARCHAR(120) NOT NULL,
				prompt TEXT NOT NULL,
				size VARCHAR(30) NOT NULL DEFAULT '',
				quality VARCHAR(30) NOT NULL DEFAULT '',
				quantity INTEGER NOT NULL DEFAULT 1,
				image_count INTEGER NOT NULL DEFAULT 0,
				response_format VARCHAR(30) NOT NULL DEFAULT 'url',
				status VARCHAR(16) NOT NULL DEFAULT 'queued',
				error_message TEXT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				finished_at TIMESTAMP NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_ai_models_status_capability ON ai_models (status, capability)`,
			`CREATE INDEX IF NOT EXISTS idx_generation_tasks_created_at ON generation_tasks (created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_generation_tasks_user_id ON generation_tasks (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_generation_tasks_capability ON generation_tasks (capability)`,
			`CREATE INDEX IF NOT EXISTS idx_generation_tasks_user_created_id ON generation_tasks (user_id, created_at, id)`,
			`CREATE INDEX IF NOT EXISTS idx_generation_tasks_user_favorite ON generation_tasks (user_id, favorite_enabled, updated_at)`,
			`CREATE INDEX IF NOT EXISTS idx_generation_tasks_public_status ON generation_tasks (public_status, updated_at)`,
			`CREATE INDEX IF NOT EXISTS idx_generation_result_images_task_id ON generation_result_images (task_id)`,
			`CREATE INDEX IF NOT EXISTS idx_credit_logs_user_id ON credit_logs (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_credit_logs_created_at ON credit_logs (created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_credit_logs_user_created_id ON credit_logs (user_id, created_at, id)`,
			`CREATE INDEX IF NOT EXISTS idx_recharge_orders_user_id ON recharge_orders (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_recharge_orders_out_trade_no ON recharge_orders (out_trade_no)`,
			`CREATE INDEX IF NOT EXISTS idx_recharge_orders_status ON recharge_orders (status)`,
			`CREATE INDEX IF NOT EXISTS idx_subscription_plans_status_sort ON subscription_plans (status, sort_order)`,
			`CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_status ON user_subscriptions (user_id, status, expires_at)`,
			`CREATE INDEX IF NOT EXISTS idx_user_subscriptions_plan_id ON user_subscriptions (plan_id)`,
			`CREATE INDEX IF NOT EXISTS idx_redeem_codes_status ON redeem_codes (status)`,
			`CREATE INDEX IF NOT EXISTS idx_redeem_codes_user_id ON redeem_codes (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_user_checkins_date ON user_checkins (checkin_date)`,
			`CREATE INDEX IF NOT EXISTS idx_user_invites_ip_created ON user_invites (invitee_ip, created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_announcements_status_sort ON announcements (status, sort_order, created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_announcement_users_user_id ON announcement_users (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_user_api_keys_user_id ON user_api_keys (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_user_api_keys_prefix_status ON user_api_keys (key_prefix, status)`,
			`CREATE INDEX IF NOT EXISTS idx_api_access_keys_user_id ON api_access_keys (user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_api_access_keys_prefix_status ON api_access_keys (key_prefix, status)`,
			`CREATE INDEX IF NOT EXISTS idx_api_access_logs_user_created ON api_access_logs (user_id, created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_api_access_logs_key_created ON api_access_logs (api_key_id, created_at)`,
			`CREATE INDEX IF NOT EXISTS idx_api_access_logs_task_id ON api_access_logs (task_id)`,
			`CREATE INDEX IF NOT EXISTS idx_api_access_logs_status_created ON api_access_logs (status, created_at)`,
		}
	}
	return []string{
		`CREATE TABLE IF NOT EXISTS invite_rebate_records (
			id VARCHAR(36) PRIMARY KEY,
			invite_id VARCHAR(36) NOT NULL,
			order_id VARCHAR(36) NOT NULL UNIQUE,
			inviter_id VARCHAR(36) NOT NULL,
			invitee_id VARCHAR(36) NOT NULL,
			order_type VARCHAR(24) NOT NULL,
			order_amount NUMERIC(12,2) NOT NULL,
			recharge_rate NUMERIC(12,4) NOT NULL,
			rebate_percent NUMERIC(6,2) NOT NULL,
			rebate_credits NUMERIC(12,4) NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_invite_rebate_records_inviter_created (inviter_id, created_at),
			INDEX idx_invite_rebate_records_invitee_created (invitee_id, created_at),
			INDEX idx_invite_rebate_records_invite_created (invite_id, created_at)
		)`,
		`CREATE TABLE IF NOT EXISTS email_delivery_logs (
			id VARCHAR(36) PRIMARY KEY,
			category VARCHAR(40) NOT NULL,
			from_address VARCHAR(160) NOT NULL,
			recipient VARCHAR(160) NOT NULL,
			subject VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			action_url TEXT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'sending',
			error_message TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			sent_at DATETIME NULL,
			INDEX idx_email_delivery_logs_created_at (created_at),
			INDEX idx_email_delivery_logs_status_created (status, created_at),
			INDEX idx_email_delivery_logs_category_created (category, created_at),
			INDEX idx_email_delivery_logs_recipient_created (recipient, created_at)
		)`,
		`CREATE TABLE IF NOT EXISTS generation_result_images (
			id VARCHAR(36) PRIMARY KEY,
			task_id VARCHAR(36) NOT NULL,
			provider_id VARCHAR(36) NOT NULL,
			url_hash CHAR(64) NOT NULL,
			image_url TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_generation_result_images_provider_hash (provider_id, url_hash),
			INDEX idx_generation_result_images_task_id (task_id)
		)`,
		`CREATE TABLE IF NOT EXISTS api_access_keys (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			name VARCHAR(80) NOT NULL,
			key_prefix VARCHAR(24) NOT NULL,
			key_hash VARCHAR(64) NOT NULL UNIQUE,
			key_plain VARCHAR(255) NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			concurrency_limit INTEGER NOT NULL DEFAULT 10,
			billing_mode VARCHAR(16) NULL,
			last_used_at DATETIME NULL,
			deleted_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_api_access_keys_user_id (user_id),
			INDEX idx_api_access_keys_prefix_status (key_prefix, status)
		)`,
		`CREATE TABLE IF NOT EXISTS api_access_logs (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			api_key_id VARCHAR(36) NOT NULL,
			task_id VARCHAR(36) NULL,
			endpoint VARCHAR(80) NOT NULL,
			model VARCHAR(120) NOT NULL,
			prompt TEXT NOT NULL,
			size VARCHAR(30) NOT NULL DEFAULT '',
			quality VARCHAR(30) NOT NULL DEFAULT '',
			quantity INTEGER NOT NULL DEFAULT 1,
			image_count INTEGER NOT NULL DEFAULT 0,
			response_format VARCHAR(30) NOT NULL DEFAULT 'url',
			status VARCHAR(16) NOT NULL DEFAULT 'queued',
			error_message TEXT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			finished_at DATETIME NULL,
			INDEX idx_api_access_logs_user_created (user_id, created_at),
			INDEX idx_api_access_logs_key_created (api_key_id, created_at),
			INDEX idx_api_access_logs_task_id (task_id),
			INDEX idx_api_access_logs_status_created (status, created_at)
		)`,
		`CREATE TABLE IF NOT EXISTS subscription_lottery_prizes (
			id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(80) NOT NULL,
			prize_type VARCHAR(20) NOT NULL DEFAULT 'subscription',
			plan_id VARCHAR(36) NOT NULL,
			weight INTEGER NOT NULL DEFAULT 1,
			daily_stock INTEGER NOT NULL DEFAULT 0,
			monthly_stock INTEGER NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0,
			status VARCHAR(16) NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_subscription_lottery_prizes_status_sort (status, sort_order)
		)`,
		`CREATE TABLE IF NOT EXISTS subscription_lottery_records (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			prize_id VARCHAR(36) NOT NULL,
			prize_type VARCHAR(20) NOT NULL DEFAULT 'subscription',
			plan_id VARCHAR(36) NOT NULL,
			draw_date DATE NOT NULL,
			user_ip VARCHAR(64) NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_subscription_lottery_user_date (user_id, draw_date),
			INDEX idx_subscription_lottery_records_user_created (user_id, created_at),
			INDEX idx_subscription_lottery_records_prize_date (prize_id, draw_date)
		)`,
		`CREATE TABLE IF NOT EXISTS announcement_receipts (
			announcement_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			signed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (announcement_id, user_id),
			INDEX idx_announcement_receipts_user_id (user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
			code VARCHAR(120) PRIMARY KEY,
			client_id VARCHAR(120) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			redirect_uri VARCHAR(500) NOT NULL,
			scope VARCHAR(200) NULL,
			expires_at DATETIME NOT NULL,
			used_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_oauth_codes_client_user (client_id, user_id),
			INDEX idx_oauth_codes_expires_at (expires_at)
		)`,
		`CREATE TABLE IF NOT EXISTS oauth_access_tokens (
			token_hash CHAR(64) PRIMARY KEY,
			client_id VARCHAR(120) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			scope VARCHAR(200) NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_oauth_tokens_user_id (user_id),
			INDEX idx_oauth_tokens_expires_at (expires_at)
		)`,
		`CREATE TABLE IF NOT EXISTS user_email_tokens (
			token_hash CHAR(64) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			purpose VARCHAR(40) NOT NULL,
			expires_at DATETIME NOT NULL,
			used_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_email_tokens_user_id (user_id),
			INDEX idx_user_email_tokens_purpose (purpose),
			INDEX idx_user_email_tokens_expires_at (expires_at)
		)`,
	}
}

func columnExistsSQL() string {
	if CurrentDialect() == DialectPostgres {
		return `
			SELECT COUNT(*)
			FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = ?
				AND column_name = ?
		`
	}
	return `
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = ?
			AND COLUMN_NAME = ?
	`
}

func columnExistsArgs(table string, column string) []any {
	return []any{table, column}
}

func indexExistsSQL() string {
	if CurrentDialect() == DialectPostgres {
		return `
			SELECT COUNT(*)
			FROM pg_indexes
			WHERE schemaname = current_schema()
				AND indexname = ?
		`
	}
	return `
		SELECT COUNT(*)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
			AND INDEX_NAME = ?
	`
}

func indexExistsArgs(indexName string) []any {
	return []any{indexName}
}
