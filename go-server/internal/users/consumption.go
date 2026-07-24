package users

import (
	"context"
	"database/sql"
	"time"

	"aipi-go/internal/appclock"
)

type ConsumptionRank struct {
	Rank         int     `json:"rank"`
	UserID       string  `json:"userId"`
	UserEmail    string  `json:"userEmail"`
	UserStatus   string  `json:"userStatus"`
	DeductCount  int     `json:"deductCount"`
	CreditsSpent float64 `json:"creditsSpent"`
	LastDeductAt *string `json:"lastDeductAt"`
	WindowDays   int     `json:"windowDays"`
}

func (r *Repository) ConsumptionRanking(ctx context.Context, days int, limit int) ([]ConsumptionRank, error) {
	if days < 0 {
		days = 0
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	where := `WHERE credit_logs.type='deduct' AND credit_logs.user_id <> '' AND COALESCE(users.role, 'user') <> 'admin'`
	args := []any{}
	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		where += ` AND credit_logs.created_at >= ?`
		args = append(args, since)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			credit_logs.user_id,
			COALESCE(users.email, ''),
			COALESCE(users.status, 'deleted'),
			COUNT(credit_logs.id) AS deduct_count,
			COALESCE(SUM(credit_logs.amount), 0) AS credits_spent,
			MAX(credit_logs.created_at) AS last_deduct_at
		FROM credit_logs
		LEFT JOIN users ON users.id = credit_logs.user_id
		`+where+`
		GROUP BY credit_logs.user_id, users.email, users.status
		ORDER BY credits_spent DESC, deduct_count DESC, last_deduct_at DESC, credit_logs.user_id ASC
		LIMIT ?
	`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ConsumptionRank, 0, limit)
	for rows.Next() {
		var item ConsumptionRank
		var lastDeduct sql.NullTime
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.UserStatus, &item.DeductCount, &item.CreditsSpent, &lastDeduct); err != nil {
			return nil, err
		}
		if lastDeduct.Valid {
			value := appclock.DatabaseTime(lastDeduct.Time).Format(time.RFC3339)
			item.LastDeductAt = &value
		}
		item.Rank = len(items) + 1
		item.WindowDays = days
		items = append(items, item)
	}
	return items, rows.Err()
}
