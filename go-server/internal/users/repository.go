package users

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

const (
	inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	inviteCodeLength   = 8
	userSelectColumns  = `id, email, invite_code, invited_by, invited_ip, password_hash, credits, role, status, sync_size, email_verified_at, created_at, updated_at`
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAll(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *user)
	}
	return items, rows.Err()
}

func (r *Repository) FindOptions(ctx context.Context, keyword string, status string, limit int) ([]User, error) {
	if limit < 1 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	conditions := []string{}
	args := []any{}
	if keyword = strings.ToLower(strings.TrimSpace(keyword)); keyword != "" {
		like := "%" + keyword + "%"
		conditions = append(conditions, "(LOWER(email) LIKE ? OR LOWER(id) LIKE ?)")
		args = append(args, like, like)
	}
	if status = strings.ToLower(strings.TrimSpace(status)); status != "" && status != "all" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		`+where+`
		ORDER BY email ASC, id ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		item, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) FindAdminPage(ctx context.Context, input AdminUserPageInput) ([]User, int, AdminUserPageStats, error) {
	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	activeSince := time.Now().AddDate(0, 0, -30)
	conditions := []string{}
	args := []any{}
	keyword := strings.ToLower(strings.TrimSpace(input.Keyword))
	if keyword != "" {
		like := "%" + keyword + "%"
		conditions = append(conditions, "(LOWER(users.email) LIKE ? OR LOWER(users.id) LIKE ?)")
		args = append(args, like, like)
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != "" && status != "all" {
		conditions = append(conditions, "users.status = ?")
		args = append(args, status)
	}
	switch strings.ToLower(strings.TrimSpace(input.Billing)) {
	case "subscription":
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM user_subscriptions active_subscription
			WHERE active_subscription.user_id = users.id
			  AND active_subscription.status = 'active'
			  AND active_subscription.expires_at > CURRENT_TIMESTAMP
		)`)
	case "payg":
		conditions = append(conditions, `NOT EXISTS (
			SELECT 1 FROM user_subscriptions active_subscription
			WHERE active_subscription.user_id = users.id
			  AND active_subscription.status = 'active'
			  AND active_subscription.expires_at > CURRENT_TIMESTAMP
		)`)
	}
	switch strings.ToLower(strings.TrimSpace(input.Activity)) {
	case "active":
		conditions = append(conditions, `users.role <> 'admin' AND EXISTS (
			SELECT 1 FROM user_ip_evidence active_evidence
			WHERE active_evidence.user_id = users.id
			  AND active_evidence.source_type IN ('login', 'api')
			  AND active_evidence.last_seen_at >= ?
		)`)
		args = append(args, activeSince)
	case "inactive":
		conditions = append(conditions, `users.role <> 'admin' AND NOT EXISTS (
			SELECT 1 FROM user_ip_evidence inactive_evidence
			WHERE inactive_evidence.user_id = users.id
			  AND inactive_evidence.source_type IN ('login', 'api')
			  AND inactive_evidence.last_seen_at >= ?
		)`)
		args = append(args, activeSince)
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users `+where, args...).Scan(&total); err != nil {
		return nil, 0, AdminUserPageStats{}, err
	}
	var stats AdminUserPageStats
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0) AS active,
			COALESCE(SUM(CASE WHEN email_verified_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS verified,
			COALESCE(SUM(CASE WHEN EXISTS (
				SELECT 1 FROM user_subscriptions active_subscription
				WHERE active_subscription.user_id = users.id
				  AND active_subscription.status = 'active'
				  AND active_subscription.expires_at > CURRENT_TIMESTAMP
			) THEN 1 ELSE 0 END), 0) AS subscribed
			,COALESCE(SUM(CASE WHEN users.role <> 'admin' AND EXISTS (
				SELECT 1 FROM user_ip_evidence active_evidence
				WHERE active_evidence.user_id = users.id
				  AND active_evidence.source_type IN ('login', 'api')
				  AND active_evidence.last_seen_at >= ?
			) THEN 1 ELSE 0 END), 0) AS active_last_30_days
		FROM users
	`, activeSince).Scan(&stats.Total, &stats.Active, &stats.Verified, &stats.Subscribed, &stats.ActiveLast30Days); err != nil {
		return nil, 0, AdminUserPageStats{}, err
	}
	queryArgs := append([]any{activeSince}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+userSelectColumns+`,
			COALESCE((SELECT ip_address FROM user_ip_evidence WHERE user_id=users.id AND source_type='login' ORDER BY last_seen_at DESC LIMIT 1), '') AS last_login_ip,
			COALESCE((SELECT ip_address FROM user_ip_evidence WHERE user_id=users.id AND source_type='api' ORDER BY last_seen_at DESC LIMIT 1), '') AS last_api_ip,
			COALESCE((SELECT 1 FROM user_ip_evidence WHERE user_id=users.id AND users.role <> 'admin' AND source_type IN ('login', 'api') AND last_seen_at >= ? LIMIT 1), 0) AS active_last_30_days
		FROM users
		`+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, AdminUserPageStats{}, err
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		item, err := scanAdminUser(rows)
		if err != nil {
			return nil, 0, AdminUserPageStats{}, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, AdminUserPageStats{}, err
	}
	return items, total, stats, nil
}

func (r *Repository) ListCreditLogs(ctx context.Context, userID string, logType string, page int, pageSize int, sort ...string) ([]CreditLog, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	where := ` WHERE user_id = ?`
	args := []any{userID}
	if logType != "" {
		where += ` AND type = ?`
		args = append(args, logType)
	}
	sortBy, sortOrder := "", ""
	if len(sort) > 0 {
		sortBy = sort[0]
	}
	if len(sort) > 1 {
		sortOrder = sort[1]
	}
	orderBy := creditLogSort(sortBy, sortOrder)

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM credit_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, type, amount, balance_after, COALESCE(remark, ''), created_at
		FROM credit_logs
	`+where+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []CreditLog{}
	for rows.Next() {
		var item CreditLog
		if err := rows.Scan(&item.ID, &item.UserID, &item.Type, &item.Amount, &item.BalanceAfter, &item.Remark, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		item.CreatedAt = appclock.DatabaseTime(item.CreatedAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	r.resolveCreditLogAdminRemarks(ctx, items)
	return items, total, nil
}

func creditLogSort(sortBy string, sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		direction = "ASC"
	}
	sortBy = strings.TrimSpace(sortBy)
	orderExpression := ""
	switch sortBy {
	case "type":
		orderExpression = "type"
	case "amount":
		orderExpression = "CASE WHEN type = 'deduct' THEN -ABS(amount) ELSE amount END"
	case "balanceAfter":
		orderExpression = "balance_after"
	case "createdAt":
		orderExpression = "created_at"
	default:
		return "created_at DESC, id DESC"
	}
	return orderExpression + " " + direction + ", created_at DESC, id DESC"
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE email = ?
		LIMIT 1
	`, email)
	return scanUser(row)
}

func (r *Repository) FindByID(ctx context.Context, id string) (*User, error) {
	legacyID := legacyIDFromCompatUUID(id)
	if legacyID != "" {
		row := r.db.QueryRowContext(ctx, `
			SELECT `+userSelectColumns+`
			FROM users
			WHERE id IN (?, ?)
			ORDER BY id = ? DESC
			LIMIT 1
		`, id, legacyID, id)
		return scanUser(row)
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE id = ?
		LIMIT 1
	`, id)
	return scanUser(row)
}

func (r *Repository) FindByInviteCode(ctx context.Context, code string) (*User, error) {
	code = NormalizeInviteCode(code)
	if code == "" {
		return nil, sql.ErrNoRows
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+userSelectColumns+`
		FROM users
		WHERE invite_code = ?
		LIMIT 1
	`, code)
	return scanUser(row)
}

func (r *Repository) Create(ctx context.Context, user User) (*User, error) {
	inviteCode := NormalizeInviteCode(user.InviteCode)
	if inviteCode == "" {
		generated, err := r.newUniqueInviteCode(ctx)
		if err != nil {
			return nil, err
		}
		inviteCode = generated
	}
	invitedBy := strings.TrimSpace(user.InvitedBy)
	invitedIP := strings.TrimSpace(user.InvitedIP)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, invite_code, invited_by, invited_ip, password_hash, credits, role, status, sync_size, email_verified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, inviteCode, nullableString(invitedBy), nullableString(invitedIP), user.PasswordHash, user.Credits, user.Role, user.Status, user.SyncSize, user.EmailVerifiedAt)
	if err != nil {
		return nil, err
	}
	return r.FindByID(ctx, user.ID)
}

func (r *Repository) EnsureInviteCode(ctx context.Context, userID string) (string, error) {
	user, err := r.FindByID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return "", err
	}
	if user.InviteCode != "" {
		return user.InviteCode, nil
	}
	code, err := r.newUniqueInviteCode(ctx)
	if err != nil {
		return "", err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET invite_code = ? WHERE id = ?`, code, user.ID); err != nil {
		return "", err
	}
	return code, nil
}

func (r *Repository) UpdatePassword(ctx context.Context, id string, passwordHash string) (*User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) MarkEmailVerified(ctx context.Context, id string) (*User, error) {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET email_verified_at = COALESCE(email_verified_at, NOW()) WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id string, input User) (*User, error) {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email = ?, role = ?, status = ?, sync_size = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.Email, input.Role, input.Status, input.SyncSize, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) SetCredits(ctx context.Context, id string, nextBalance float64, remark string) (*User, error) {
	nextBalance, ok := normalizeCredits(nextBalance)
	if !ok {
		return nil, ErrInvalidCredits
	}
	remark = strings.TrimSpace(remark)
	if remark == "" {
		remark = "管理员调整余额"
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var currentBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT credits FROM users WHERE id = ? FOR UPDATE`, id).Scan(&currentBalance); err != nil {
		return nil, err
	}
	currentBalance, _ = normalizeCredits(currentBalance)
	delta := math.Round((nextBalance-currentBalance)*10000) / 10000
	if delta != 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE users
			SET credits = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, nextBalance, id); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO credit_logs (id, user_id, type, amount, balance_after, remark)
			VALUES (?, ?, 'manual_adjust', ?, ?, ?)
		`, newRepositoryID(), id, delta, nextBalance, remark); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) AdjustCredits(ctx context.Context, id string, amount float64, remark string) (*User, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, ErrInvalidCredits
	}
	amount = math.Round(amount*10000) / 10000
	remark = strings.TrimSpace(remark)
	if remark == "" {
		remark = "管理员调整余额"
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var currentBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT credits FROM users WHERE id = ? FOR UPDATE`, id).Scan(&currentBalance); err != nil {
		return nil, err
	}
	currentBalance, _ = normalizeCredits(currentBalance)
	nextBalance, ok := normalizeCredits(currentBalance + amount)
	if !ok {
		return nil, ErrInvalidCredits
	}
	if amount != 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE users
			SET credits = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, nextBalance, id); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO credit_logs (id, user_id, type, amount, balance_after, remark)
			VALUES (?, ?, 'manual_adjust', ?, ?, ?)
		`, newRepositoryID(), id, amount, nextBalance, remark); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ? AND role <> 'admin'`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (r *Repository) DeleteMany(ctx context.Context, ids []string) (int, error) {
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(unique))
	args := make([]any, len(unique))
	for index, id := range unique {
		placeholders[index] = "?"
		args[index] = id
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE role <> 'admin' AND id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	return int(rows), err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*User, error) {
	return scanUserColumns(row, false)
}

func scanAdminUser(row scanner) (*User, error) {
	return scanUserColumns(row, true)
}

func scanUserColumns(row scanner, includeIPEvidence bool) (*User, error) {
	var user User
	var inviteCode sql.NullString
	var invitedBy sql.NullString
	var invitedIP sql.NullString
	var lastLoginIP sql.NullString
	var lastAPIIP sql.NullString
	var activeLast30Days int
	var verifiedAt sql.NullTime
	destinations := []any{
		&user.ID,
		&user.Email,
		&inviteCode,
		&invitedBy,
		&invitedIP,
		&user.PasswordHash,
		&user.Credits,
		&user.Role,
		&user.Status,
		&user.SyncSize,
		&verifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	}
	if includeIPEvidence {
		destinations = append(destinations, &lastLoginIP, &lastAPIIP, &activeLast30Days)
	}
	if err := row.Scan(destinations...); err != nil {
		return nil, err
	}
	if verifiedAt.Valid {
		value := appclock.DatabaseTime(verifiedAt.Time)
		user.EmailVerifiedAt = &value
	}
	if inviteCode.Valid {
		user.InviteCode = NormalizeInviteCode(inviteCode.String)
	}
	if invitedBy.Valid {
		user.InvitedBy = strings.TrimSpace(invitedBy.String)
	}
	if invitedIP.Valid {
		user.InvitedIP = strings.TrimSpace(invitedIP.String)
	}
	if lastLoginIP.Valid {
		user.LastLoginIP = strings.TrimSpace(lastLoginIP.String)
	}
	if lastAPIIP.Valid {
		user.LastAPIIP = strings.TrimSpace(lastAPIIP.String)
	}
	user.ActiveLast30Days = activeLast30Days > 0
	user.CreatedAt = appclock.DatabaseTime(user.CreatedAt)
	user.UpdatedAt = appclock.DatabaseTime(user.UpdatedAt)
	return &user, nil
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func NormalizeInviteCode(value string) string {
	code := strings.ToUpper(strings.TrimSpace(value))
	if len(code) < 6 || len(code) > 16 {
		return ""
	}
	for _, ch := range code {
		if !strings.ContainsRune(inviteCodeAlphabet, ch) {
			return ""
		}
	}
	return code
}

func (r *Repository) newUniqueInviteCode(ctx context.Context) (string, error) {
	for attempts := 0; attempts < 64; attempts++ {
		code, err := randomInviteCode(inviteCodeLength)
		if err != nil {
			return "", err
		}
		if _, err := r.FindByInviteCode(ctx, code); err == sql.ErrNoRows {
			return code, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("failed to generate unique invite code")
}

func randomInviteCode(length int) (string, error) {
	if length <= 0 {
		length = inviteCodeLength
	}
	max := big.NewInt(int64(len(inviteCodeAlphabet)))
	var builder strings.Builder
	builder.Grow(length)
	for builder.Len() < length {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(inviteCodeAlphabet[index.Int64()])
	}
	return builder.String(), nil
}

func normalizeCredits(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 99999999.9999 {
		return 0, false
	}
	return math.Round(value*10000) / 10000, true
}

func newRepositoryID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	value := hex.EncodeToString(bytes[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", value[0:8], value[8:12], value[12:16], value[16:20], value[20:32])
}

var compatUUIDPattern = regexp.MustCompile(`^00000000-0000-4000-8000-(\d{12})$`)

func legacyIDFromCompatUUID(id string) string {
	matches := compatUUIDPattern.FindStringSubmatch(id)
	if len(matches) != 2 {
		return ""
	}
	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return ""
	}
	return "legacy-" + strconv.Itoa(number)
}
