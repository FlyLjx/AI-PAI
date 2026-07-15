package users

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

const (
	inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	inviteCodeLength   = 8
	userSelectColumns  = `id, email, invite_code, invited_by, invited_ip, password_hash, credits, role, status, email_verified_at, created_at, updated_at`
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
		INSERT INTO users (id, email, invite_code, invited_by, invited_ip, password_hash, credits, role, status, email_verified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, inviteCode, nullableString(invitedBy), nullableString(invitedIP), user.PasswordHash, user.Credits, user.Role, user.Status, user.EmailVerifiedAt)
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
		SET email = ?, credits = ?, role = ?, status = ?
		WHERE id = ?
	`, input.Email, input.Credits, input.Role, input.Status, id); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*User, error) {
	var user User
	var inviteCode sql.NullString
	var invitedBy sql.NullString
	var invitedIP sql.NullString
	var verifiedAt sql.NullTime
	if err := row.Scan(
		&user.ID,
		&user.Email,
		&inviteCode,
		&invitedBy,
		&invitedIP,
		&user.PasswordHash,
		&user.Credits,
		&user.Role,
		&user.Status,
		&verifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
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
