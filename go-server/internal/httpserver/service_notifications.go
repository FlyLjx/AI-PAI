package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
	"aipi-go/internal/settings"
)

var errServiceNotificationSuppressed = errors.New("service notification suppressed")

const (
	balanceReminderCooldown      = 24 * time.Hour
	subscriptionExpiryWindow     = 72 * time.Hour
	subscriptionReminderCooldown = 365 * 24 * time.Hour
	subscriptionSweepInterval    = time.Hour
	defaultNotificationOrigin    = "https://ai.yccc.me"
)

type serviceNotificationManager struct {
	db       *database.DB
	logger   *slog.Logger
	now      func() time.Time
	sendMail func(smtpSettings, string, string, string, ...mailAction) error

	mu   sync.Mutex
	sent map[string]time.Time
}

type subscriptionExpiryCandidate struct {
	ID        string
	UserID    string
	Email     string
	PlanName  string
	ExpiresAt time.Time
}

func newServiceNotificationManager(db *database.DB, logger *slog.Logger) *serviceNotificationManager {
	return &serviceNotificationManager{
		db:       db,
		logger:   logger,
		now:      time.Now,
		sendMail: sendSMTPMail,
		sent:     map[string]time.Time{},
	}
}

func StartServiceNotificationWorker(ctx context.Context, db *database.DB, logger *slog.Logger) {
	manager := newServiceNotificationManager(db, logger)
	go manager.runSubscriptionExpiryWorker(ctx)
	go manager.runUpstreamHealthWorker(ctx)
	go manager.runOpenAIStatusWorker(ctx)
}

func (r *Router) notifyBalanceInsufficient(userID string) {
	if r == nil || r.notifications == nil {
		return
	}
	r.notifications.notifyBalanceInsufficient(userID)
}

func (m *serviceNotificationManager) notifyBalanceInsufficient(userID string) {
	userID = strings.TrimSpace(userID)
	key := "balance:" + userID
	if userID == "" || !m.reserve(key, balanceReminderCooldown) {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.sendBalanceReminder(ctx, userID); errors.Is(err, errServiceNotificationSuppressed) {
			return
		} else if err != nil {
			m.logWarn("balance reminder email failed", "userId", userID, "error", err)
			return
		}
		m.logInfo("balance reminder email sent", "userId", userID)
	}()
}

func (m *serviceNotificationManager) sendBalanceReminder(ctx context.Context, userID string) error {
	persistentKey := "notify.balance." + userID
	alreadySent, err := m.reminderAlreadySent(ctx, persistentKey, balanceReminderCooldown)
	if err != nil {
		return err
	}
	if alreadySent {
		return errServiceNotificationSuppressed
	}
	values, err := settings.NewRepository(m.db).Get(ctx)
	if err != nil {
		return err
	}
	smtpConfig := smtpSettingsFromMap(values)
	if err := smtpConfig.validate(); err != nil {
		return err
	}
	var email, status string
	var credits float64
	if err := m.db.QueryRowContext(ctx, `
		SELECT email, credits, status
		FROM users
		WHERE id = ?
		LIMIT 1
	`, userID).Scan(&email, &credits, &status); err != nil {
		return err
	}
	email = strings.TrimSpace(email)
	if email == "" || status != "active" {
		return sql.ErrNoRows
	}
	brand := emailBrandName(anyString(values["siteName"]))
	body := fmt.Sprintf("您的账户余额不足，本次 API 调用未能继续处理。\n\n当前余额：%s\n\n请充值后重新发起调用。为避免频繁打扰，余额不足提醒在 24 小时内只会发送一次。", formatNotificationCredits(credits))
	if err := m.deliverMail(
		ctx,
		"balance_reminder",
		smtpConfig,
		email,
		brand+" 余额不足提醒",
		body,
		mailAction{Text: "立即充值", URL: notificationActionURL(anyString(values["frontendUrl"]), "/recharge")},
	); err != nil {
		return err
	}
	return m.markReminderSent(ctx, persistentKey)
}

func (m *serviceNotificationManager) runSubscriptionExpiryWorker(ctx context.Context) {
	m.checkAndLogSubscriptionExpiries(ctx)
	ticker := time.NewTicker(subscriptionSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAndLogSubscriptionExpiries(ctx)
		}
	}
}

func (m *serviceNotificationManager) checkAndLogSubscriptionExpiries(ctx context.Context) {
	if err := m.sendSubscriptionExpiryReminders(ctx); err != nil && ctx.Err() == nil {
		m.logWarn("subscription expiry reminder scan failed", "error", err)
	}
}

func (m *serviceNotificationManager) sendSubscriptionExpiryReminders(ctx context.Context) error {
	values, err := settings.NewRepository(m.db).Get(ctx)
	if err != nil {
		return err
	}
	smtpConfig := smtpSettingsFromMap(values)
	if !smtpConfig.Enabled {
		return nil
	}
	if err := smtpConfig.validate(); err != nil {
		return err
	}
	now := m.now()
	items, err := m.subscriptionExpiryCandidates(ctx, now, now.Add(subscriptionExpiryWindow))
	if err != nil {
		return err
	}
	brand := emailBrandName(anyString(values["siteName"]))
	actionURL := notificationActionURL(anyString(values["frontendUrl"]), "/subscriptions")
	for _, item := range items {
		expiresAt := appclock.DatabaseTime(item.ExpiresAt)
		key := fmt.Sprintf("subscription:%s:%d", item.ID, expiresAt.Unix())
		if !m.reserve(key, subscriptionReminderCooldown) {
			continue
		}
		persistentKey := fmt.Sprintf("notify.subscription.%s.%d", item.ID, expiresAt.Unix())
		alreadySent, err := m.reminderAlreadySent(ctx, persistentKey, subscriptionReminderCooldown)
		if err != nil {
			m.release(key)
			m.logWarn("subscription expiry reminder state check failed", "subscriptionId", item.ID, "error", err)
			continue
		}
		if alreadySent {
			continue
		}
		days := int(math.Ceil(expiresAt.Sub(now).Hours() / 24))
		if days < 1 {
			days = 1
		}
		body := fmt.Sprintf("您的订阅套餐“%s”将在 %d 天内到期。\n\n到期时间：%s\n\n为避免 API 调用在订阅到期后中断，请及时续费。", item.PlanName, days, expiresAt.Format("2006-01-02 15:04"))
		if err := m.deliverMail(ctx, "subscription_expiry", smtpConfig, item.Email, brand+" 订阅即将到期提醒", body, mailAction{Text: "查看订阅", URL: actionURL}); err != nil {
			m.release(key)
			m.logWarn("subscription expiry reminder email failed", "userId", item.UserID, "subscriptionId", item.ID, "error", err)
			continue
		}
		if err := m.markReminderSent(ctx, persistentKey); err != nil {
			m.logWarn("subscription expiry reminder state save failed", "subscriptionId", item.ID, "error", err)
		}
		m.logInfo("subscription expiry reminder email sent", "userId", item.UserID, "subscriptionId", item.ID, "expiresAt", expiresAt.Format(time.RFC3339))
	}
	return nil
}

func (m *serviceNotificationManager) reminderAlreadySent(ctx context.Context, key string, cooldown time.Duration) (bool, error) {
	var value string
	err := m.db.QueryRowContext(ctx, `
		SELECT setting_value
		FROM system_settings
		WHERE setting_key = ?
		LIMIT 1
	`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sentAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return false, nil
	}
	return m.now().Sub(sentAt) < cooldown, nil
}

func (m *serviceNotificationManager) markReminderSent(ctx context.Context, key string) error {
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO system_settings (setting_key, setting_value)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value), updated_at = CURRENT_TIMESTAMP
	`, key, m.now().Format(time.RFC3339Nano))
	return err
}

func (m *serviceNotificationManager) subscriptionExpiryCandidates(ctx context.Context, start time.Time, end time.Time) ([]subscriptionExpiryCandidate, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT user_subscriptions.id, user_subscriptions.user_id, users.email,
			user_subscriptions.expires_at, user_subscriptions.plan_snapshot,
			COALESCE(subscription_plans.name, '')
		FROM user_subscriptions
		INNER JOIN users ON users.id = user_subscriptions.user_id
		LEFT JOIN subscription_plans ON subscription_plans.id = user_subscriptions.plan_id
		WHERE user_subscriptions.status = 'active'
			AND users.status = 'active'
			AND users.email <> ''
			AND user_subscriptions.expires_at > ?
			AND user_subscriptions.expires_at <= ?
		ORDER BY user_subscriptions.expires_at ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []subscriptionExpiryCandidate{}
	for rows.Next() {
		var item subscriptionExpiryCandidate
		var snapshot sql.NullString
		var currentPlanName string
		if err := rows.Scan(&item.ID, &item.UserID, &item.Email, &item.ExpiresAt, &snapshot, &currentPlanName); err != nil {
			return nil, err
		}
		item.Email = strings.TrimSpace(item.Email)
		item.PlanName = subscriptionSnapshotName(snapshot.String)
		if item.PlanName == "" {
			item.PlanName = strings.TrimSpace(currentPlanName)
		}
		if item.PlanName == "" {
			item.PlanName = "订阅套餐"
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func subscriptionSnapshotName(snapshot string) string {
	var value struct {
		Name string `json:"name"`
	}
	if strings.TrimSpace(snapshot) == "" || json.Unmarshal([]byte(snapshot), &value) != nil {
		return ""
	}
	return strings.TrimSpace(value.Name)
}

func (m *serviceNotificationManager) reserve(key string, cooldown time.Duration) bool {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if sentAt, exists := m.sent[key]; exists && now.Sub(sentAt) < cooldown {
		return false
	}
	m.sent[key] = now
	return true
}

func (m *serviceNotificationManager) release(key string) {
	m.mu.Lock()
	delete(m.sent, key)
	m.mu.Unlock()
}

func notificationActionURL(frontendURL string, path string) string {
	frontendURL = strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	publicOrigin := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_PUBLIC_ORIGIN")), "/")
	if publicOrigin != "" && (frontendURL == "" || isLocalDevelopmentOrigin(frontendURL)) {
		frontendURL = publicOrigin
	}
	if frontendURL == "" || isLocalDevelopmentOrigin(frontendURL) {
		frontendURL = defaultNotificationOrigin
	}
	return frontendURL + "/" + strings.TrimLeft(path, "/")
}

func isLocalDevelopmentOrigin(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func formatNotificationCredits(value float64) string {
	text := fmt.Sprintf("%.4f", value)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func (m *serviceNotificationManager) logInfo(message string, args ...any) {
	if m.logger != nil {
		m.logger.Info(message, args...)
	}
}

func (m *serviceNotificationManager) logWarn(message string, args ...any) {
	if m.logger != nil {
		m.logger.Warn(message, args...)
	}
}
