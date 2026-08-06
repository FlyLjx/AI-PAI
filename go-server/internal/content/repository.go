package content

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindAnnouncements(ctx context.Context, onlyVisible bool, userID string, includeSigned bool) ([]Announcement, error) {
	query := `
		SELECT announcements.id, announcements.title, announcements.content,
			COALESCE(announcements.display_mode, 'popup') AS display_mode,
			announcements.target_type, announcements.status, announcements.sort_order,
			COALESCE(announcements.reward_credits, 0) AS reward_credits,
			CASE WHEN EXISTS (
				SELECT 1 FROM announcement_receipts reward_receipt
				WHERE reward_receipt.announcement_id = announcements.id
				  AND reward_receipt.user_id = ?
				  AND reward_receipt.reward_claimed_at IS NOT NULL
			) THEN 1 ELSE 0 END AS reward_claimed,
			GROUP_CONCAT(DISTINCT announcement_users.user_id) AS user_ids,
			announcements.created_at, announcements.updated_at
		FROM announcements
		LEFT JOIN announcement_users ON announcement_users.announcement_id = announcements.id
	`
	args := []any{userID}
	if onlyVisible {
		query += `
			WHERE announcements.status = 'active'
			  AND (
				announcements.target_type = 'all'
				OR (? <> '' AND EXISTS (
					SELECT 1 FROM announcement_users target_users
					WHERE target_users.announcement_id = announcements.id
					  AND target_users.user_id = ?
				))
			  )
			  AND (? OR COALESCE(announcements.display_mode, 'popup') <> 'popup' OR ? = '' OR NOT EXISTS (
				SELECT 1 FROM announcement_receipts
				WHERE announcement_receipts.announcement_id = announcements.id
				  AND announcement_receipts.user_id = ?
				  AND (COALESCE(announcements.reward_credits, 0) <= 0 OR announcement_receipts.reward_claimed_at IS NOT NULL)
			  ))
		`
		args = append(args, userID, userID, includeSigned, userID, userID)
	}
	query += ` GROUP BY announcements.id ORDER BY announcements.sort_order ASC, announcements.created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Announcement{}
	for rows.Next() {
		item, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) FindAnnouncement(ctx context.Context, id string) (*Announcement, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT announcements.id, announcements.title, announcements.content,
			COALESCE(announcements.display_mode, 'popup') AS display_mode,
			announcements.target_type, announcements.status, announcements.sort_order,
			COALESCE(announcements.reward_credits, 0) AS reward_credits,
			0 AS reward_claimed,
			GROUP_CONCAT(DISTINCT announcement_users.user_id) AS user_ids,
			announcements.created_at, announcements.updated_at
		FROM announcements
		LEFT JOIN announcement_users ON announcement_users.announcement_id = announcements.id
		WHERE announcements.id = ?
		GROUP BY announcements.id
		LIMIT 1
	`, id)
	item, err := scanAnnouncement(row)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) SaveAnnouncement(ctx context.Context, item Announcement) (*Announcement, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO announcements (id, title, content, display_mode, target_type, status, sort_order, reward_credits)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			title = VALUES(title),
			content = VALUES(content),
			display_mode = VALUES(display_mode),
			target_type = VALUES(target_type),
			status = VALUES(status),
			sort_order = VALUES(sort_order),
			reward_credits = VALUES(reward_credits),
			updated_at = CURRENT_TIMESTAMP
	`, item.ID, item.Title, item.Content, defaultStringLocal(item.DisplayMode, "popup"), defaultStringLocal(item.TargetType, "all"), defaultStringLocal(item.Status, "active"), item.SortOrder, item.RewardCredits)
	if err != nil {
		return nil, err
	}
	if err := r.ReplaceAnnouncementUsers(ctx, item.ID, item.UserIDs); err != nil {
		return nil, err
	}
	return r.FindAnnouncement(ctx, item.ID)
}

func (r *Repository) DeleteAnnouncement(ctx context.Context, id string) (bool, error) {
	_, _ = r.db.ExecContext(ctx, `DELETE FROM announcement_receipts WHERE announcement_id = ?`, id)
	_, _ = r.db.ExecContext(ctx, `DELETE FROM announcement_users WHERE announcement_id = ?`, id)
	result, err := r.db.ExecContext(ctx, `DELETE FROM announcements WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (r *Repository) SignAnnouncement(ctx context.Context, announcementID string, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO announcement_receipts (announcement_id, user_id)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE signed_at = CURRENT_TIMESTAMP
	`, announcementID, userID)
	return err
}

func (r *Repository) ReplaceAnnouncementUsers(ctx context.Context, announcementID string, userIDs []string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM announcement_users WHERE announcement_id = ?`, announcementID); err != nil {
		return err
	}
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, err := r.db.ExecContext(ctx, `INSERT INTO announcement_users (announcement_id, user_id) VALUES (?, ?)`, announcementID, userID); err != nil {
			return err
		}
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAnnouncement(row scanner) (Announcement, error) {
	var item Announcement
	var userIDs sql.NullString
	var rewardClaimed int
	var createdAt, updatedAt time.Time
	if err := row.Scan(&item.ID, &item.Title, &item.Content, &item.DisplayMode, &item.TargetType, &item.Status, &item.SortOrder, &item.RewardCredits, &rewardClaimed, &userIDs, &createdAt, &updatedAt); err != nil {
		return item, err
	}
	item.RewardCredits = math.Round(item.RewardCredits*10000) / 10000
	item.RewardClaimed = rewardClaimed > 0
	item.UserIDs = splitIDs(userIDs.String)
	item.CreatedAt = appclock.DatabaseTime(createdAt).Format(time.RFC3339)
	item.UpdatedAt = appclock.DatabaseTime(updatedAt).Format(time.RFC3339)
	return item, nil
}

func (r *Repository) ClaimAnnouncementReward(ctx context.Context, announcementID string, userID string) (RewardClaimResult, error) {
	announcementID = strings.TrimSpace(announcementID)
	userID = strings.TrimSpace(userID)
	if announcementID == "" || userID == "" {
		return RewardClaimResult{}, ErrAnnouncementNotFound
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return RewardClaimResult{}, err
	}
	defer tx.Rollback()

	var title, targetType, status string
	var rewardCredits float64
	if err := tx.QueryRowContext(ctx, `
		SELECT title, COALESCE(reward_credits, 0), target_type, status
		FROM announcements
		WHERE id = ?
		LIMIT 1
		FOR UPDATE
	`, announcementID).Scan(&title, &rewardCredits, &targetType, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RewardClaimResult{}, ErrAnnouncementNotFound
		}
		return RewardClaimResult{}, err
	}
	if status != "active" {
		return RewardClaimResult{}, ErrAnnouncementInactive
	}
	rewardCredits = math.Round(rewardCredits*10000) / 10000
	if rewardCredits <= 0 {
		return RewardClaimResult{}, ErrAnnouncementNoReward
	}
	if targetType == "users" {
		var targetCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM announcement_users
			WHERE announcement_id = ? AND user_id = ?
		`, announcementID, userID).Scan(&targetCount); err != nil {
			return RewardClaimResult{}, err
		}
		if targetCount == 0 {
			return RewardClaimResult{}, ErrAnnouncementNotEligible
		}
	}

	var claimedAt sql.NullTime
	receiptErr := tx.QueryRowContext(ctx, `
		SELECT reward_claimed_at
		FROM announcement_receipts
		WHERE announcement_id = ? AND user_id = ?
		LIMIT 1
		FOR UPDATE
	`, announcementID, userID).Scan(&claimedAt)
	if receiptErr == nil && claimedAt.Valid {
		if err := tx.Commit(); err != nil {
			return RewardClaimResult{}, err
		}
		return RewardClaimResult{RewardCredits: rewardCredits, Granted: false}, nil
	}
	if receiptErr != nil && !errors.Is(receiptErr, sql.ErrNoRows) {
		return RewardClaimResult{}, receiptErr
	}

	var currentBalance float64
	if err := tx.QueryRowContext(ctx, `
		SELECT credits
		FROM users
		WHERE id = ? AND status = 'active'
		LIMIT 1
		FOR UPDATE
	`, userID).Scan(&currentBalance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RewardClaimResult{}, ErrAnnouncementNotEligible
		}
		return RewardClaimResult{}, err
	}
	currentBalance = math.Round(currentBalance*10000) / 10000
	balanceAfter := math.Round((currentBalance+rewardCredits)*10000) / 10000
	if balanceAfter > 99999999.9999 {
		return RewardClaimResult{}, ErrAnnouncementBalanceCap
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET credits = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, balanceAfter, userID); err != nil {
		return RewardClaimResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO credit_logs (id, user_id, type, amount, balance_after, remark)
		VALUES (?, ?, 'manual_adjust', ?, ?, ?)
	`, newContentID(), userID, rewardCredits, balanceAfter, "公告奖励："+title); err != nil {
		return RewardClaimResult{}, err
	}
	if errors.Is(receiptErr, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO announcement_receipts (announcement_id, user_id, signed_at, reward_claimed_at)
			VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, announcementID, userID); err != nil {
			return RewardClaimResult{}, err
		}
	} else if _, err := tx.ExecContext(ctx, `
		UPDATE announcement_receipts
		SET signed_at = CURRENT_TIMESTAMP, reward_claimed_at = CURRENT_TIMESTAMP
		WHERE announcement_id = ? AND user_id = ?
	`, announcementID, userID); err != nil {
		return RewardClaimResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RewardClaimResult{}, err
	}
	return RewardClaimResult{RewardCredits: rewardCredits, Granted: true, BalanceAfter: balanceAfter}, nil
}

func newContentID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%032x", time.Now().UTC().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func splitIDs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := []string{}
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			result = append(result, strings.TrimSpace(part))
		}
	}
	return result
}

func defaultStringLocal(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
