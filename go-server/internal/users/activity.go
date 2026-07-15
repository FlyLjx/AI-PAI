package users

import (
	"context"
	"database/sql"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

type ActivityRank struct {
	Rank          int     `json:"rank"`
	UserID        string  `json:"userId"`
	UserEmail     string  `json:"userEmail"`
	UserStatus    string  `json:"userStatus"`
	TaskCount     int     `json:"taskCount"`
	SuccessTasks  int     `json:"successTasks"`
	SuccessImages int     `json:"successImages"`
	LastActiveAt  *string `json:"lastActiveAt"`
	ActivityScore float64 `json:"activityScore"`
	WindowDays    int     `json:"windowDays"`
}

func (r *Repository) ActivityRanking(ctx context.Context, days int, limit int) ([]ActivityRank, error) {
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	since := time.Now().AddDate(0, 0, -days)
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			users.id,
			users.email,
			users.status,
			COUNT(generation_tasks.id) AS task_count,
			`+database.BoolCountExpr(`generation_tasks.status = 'success'`)+` AS success_tasks,
			COALESCE(SUM(CASE WHEN generation_tasks.status = 'success' THEN generation_tasks.quantity ELSE 0 END), 0) AS success_images,
			MAX(generation_tasks.created_at) AS last_active_at
		FROM generation_tasks
		LEFT JOIN users ON users.id = generation_tasks.user_id
		WHERE generation_tasks.created_at >= ?
			AND generation_tasks.user_id <> ''
		GROUP BY users.id, users.email, users.status
		ORDER BY success_images DESC, task_count DESC, last_active_at DESC
		LIMIT ?
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ActivityRank{}
	for rows.Next() {
		var item ActivityRank
		var lastActive sql.NullTime
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.UserStatus, &item.TaskCount, &item.SuccessTasks, &item.SuccessImages, &lastActive); err != nil {
			return nil, err
		}
		if lastActive.Valid {
			value := appclock.DatabaseTime(lastActive.Time).Format(time.RFC3339)
			item.LastActiveAt = &value
		}
		item.WindowDays = days
		item.Rank = len(items) + 1
		item.ActivityScore = float64(item.SuccessImages)*10 + float64(item.SuccessTasks)*4 + float64(item.TaskCount)*2
		items = append(items, item)
	}
	return items, rows.Err()
}
