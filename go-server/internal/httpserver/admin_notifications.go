package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"aipi-go/internal/operations"
	"aipi-go/internal/settings"
)

const (
	upstreamHealthInitialDelay = 20 * time.Second
	upstreamAlertCooldown      = 6 * time.Hour
	upstreamStabilityThreshold = 95.0
	openAIStatusAlertCooldown  = 6 * time.Hour
)

var errNoAdminMailRecipients = errors.New("没有可接收通知的启用中管理员邮箱")

func (r *Router) notifyRechargeSuccess(order *operations.RechargeOrder) {
	if r == nil || r.notifications == nil || order == nil {
		return
	}
	r.notifications.notifyRechargeSuccess(*order)
}

func (m *serviceNotificationManager) notifyRechargeSuccess(order operations.RechargeOrder) {
	key := "recharge:" + strings.TrimSpace(order.ID)
	if order.ID == "" || !m.reserve(key, 365*24*time.Hour) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := m.sendRechargeSuccessNotification(ctx, order); errors.Is(err, errServiceNotificationSuppressed) {
			return
		} else if err != nil {
			m.logWarn("admin recharge notification failed", "orderId", order.ID, "error", err)
			return
		}
		m.logInfo("admin recharge notification sent", "orderId", order.ID, "userId", order.UserID)
	}()
}

func (m *serviceNotificationManager) sendRechargeSuccessNotification(ctx context.Context, order operations.RechargeOrder) error {
	values, err := settings.NewRepository(m.db).Get(ctx)
	if err != nil {
		return err
	}
	if !anyBool(values["adminRechargeNotificationEnabled"]) {
		return errServiceNotificationSuppressed
	}
	smtpConfig := smtpSettingsFromMap(values)
	if err := smtpConfig.validate(); err != nil {
		return err
	}

	userEmail := ""
	if order.UserEmail != nil {
		userEmail = strings.TrimSpace(*order.UserEmail)
	}
	if userEmail == "" {
		if err := m.db.QueryRowContext(ctx, `SELECT email FROM users WHERE id=? LIMIT 1`, order.UserID).Scan(&userEmail); err != nil {
			return err
		}
	}
	typeLabel := "余额充值"
	productLabel := "账户余额"
	if order.OrderType == "subscription" {
		typeLabel = "订阅购买"
		productLabel = "订阅套餐"
		if order.SubscriptionPlanID != nil && strings.TrimSpace(*order.SubscriptionPlanID) != "" {
			var planName string
			if err := m.db.QueryRowContext(ctx, `SELECT name FROM subscription_plans WHERE id=? LIMIT 1`, strings.TrimSpace(*order.SubscriptionPlanID)).Scan(&planName); err == nil && strings.TrimSpace(planName) != "" {
				productLabel = strings.TrimSpace(planName)
			} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
	}
	paidAt := "刚刚"
	if order.PaidAt != nil && strings.TrimSpace(*order.PaidAt) != "" {
		paidAt = strings.TrimSpace(*order.PaidAt)
	}
	body := fmt.Sprintf(
		"系统收到一笔支付成功订单。\n\n客户邮箱：%s\n订单类型：%s\n购买内容：%s\n支付金额：¥%.2f\n商户订单号：%s\n支付宝交易号：%s\n到账时间：%s",
		userEmail,
		typeLabel,
		productLabel,
		order.Amount,
		order.OutTradeNo,
		optionalMailValue(order.TradeNo),
		paidAt,
	)
	if order.InviteRebate != nil {
		body += fmt.Sprintf("\n邀请返利：%s 余额（%.2f%%）", formatNotificationCredits(order.InviteRebate.RebateCredits), order.InviteRebate.RebatePercent)
	}
	brand := emailBrandName(anyString(values["siteName"]))
	return m.sendAdminNotification(
		ctx,
		smtpConfig,
		"recharge_success",
		brand+" 充值成功通知",
		body,
		mailAction{Text: "查看充值流水", URL: notificationActionURL(anyString(values["frontendUrl"]), "/sys-admins/recharges")},
	)
}

func optionalMailValue(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "-"
	}
	return strings.TrimSpace(*value)
}

func (m *serviceNotificationManager) runUpstreamHealthWorker(ctx context.Context) {
	timer := time.NewTimer(upstreamHealthInitialDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			m.checkAndNotifyUpstreamHealth(ctx)
			timer.Reset(m.upstreamHealthCheckInterval(ctx))
		}
	}
}

func (m *serviceNotificationManager) upstreamHealthCheckInterval(ctx context.Context) time.Duration {
	values, err := settings.NewRepository(m.db).Get(ctx)
	if err != nil {
		return 5 * time.Minute
	}
	minutes := anyInt(values["adminUpstreamCheckIntervalMinutes"], 5)
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 1440 {
		minutes = 1440
	}
	return time.Duration(minutes) * time.Minute
}

func (m *serviceNotificationManager) checkAndNotifyUpstreamHealth(ctx context.Context) {
	if err := m.sendUpstreamHealthNotification(ctx); errors.Is(err, errServiceNotificationSuppressed) {
		return
	} else if err != nil && ctx.Err() == nil {
		m.logWarn("admin upstream health notification failed", "error", err)
	}
}

func (m *serviceNotificationManager) sendUpstreamHealthNotification(ctx context.Context) error {
	values, err := settings.NewRepository(m.db).Get(ctx)
	if err != nil {
		return err
	}
	if !anyBool(values["adminUpstreamNotificationEnabled"]) {
		return errServiceNotificationSuppressed
	}
	checkCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	snapshot := fetchUpstreamStabilitySnapshot(checkCtx)
	cancel()
	healthy, stateLabel := upstreamSnapshotHealth(snapshot)
	currentState := "unhealthy"
	if healthy {
		currentState = "healthy"
	}
	previousState, err := m.notificationState(ctx, "notify.upstream.state")
	if err != nil {
		return err
	}
	if previousState == "" && healthy {
		return m.setNotificationState(ctx, "notify.upstream.state", currentState)
	}
	if previousState == currentState && healthy {
		return errServiceNotificationSuppressed
	}
	if previousState == currentState && !healthy {
		recent, err := m.reminderAlreadySent(ctx, "notify.upstream.alert", upstreamAlertCooldown)
		if err != nil {
			return err
		}
		if recent {
			return errServiceNotificationSuppressed
		}
	}

	smtpConfig := smtpSettingsFromMap(values)
	category := "upstream_alert"
	subject := emailBrandName(anyString(values["siteName"])) + " 上游接口异常"
	actionText := "查看上游状态"
	if healthy {
		category = "upstream_recovery"
		subject = emailBrandName(anyString(values["siteName"])) + " 上游接口已恢复"
	}
	body := upstreamHealthMailBody(snapshot, healthy, stateLabel)
	sendErr := m.sendAdminNotification(
		ctx,
		smtpConfig,
		category,
		subject,
		body,
		mailAction{Text: actionText, URL: notificationActionURL(anyString(values["frontendUrl"]), "/sys-admins/upstream-apis")},
	)
	stateErr := m.setNotificationState(ctx, "notify.upstream.state", currentState)
	if !healthy {
		_ = m.markReminderSent(ctx, "notify.upstream.alert")
	}
	if sendErr != nil {
		return sendErr
	}
	return stateErr
}

func upstreamSnapshotHealth(snapshot map[string]any) (bool, string) {
	status := strings.ToLower(strings.TrimSpace(anyString(snapshot["status"])))
	reachable := anyBool(snapshot["reachable"])
	stability := anyFloat(snapshot["stability_percent"], 0)
	if !reachable {
		return false, "不可达"
	}
	if stability < upstreamStabilityThreshold {
		return false, fmt.Sprintf("稳定率 %.2f%%", stability)
	}
	switch status {
	case "unreachable", "degraded", "down", "error", "failed", "unstable", "unavailable", "unknown":
		return false, status
	case "", "ok", "healthy", "stable", "idle", "operational", "up", "available", "success":
		return true, firstNonEmpty(status, "正常")
	default:
		return true, status
	}
}

func upstreamHealthMailBody(snapshot map[string]any, healthy bool, stateLabel string) string {
	heading := "上游状态检测发现异常，请及时检查接口可用性。"
	if healthy {
		heading = "上游接口已从异常状态恢复。"
	}
	return fmt.Sprintf(
		"%s\n\n当前状态：%s\n稳定率：%.2f%%\n最近请求：%d\n成功：%d\n失败：%d\nHTTP 状态码：%d\n检测时间：%s\n异常信息：%s",
		heading,
		stateLabel,
		anyFloat(snapshot["stability_percent"], 0),
		anyInt(snapshot["total"], 0),
		anyInt(snapshot["success"], 0),
		anyInt(snapshot["failed"], 0),
		anyInt(snapshot["upstream_status_code"], 0),
		firstNonEmpty(anyString(snapshot["fetched_at"]), time.Now().Format(time.RFC3339)),
		firstNonEmpty(anyString(snapshot["error"]), "无"),
	)
}

func (m *serviceNotificationManager) runOpenAIStatusWorker(ctx context.Context) {
	timer := time.NewTimer(upstreamHealthInitialDelay + 10*time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			m.checkAndNotifyOpenAIStatus(ctx)
			timer.Reset(m.upstreamHealthCheckInterval(ctx))
		}
	}
}

func (m *serviceNotificationManager) checkAndNotifyOpenAIStatus(ctx context.Context) {
	if err := m.sendOpenAIStatusNotification(ctx); errors.Is(err, errServiceNotificationSuppressed) {
		return
	} else if err != nil && ctx.Err() == nil {
		m.logWarn("admin OpenAI image status notification failed", "error", err)
	}
}

func (m *serviceNotificationManager) sendOpenAIStatusNotification(ctx context.Context) error {
	values, err := settings.NewRepository(m.db).Get(ctx)
	if err != nil {
		return err
	}
	if !anyBool(values["adminOpenAIStatusNotificationEnabled"]) {
		return errServiceNotificationSuppressed
	}
	checkCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	snapshot := fetchOpenAIImageStatusSnapshot(checkCtx)
	cancel()
	healthy, stateLabel := openAIStatusSnapshotHealth(snapshot)
	currentState := "unhealthy:" + strings.ToLower(strings.TrimSpace(stateLabel))
	if healthy {
		currentState = "healthy"
	}
	previousState, err := m.notificationState(ctx, "notify.openai.image.state")
	if err != nil {
		return err
	}
	if previousState == "" && healthy {
		return m.setNotificationState(ctx, "notify.openai.image.state", currentState)
	}
	if previousState == currentState && healthy {
		return errServiceNotificationSuppressed
	}
	if previousState == currentState && !healthy {
		recent, err := m.reminderAlreadySent(ctx, "notify.openai.image.alert", openAIStatusAlertCooldown)
		if err != nil {
			return err
		}
		if recent {
			return errServiceNotificationSuppressed
		}
	}

	smtpConfig := smtpSettingsFromMap(values)
	category := "openai_image_alert"
	subject := emailBrandName(anyString(values["siteName"])) + " OpenAI Image 状态异常"
	actionText := "查看状态"
	if healthy {
		category = "openai_image_recovery"
		subject = emailBrandName(anyString(values["siteName"])) + " OpenAI Image 状态已恢复"
	}
	body := openAIStatusMailBody(snapshot, healthy, stateLabel)
	sendErr := m.sendAdminNotification(
		ctx,
		smtpConfig,
		category,
		subject,
		body,
		mailAction{Text: actionText, URL: notificationActionURL(anyString(values["frontendUrl"]), "/sys-admins/upstream-apis")},
	)
	stateErr := m.setNotificationState(ctx, "notify.openai.image.state", currentState)
	if !healthy {
		_ = m.markReminderSent(ctx, "notify.openai.image.alert")
	}
	if sendErr != nil {
		return sendErr
	}
	return stateErr
}

func openAIStatusSnapshotHealth(snapshot map[string]any) (bool, string) {
	if !anyBool(snapshot["reachable"]) {
		return false, "状态源不可达"
	}
	status := strings.ToLower(strings.TrimSpace(anyString(snapshot["status"])))
	label := firstNonEmpty(anyString(snapshot["statusLabel"]), status)
	switch status {
	case "", "operational", "ok", "healthy", "available", "resolved":
		return true, firstNonEmpty(label, "正常")
	default:
		return false, firstNonEmpty(label, status)
	}
}

func openAIStatusMailBody(snapshot map[string]any, healthy bool, stateLabel string) string {
	heading := "OpenAI Image 状态订阅检测到异常，请关注上游状态。"
	if healthy {
		heading = "OpenAI Image 状态已恢复。"
	}
	incidentTitle := "-"
	incidentLink := "-"
	if latest, ok := snapshot["latestImageIncident"].(openAIImageIncident); ok {
		incidentTitle = firstNonEmpty(latest.Title, "-")
		incidentLink = firstNonEmpty(latest.Link, "-")
	}
	return fmt.Sprintf(
		"%s\n\n当前状态：%s\n说明：%s\n最新事件：%s\n事件链接：%s\n检测时间：%s\n数据源：%s",
		heading,
		stateLabel,
		firstNonEmpty(anyString(snapshot["summary"]), "无"),
		incidentTitle,
		incidentLink,
		firstNonEmpty(anyString(snapshot["fetchedAt"]), time.Now().Format(time.RFC3339)),
		firstNonEmpty(anyString(snapshot["source"]), openAIStatusFeedEndpoint),
	)
}

func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func (m *serviceNotificationManager) sendAdminNotification(
	ctx context.Context,
	smtpConfig smtpSettings,
	category string,
	subject string,
	body string,
	actions ...mailAction,
) error {
	if err := smtpConfig.validate(); err != nil {
		return err
	}
	recipients, err := m.adminMailRecipients(ctx)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return errNoAdminMailRecipients
	}
	failed := 0
	for _, email := range recipients {
		if err := m.deliverMail(ctx, category, smtpConfig, email, subject, body, actions...); err != nil {
			failed++
			m.logWarn("admin email delivery failed", "category", category, "recipient", email, "error", err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("管理员邮件发送失败 %d/%d", failed, len(recipients))
	}
	return nil
}

func (m *serviceNotificationManager) adminMailRecipients(ctx context.Context) ([]string, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT email
		FROM users
		WHERE role='admin' AND status='active' AND email <> ''
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0, 2)
	seen := map[string]bool{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		email = strings.TrimSpace(email)
		key := strings.ToLower(email)
		if email == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, email)
	}
	return result, rows.Err()
}

func (m *serviceNotificationManager) notificationState(ctx context.Context, key string) (string, error) {
	var value string
	err := m.db.QueryRowContext(ctx, `
		SELECT setting_value FROM system_settings WHERE setting_key=? LIMIT 1
	`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(value), err
}

func (m *serviceNotificationManager) setNotificationState(ctx context.Context, key string, value string) error {
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO system_settings (setting_key, setting_value)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value), updated_at=CURRENT_TIMESTAMP
	`, key, value)
	return err
}
