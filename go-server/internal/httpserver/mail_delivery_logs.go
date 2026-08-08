package httpserver

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
)

type mailSender func(smtpSettings, string, string, string, ...mailAction) error

type mailDeliveryLog struct {
	ID           string  `json:"id"`
	Category     string  `json:"category"`
	FromAddress  string  `json:"fromAddress"`
	Recipient    string  `json:"recipient"`
	Subject      string  `json:"subject"`
	Content      string  `json:"content"`
	ActionURL    *string `json:"actionUrl,omitempty"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"errorMessage,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	SentAt       *string `json:"sentAt,omitempty"`
}

type mailDeliverySummary struct {
	Total   int `json:"total"`
	Sent    int `json:"sent"`
	Failed  int `json:"failed"`
	Sending int `json:"sending"`
	Today   int `json:"today"`
}

func deliverTrackedMail(
	ctx context.Context,
	db *database.DB,
	sender mailSender,
	settings smtpSettings,
	category string,
	to string,
	subject string,
	content string,
	actions ...mailAction,
) error {
	category = strings.TrimSpace(category)
	if category == "" {
		category = "system"
	}
	if err := validateMailAudience(category, content, actions); err != nil {
		return err
	}
	fromAddress := strings.TrimSpace(settings.FromAddress)
	if fromAddress == "" {
		fromAddress = strings.TrimSpace(settings.User)
	}
	actionURL := ""
	if len(actions) > 0 {
		actionURL = strings.TrimSpace(actions[0].URL)
	}
	id := newID()
	recorded := false
	if db != nil {
		_, err := db.ExecContext(ctx, `
			INSERT INTO email_delivery_logs (
				id, category, from_address, recipient, subject, content, action_url, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'sending')
		`, id, category, fromAddress, strings.TrimSpace(to), strings.TrimSpace(subject), content, nullableMailText(actionURL))
		if err != nil && ctx.Err() != nil {
			writeCtx, cancel := mailLogWriteContext(ctx)
			_, err = db.ExecContext(writeCtx, `
				INSERT INTO email_delivery_logs (
					id, category, from_address, recipient, subject, content, action_url, status
				) VALUES (?, ?, ?, ?, ?, ?, ?, 'sending')
			`, id, category, fromAddress, strings.TrimSpace(to), strings.TrimSpace(subject), content, nullableMailText(actionURL))
			cancel()
		}
		recorded = err == nil
	}

	sendErr := sender(settings, to, subject, content, actions...)
	if !recorded {
		return sendErr
	}
	if sendErr != nil {
		writeCtx, cancel := mailLogWriteContext(ctx)
		defer cancel()
		_, _ = db.ExecContext(writeCtx, `
			UPDATE email_delivery_logs
			SET status='failed', error_message=?, sent_at=NULL
			WHERE id=?
		`, sendErr.Error(), id)
		return sendErr
	}
	writeCtx, cancel := mailLogWriteContext(ctx)
	defer cancel()
	_, _ = db.ExecContext(writeCtx, `
		UPDATE email_delivery_logs
		SET status='sent', error_message=NULL, sent_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, id)
	return nil
}

func mailLogWriteContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
}

func validateMailAudience(category string, content string, actions []mailAction) error {
	if isAdminMailCategory(category) {
		return nil
	}
	if containsAdminRoute(content) {
		return newAppError(http.StatusBadRequest, "用户邮件正文不能包含管理后台地址")
	}
	for _, action := range actions {
		if containsAdminRoute(action.URL) {
			return newAppError(http.StatusBadRequest, "用户邮件按钮不能跳转到管理后台")
		}
	}
	return nil
}

func isAdminMailCategory(category string) bool {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "recharge_success", "upstream_alert", "upstream_recovery", "openai_image_alert", "openai_image_recovery":
		return true
	default:
		return false
	}
}

func containsAdminRoute(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(normalized, "/sys-admins") {
		return true
	}
	decoded, err := url.QueryUnescape(normalized)
	return err == nil && strings.Contains(decoded, "/sys-admins")
}

func nullableMailText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (r *Router) deliverMail(ctx context.Context, category string, settings smtpSettings, to string, subject string, content string, actions ...mailAction) error {
	return deliverTrackedMail(ctx, r.db, sendSMTPMail, settings, category, to, subject, content, actions...)
}

func (m *serviceNotificationManager) deliverMail(ctx context.Context, category string, settings smtpSettings, to string, subject string, content string, actions ...mailAction) error {
	return deliverTrackedMail(ctx, m.db, m.sendMail, settings, category, to, subject, content, actions...)
}

func (r *Router) adminMailLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	reconcileStaleMailLogs(ctx, r.db)
	page := queryInt(req, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := queryInt(req, "pageSize", 30)
	if pageSize < 1 {
		pageSize = 30
	}
	if pageSize > 100 {
		pageSize = 100
	}

	whereSQL, args := mailLogWhere(req)
	orderBy := mailLogOrder(req.URL.Query().Get("sortBy"), req.URL.Query().Get("sortOrder"))
	var summary mailDeliverySummary
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN status='sent' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status='sending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= CURRENT_DATE THEN 1 ELSE 0 END), 0)
		FROM email_delivery_logs
		`+whereSQL,
		args...,
	).Scan(&summary.Total, &summary.Sent, &summary.Failed, &summary.Sending, &summary.Today); err != nil {
		writeError(w, err)
		return
	}

	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, category, from_address, recipient, subject, content, action_url,
			status, error_message, created_at, sent_at
		FROM email_delivery_logs
		`+whereSQL+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, append(args, pageSize, offset)...)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rows.Close()

	items := make([]mailDeliveryLog, 0, pageSize)
	for rows.Next() {
		var item mailDeliveryLog
		var actionURL, errorMessage sql.NullString
		var createdAt time.Time
		var sentAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.Category, &item.FromAddress, &item.Recipient, &item.Subject, &item.Content,
			&actionURL, &item.Status, &errorMessage, &createdAt, &sentAt,
		); err != nil {
			writeError(w, err)
			return
		}
		if actionURL.Valid {
			item.ActionURL = &actionURL.String
		}
		if errorMessage.Valid {
			item.ErrorMessage = &errorMessage.String
		}
		item.CreatedAt = appclock.DatabaseTime(createdAt).Format(time.RFC3339)
		if sentAt.Valid {
			value := appclock.DatabaseTime(sentAt.Time).Format(time.RFC3339)
			item.SentAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"items":   items,
			"summary": summary,
		},
		"pagination": map[string]any{
			"total": summary.Total, "page": page, "pageSize": pageSize,
		},
	})
}

func reconcileStaleMailLogs(ctx context.Context, db *database.DB) {
	if db == nil {
		return
	}
	cutoff := time.Now().Add(-10 * time.Minute)
	_, _ = db.ExecContext(ctx, `
		UPDATE email_delivery_logs
		SET status='failed',
			error_message=COALESCE(error_message, '发送任务超时，未能确认投递结果'),
			sent_at=NULL
		WHERE status='sending' AND created_at < ?
	`, cutoff)
}

func mailLogWhere(req *http.Request) (string, []any) {
	conditions := []string{"1=1"}
	args := make([]any, 0, 4)
	if keyword := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("keyword"))); keyword != "" {
		pattern := "%" + keyword + "%"
		conditions = append(conditions, "(LOWER(recipient) LIKE ? OR LOWER(subject) LIKE ? OR LOWER(content) LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}
	if status := strings.TrimSpace(req.URL.Query().Get("status")); status != "" && status != "all" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if category := strings.TrimSpace(req.URL.Query().Get("category")); category != "" && category != "all" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func mailLogOrder(sortBy string, sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		direction = "ASC"
	}
	sortBy = strings.TrimSpace(sortBy)
	orderExpression := ""
	switch sortBy {
	case "subject":
		orderExpression = "LOWER(subject)"
	case "recipient":
		orderExpression = "LOWER(recipient)"
	case "category":
		orderExpression = "category"
	case "status":
		orderExpression = "status"
	case "createdAt":
		orderExpression = "created_at"
	case "sentAt":
		orderExpression = "sent_at"
	default:
		return "created_at DESC, id DESC"
	}
	return orderExpression + " " + direction + ", created_at DESC, id DESC"
}
