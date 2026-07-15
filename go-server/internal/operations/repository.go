package operations

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/appclock"
	"aipi-go/internal/database"
	"aipi-go/internal/tasks"
)

type Repository struct {
	db *database.DB
}

var (
	ErrNoLotteryPrize       = errors.New("no active lottery prize")
	ErrInvalidRechargeOrder = errors.New("充值订单的余额数量不正确")
)

const lotteryAutoThanksPrizeID = "auto-thanks"
const lotteryChanceScale = 10000
const lotteryBaselineDailyDraws = 100

func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

type PageInput struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

func (r *Repository) Dashboard(ctx context.Context) (map[string]any, error) {
	result := map[string]any{}
	totalUsers, err := r.count(ctx, `SELECT COUNT(*) FROM users`)
	if err != nil {
		return nil, err
	}
	activeUsers, err := r.count(ctx, `SELECT COUNT(*) FROM users WHERE status='active'`)
	if err != nil {
		return nil, err
	}
	todayUsers, err := r.count(ctx, `SELECT COUNT(*) FROM users WHERE created_at >= CURDATE()`)
	if err != nil {
		return nil, err
	}
	orderTotals := map[string]int{"all": 0, "paid": 0, "pending": 0, "closed": 0, "failed": 0}
	orderRows, err := r.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM recharge_orders GROUP BY status`)
	if err != nil {
		return nil, err
	}
	for orderRows.Next() {
		var status string
		var total int
		if err := orderRows.Scan(&status, &total); err != nil {
			orderRows.Close()
			return nil, err
		}
		orderTotals["all"] += total
		if _, ok := orderTotals[status]; ok {
			orderTotals[status] = total
		}
	}
	if err := orderRows.Close(); err != nil {
		return nil, err
	}
	if err := orderRows.Err(); err != nil {
		return nil, err
	}
	todayOrders := 0
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recharge_orders WHERE created_at >= CURDATE()`).Scan(&todayOrders)
	var todayPaidAmount float64
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount),0) FROM recharge_orders WHERE status='paid' AND paid_at >= CURDATE()`).Scan(&todayPaidAmount)
	var todayTasks, todayRunning, todayFailed int
	_ = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(status IN ('queued','pending','processing')),0),
			COALESCE(SUM(status IN ('failed','canceled')),0)
		FROM generation_tasks WHERE created_at >= CURDATE()
	`).Scan(&todayTasks, &todayRunning, &todayFailed)
	pendingOrders, _ := r.count(ctx, `SELECT COUNT(*) FROM recharge_orders WHERE status='pending'`)
	runningTasks, _ := r.count(ctx, `SELECT COUNT(*) FROM generation_tasks WHERE status IN ('queued','pending','processing')`)
	recentFailed, _ := r.count(ctx, `SELECT COUNT(*) FROM generation_tasks WHERE status IN ('failed','canceled') AND created_at >= DATE_SUB(NOW(), INTERVAL 24 HOUR)`)
	privateImages, _ := r.count(ctx, `SELECT COUNT(*) FROM generation_tasks WHERE status='success' AND display_enabled=FALSE`)
	activeProviders, _ := r.count(ctx, `SELECT COUNT(*) FROM api_providers WHERE status='active'`)
	disabledProviders, _ := r.count(ctx, `SELECT COUNT(*) FROM api_providers WHERE status='disabled'`)
	activeModels, _ := r.count(ctx, `SELECT COUNT(*) FROM ai_models WHERE capability='chat_image' AND status='active'`)
	disabledModels, _ := r.count(ctx, `SELECT COUNT(*) FROM ai_models WHERE capability='chat_image' AND status='disabled'`)
	taskTotal := 0
	taskQueued := 0
	taskPending := 0
	taskProcessing := 0
	taskSuccess := 0
	taskFailed := 0
	taskCanceled := 0
	taskTotalImages := 0
	taskStatRows, err := r.db.QueryContext(ctx, `
		SELECT
			status,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = 'success' THEN quantity ELSE 0 END), 0) AS total_images
		FROM generation_tasks
		GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	for taskStatRows.Next() {
		var status string
		var total int
		var images int
		if err := taskStatRows.Scan(&status, &total, &images); err != nil {
			taskStatRows.Close()
			return nil, err
		}
		taskTotal += total
		taskTotalImages += images
		switch status {
		case "queued":
			taskQueued = total
		case "pending":
			taskPending = total
		case "processing":
			taskProcessing = total
		case "success":
			taskSuccess = total
		case "failed":
			taskFailed = total
		case "canceled":
			taskCanceled = total
		}
	}
	if err := taskStatRows.Close(); err != nil {
		return nil, err
	}
	if err := taskStatRows.Err(); err != nil {
		return nil, err
	}
	var lastTask sql.NullTime
	_ = r.db.QueryRowContext(ctx, `SELECT MAX(created_at) FROM generation_tasks`).Scan(&lastTask)
	lastTaskValue := any(nil)
	if lastTask.Valid {
		lastTaskValue = appclock.DatabaseTime(lastTask.Time).Format(time.RFC3339)
	}
	result["today"] = map[string]any{
		"users": todayUsers, "orders": todayOrders, "paidAmount": todayPaidAmount,
		"tasks": todayTasks, "runningTasks": todayRunning, "failedTasks": todayFailed,
	}
	result["users"] = map[string]any{
		"total": totalUsers, "active": activeUsers,
	}
	result["orders"] = orderTotals
	result["taskStats"] = map[string]any{
		"total":       taskTotal,
		"queued":      taskQueued,
		"pending":     taskPending,
		"processing":  taskProcessing,
		"success":     taskSuccess,
		"failed":      taskFailed,
		"canceled":    taskCanceled,
		"totalImages": taskTotalImages,
	}
	result["pending"] = map[string]any{
		"pendingOrders": pendingOrders, "runningTasks": runningTasks,
		"recentFailedTasks": recentFailed, "privateImages": privateImages,
	}
	result["system"] = map[string]any{
		"api": "ok", "database": "ok", "activeProviders": activeProviders,
		"disabledProviders": disabledProviders, "activeModels": activeModels,
		"disabledModels": disabledModels, "lastTaskAt": lastTaskValue,
	}
	return result, nil
}

func (r *Repository) DashboardSummary(ctx context.Context, limit int) (map[string]any, error) {
	if limit < 1 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	result, err := r.Dashboard(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := r.DashboardOrders(ctx, limit)
	if err != nil {
		return nil, err
	}
	tasks, err := r.DashboardTasks(ctx, limit)
	if err != nil {
		return nil, err
	}
	result["recentOrders"] = orders
	result["recentTasks"] = tasks
	return result, nil
}

func (r *Repository) DashboardOrders(ctx context.Context, limit int) ([]RechargeOrder, error) {
	if limit < 1 {
		limit = 8
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT recharge_orders.id, recharge_orders.user_id, users.email, recharge_orders.out_trade_no, recharge_orders.trade_no,
			recharge_orders.order_type, recharge_orders.subscription_plan_id, recharge_orders.amount, recharge_orders.credits,
			recharge_orders.status, recharge_orders.pay_url, recharge_orders.qr_code, recharge_orders.paid_at, recharge_orders.created_at, recharge_orders.updated_at
		FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id
		ORDER BY recharge_orders.created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RechargeOrder{}
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) DashboardTasks(ctx context.Context, limit int) ([]DashboardTaskSummary, error) {
	if limit < 1 {
		limit = 8
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT generation_tasks.id, generation_tasks.user_id, users.email, generation_tasks.model_id,
			ai_models.model_name, ai_models.display_name, generation_tasks.quantity,
			generation_tasks.status, generation_tasks.created_at
		FROM generation_tasks
		LEFT JOIN users ON users.id = generation_tasks.user_id
		LEFT JOIN ai_models ON ai_models.id = generation_tasks.model_id
		ORDER BY generation_tasks.created_at DESC, generation_tasks.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DashboardTaskSummary{}
	for rows.Next() {
		var item DashboardTaskSummary
		var created time.Time
		var email, modelName, modelDisplayName sql.NullString
		if err := rows.Scan(&item.ID, &item.UserID, &email, &item.ModelID, &modelName, &modelDisplayName, &item.Quantity, &item.Status, &created); err != nil {
			return nil, err
		}
		item.UserEmail = nullString(email)
		item.ModelName = nullString(modelName)
		item.ModelDisplayName = nullString(modelDisplayName)
		item.CreatedAt = appclock.DatabaseTime(created).Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Plans(ctx context.Context, activeOnly bool) ([]SubscriptionPlan, error) {
	query := `SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent, allowed_provider_ids, allowed_model_ids, badge, sort_order, status, created_at, updated_at FROM subscription_plans`
	if activeOnly {
		query += ` WHERE status='active'`
	}
	query += ` ORDER BY sort_order ASC, amount ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SubscriptionPlan{}
	for rows.Next() {
		item, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) SavePlan(ctx context.Context, item SubscriptionPlan) (*SubscriptionPlan, error) {
	item.BonusCredits = 0
	if item.QuotaImages <= 0 {
		item.QuotaImages = defaultPlanQuotaImages(item.DurationDays)
	}
	providersJSON, _ := json.Marshal(item.AllowedProviderIDs)
	modelsJSON, _ := json.Marshal(item.AllowedModelIDs)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO subscription_plans (id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent, allowed_provider_ids, allowed_model_ids, badge, sort_order, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE name=VALUES(name), description=VALUES(description), amount=VALUES(amount), duration_days=VALUES(duration_days),
			quota_images=VALUES(quota_images), bonus_credits=VALUES(bonus_credits), discount_percent=VALUES(discount_percent), allowed_provider_ids=VALUES(allowed_provider_ids),
			allowed_model_ids=VALUES(allowed_model_ids), badge=VALUES(badge), sort_order=VALUES(sort_order), status=VALUES(status), updated_at=CURRENT_TIMESTAMP
	`, item.ID, item.Name, item.Description, item.Amount, item.DurationDays, item.QuotaImages, item.BonusCredits, item.DiscountPercent, string(providersJSON), string(modelsJSON), item.Badge, item.SortOrder, defaultStatus(item.Status))
	if err != nil {
		return nil, err
	}
	return r.FindPlan(ctx, item.ID)
}

func (r *Repository) FindPlan(ctx context.Context, id string) (*SubscriptionPlan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, description, amount, duration_days, quota_images, bonus_credits, discount_percent, allowed_provider_ids, allowed_model_ids, badge, sort_order, status, created_at, updated_at FROM subscription_plans WHERE id=? LIMIT 1`, id)
	item, err := scanPlan(row)
	return &item, err
}

func (r *Repository) DeletePlan(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM subscription_plans WHERE id=?`, id)
	return affected(result, err)
}

func (r *Repository) GrantSubscription(ctx context.Context, userID string, planID string) error {
	userID = strings.TrimSpace(userID)
	planID = strings.TrimSpace(planID)
	if userID == "" || planID == "" {
		return sql.ErrNoRows
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingUserID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? FOR UPDATE`, userID).Scan(&existingUserID); err != nil {
		return err
	}
	var activePlanID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM subscription_plans WHERE id=? AND status='active' FOR UPDATE`, planID).Scan(&activePlanID); err != nil {
		return err
	}
	if err := grantSubscriptionInTx(ctx, tx, existingUserID, activePlanID, 0, time.Now()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) CurrentSubscription(ctx context.Context, userID string, freeLimits FreeQuotaLimits) (*SubscriptionEntitlement, error) {
	entitlement, err := r.currentPaidSubscription(ctx, userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil && entitlement != nil {
		used, err := r.GenerationUsage(ctx, userID, entitlement.periodStart, entitlement.periodEnd)
		if err != nil {
			return nil, err
		}
		return entitlement.public(used), nil
	}
	freeLimits = normalizeFreeQuotaLimits(freeLimits)
	hourStart, hourEnd := currentHourWindow()
	hourUsed, err := r.GenerationUsage(ctx, userID, hourStart, hourEnd)
	if err != nil {
		return nil, err
	}
	dayStart, dayEnd := currentDayWindow()
	dayUsed, err := r.GenerationUsage(ctx, userID, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}
	monthStart, monthEnd := currentMonthWindow()
	monthUsed, err := r.GenerationUsage(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, err
	}
	hourWindow := quotaWindow("hour", "小时", freeLimits.Hourly, hourUsed, hourStart, hourEnd)
	dayWindow := quotaWindow("day", "今日", freeLimits.Daily, dayUsed, dayStart, dayEnd)
	monthWindow := quotaWindow("month", "本月", freeLimits.Monthly, monthUsed, monthStart, monthEnd)
	effectiveRemaining := minNonNegative(hourWindow.QuotaRemaining, dayWindow.QuotaRemaining, monthWindow.QuotaRemaining)
	return &SubscriptionEntitlement{
		Status:             "free",
		Tier:               "free",
		IsPaid:             false,
		PeriodStartedAt:    monthStart.In(time.Local).Format(time.RFC3339),
		PeriodEndsAt:       monthEnd.In(time.Local).Format(time.RFC3339),
		PlanName:           "免费版",
		QuotaImages:        freeLimits.Monthly,
		QuotaLimit:         freeLimits.Monthly,
		QuotaUsed:          monthUsed,
		QuotaRemaining:     monthWindow.QuotaRemaining,
		EffectiveRemaining: effectiveRemaining,
		QuotaUnlimited:     false,
		QuotaWindows:       []SubscriptionQuotaWindow{hourWindow, dayWindow, monthWindow},
	}, nil
}

func (r *Repository) CurrentSubscriptionPlan(ctx context.Context, userID string) (*SubscriptionPlan, error) {
	entitlement, err := r.currentPaidSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}
	if entitlement == nil || entitlement.plan == nil {
		return nil, sql.ErrNoRows
	}
	return entitlement.plan, nil
}

func (r *Repository) GenerationUsage(ctx context.Context, userID string, start time.Time, end time.Time) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, quantity, result_json
		FROM generation_tasks
		WHERE user_id=?
			AND status IN ('queued', 'pending', 'processing', 'success')
			AND created_at >= ?
			AND created_at < ?
	`, strings.TrimSpace(userID), start, end)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	used := 0
	for rows.Next() {
		var status string
		var quantity int
		var resultJSON sql.NullString
		if err := rows.Scan(&status, &quantity, &resultJSON); err != nil {
			return 0, err
		}
		used += generationUsageQuantity(status, quantity, resultJSON.String)
	}
	return used, rows.Err()
}

func generationUsageQuantity(status string, quantity int, resultJSON string) int {
	if quantity < 0 {
		quantity = 0
	}
	if status != "success" {
		return quantity
	}
	if actual := resultImageCount(resultJSON); actual > 0 {
		return actual
	}
	return quantity
}

func resultImageCount(resultJSON string) int {
	if strings.TrimSpace(resultJSON) == "" {
		return 0
	}
	var payload any
	if err := json.Unmarshal([]byte(resultJSON), &payload); err != nil {
		return 0
	}
	return len(tasks.ResultURLs(payload))
}

type paidSubscriptionEntitlement struct {
	*SubscriptionEntitlement
	plan        *SubscriptionPlan
	periodStart time.Time
	periodEnd   time.Time
}

func (item *paidSubscriptionEntitlement) public(used int) *SubscriptionEntitlement {
	limit := item.plan.QuotaImages
	if limit < 0 {
		limit = 0
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	item.SubscriptionEntitlement.QuotaImages = limit
	item.SubscriptionEntitlement.QuotaLimit = limit
	item.SubscriptionEntitlement.QuotaUsed = used
	item.SubscriptionEntitlement.QuotaRemaining = remaining
	item.SubscriptionEntitlement.EffectiveRemaining = remaining
	item.SubscriptionEntitlement.QuotaUnlimited = false
	return item.SubscriptionEntitlement
}

func (r *Repository) currentPaidSubscription(ctx context.Context, userID string) (*paidSubscriptionEntitlement, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT user_subscriptions.id, user_subscriptions.status, user_subscriptions.started_at, user_subscriptions.expires_at,
			subscription_plans.id, subscription_plans.name, subscription_plans.description, subscription_plans.amount,
			subscription_plans.duration_days, subscription_plans.quota_images, subscription_plans.bonus_credits, subscription_plans.discount_percent,
			subscription_plans.allowed_provider_ids, subscription_plans.allowed_model_ids, subscription_plans.badge,
			subscription_plans.sort_order, subscription_plans.status, subscription_plans.created_at, subscription_plans.updated_at
		FROM user_subscriptions
		INNER JOIN subscription_plans ON subscription_plans.id = user_subscriptions.plan_id
		WHERE user_subscriptions.user_id = ?
			AND user_subscriptions.status = 'active'
			AND user_subscriptions.expires_at > NOW()
			AND subscription_plans.status = 'active'
		ORDER BY user_subscriptions.expires_at DESC
		LIMIT 1
	`, strings.TrimSpace(userID))
	var subscriptionID, subscriptionStatus string
	var started, expires time.Time
	plan, err := scanPlanWithPrefix(row, &subscriptionID, &subscriptionStatus, &started, &expires)
	if err != nil {
		return nil, err
	}
	return &paidSubscriptionEntitlement{
		SubscriptionEntitlement: &SubscriptionEntitlement{
			ID:                 subscriptionID,
			Status:             subscriptionStatus,
			Tier:               "paid",
			IsPaid:             true,
			StartedAt:          appclock.DatabaseTime(started).Format(time.RFC3339),
			ExpiresAt:          appclock.DatabaseTime(expires).Format(time.RFC3339),
			PeriodStartedAt:    appclock.DatabaseTime(started).Format(time.RFC3339),
			PeriodEndsAt:       appclock.DatabaseTime(expires).Format(time.RFC3339),
			PlanID:             plan.ID,
			PlanName:           plan.Name,
			DiscountPercent:    plan.DiscountPercent,
			AllowedProviderIDs: plan.AllowedProviderIDs,
			AllowedModelIDs:    plan.AllowedModelIDs,
			Plan:               plan,
		},
		plan:        plan,
		periodStart: started,
		periodEnd:   expires,
	}, nil
}

func currentMonthWindow() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	return start, start.AddDate(0, 1, 0)
}

func currentHourWindow() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.Local)
	return start, start.Add(time.Hour)
}

func currentDayWindow() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return start, start.AddDate(0, 0, 1)
}

func normalizeFreeQuotaLimits(limits FreeQuotaLimits) FreeQuotaLimits {
	if limits.Hourly < 0 {
		limits.Hourly = 0
	}
	if limits.Daily < 0 {
		limits.Daily = 0
	}
	if limits.Monthly < 0 {
		limits.Monthly = 0
	}
	return limits
}

func quotaWindow(key string, label string, limit int, used int, start time.Time, end time.Time) SubscriptionQuotaWindow {
	if limit < 0 {
		limit = 0
	}
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return SubscriptionQuotaWindow{
		Key:             key,
		Label:           label,
		QuotaLimit:      limit,
		QuotaUsed:       used,
		QuotaRemaining:  remaining,
		PeriodStartedAt: start.In(time.Local).Format(time.RFC3339),
		PeriodEndsAt:    end.In(time.Local).Format(time.RFC3339),
	}
}

func minNonNegative(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	if min < 0 {
		min = 0
	}
	for _, value := range values[1:] {
		if value < 0 {
			value = 0
		}
		if value < min {
			min = value
		}
	}
	return min
}

func defaultPlanQuotaImages(durationDays int) int {
	switch {
	case durationDays <= 1:
		return 20
	case durationDays <= 31:
		return 300
	case durationDays <= 92:
		return 1000
	default:
		return 100
	}
}

func (r *Repository) Invites(ctx context.Context, input PageInput) ([]Invite, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	total, err := r.count(ctx, `SELECT COUNT(*) FROM user_invites`)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_invites.id, user_invites.inviter_id, inviter.email, user_invites.invitee_id, invitee.email,
			user_invites.reward_credits,
			COALESCE(user_invites.reward_type, 'subscription') AS reward_type,
			user_invites.reward_plan_id,
			user_invites.reward_label,
			user_invites.invitee_ip,
			user_invites.created_at
		FROM user_invites
		LEFT JOIN users inviter ON inviter.id=user_invites.inviter_id
		LEFT JOIN users invitee ON invitee.id=user_invites.invitee_id
		ORDER BY user_invites.created_at DESC LIMIT ? OFFSET ?
	`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Invite{}
	for rows.Next() {
		item, err := scanInvite(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *Repository) InviteSummary(ctx context.Context, userID string) (map[string]any, error) {
	var count int
	var subscriptionRewards int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN COALESCE(reward_type, 'subscription') = 'subscription' THEN 1 ELSE 0 END),0)
		FROM user_invites
		WHERE inviter_id=?
	`, userID).Scan(&count, &subscriptionRewards); err != nil {
		return nil, err
	}

	records, err := r.inviteRecords(ctx, `user_invites.inviter_id=?`, userID, 50)
	if err != nil {
		return nil, err
	}
	receivedRecords, err := r.inviteRecords(ctx, `user_invites.invitee_id=?`, userID, 1)
	if err != nil {
		return nil, err
	}
	var receivedInvite any
	if len(receivedRecords) > 0 {
		receivedInvite = receivedRecords[0]
	}

	return map[string]any{
		"inviteCount":              count,
		"total":                    count,
		"totalSubscriptionRewards": subscriptionRewards,
		"records":                  records,
		"receivedInvite":           receivedInvite,
	}, nil
}

func (r *Repository) inviteRecords(ctx context.Context, where string, arg any, limit int) ([]Invite, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_invites.id, user_invites.inviter_id, inviter.email, user_invites.invitee_id, invitee.email,
			user_invites.reward_credits,
			COALESCE(user_invites.reward_type, 'subscription') AS reward_type,
			user_invites.reward_plan_id,
			user_invites.reward_label,
			user_invites.invitee_ip,
			user_invites.created_at
		FROM user_invites
		LEFT JOIN users inviter ON inviter.id=user_invites.inviter_id
		LEFT JOIN users invitee ON invitee.id=user_invites.invitee_id
		WHERE `+where+`
		ORDER BY user_invites.created_at DESC LIMIT ?
	`, arg, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Invite{}
	for rows.Next() {
		item, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) RewardInvite(ctx context.Context, inviterID string, inviteeID string, reward float64, ip string) error {
	return nil
}

func (r *Repository) RewardInviteSubscription(ctx context.Context, inviterID string, inviteeID string, planID string, ip string) error {
	inviterID = strings.TrimSpace(inviterID)
	inviteeID = strings.TrimSpace(inviteeID)
	planID = strings.TrimSpace(planID)
	if inviterID == "" || inviteeID == "" || inviterID == inviteeID || planID == "" {
		return nil
	}
	plan, err := r.FindPlan(ctx, planID)
	if err == sql.ErrNoRows || plan == nil || plan.Status != "active" || plan.DurationDays <= 0 {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var activeUserID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? AND status='active' FOR UPDATE`, inviterID).Scan(&activeUserID); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_invites WHERE invitee_id=?`, inviteeID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return tx.Commit()
	}
	now := time.Now()
	baseTime := now
	var currentExpires sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT expires_at FROM user_subscriptions WHERE user_id=? FOR UPDATE`, inviterID).Scan(&currentExpires)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if currentExpires.Valid && currentExpires.Time.After(now) {
		baseTime = currentExpires.Time
	}
	expiresAt := baseTime.AddDate(0, 0, plan.DurationDays)
	if err == sql.ErrNoRows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_subscriptions (id, user_id, plan_id, status, started_at, expires_at)
			VALUES (?, ?, ?, 'active', ?, ?)
		`, newOperationID(), inviterID, plan.ID, now, expiresAt); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_subscriptions
			SET plan_id=?, status='active', started_at=?, expires_at=?, updated_at=CURRENT_TIMESTAMP
			WHERE user_id=?
		`, plan.ID, now, expiresAt, inviterID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_invites (id, inviter_id, invitee_id, reward_credits, reward_type, reward_plan_id, reward_label, invitee_ip)
		VALUES (?, ?, ?, 0, 'subscription', ?, ?, ?)
	`, newOperationID(), inviterID, inviteeID, plan.ID, plan.Name, ip); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) DeleteInvite(ctx context.Context, id string) (*InviteDeleteResult, error) {
	id = strings.TrimSpace(id)
	result := &InviteDeleteResult{}
	if id == "" {
		return result, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var inviterID string
	var rewardType string
	var rewardPlanID sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT inviter_id, COALESCE(reward_type, ''), reward_plan_id
		FROM user_invites
		WHERE id=?
		FOR UPDATE
	`, id).Scan(&inviterID, &rewardType, &rewardPlanID)
	if err == sql.ErrNoRows {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err != nil {
		return nil, err
	}

	result.InviterID = inviterID
	if rewardType == "subscription" && rewardPlanID.Valid && strings.TrimSpace(rewardPlanID.String) != "" {
		revoked, days, err := revokeInviteSubscriptionReward(ctx, tx, inviterID, rewardPlanID.String)
		if err != nil {
			return nil, err
		}
		result.SubscriptionRevoked = revoked
		result.RevokedDays = days
	}

	deleteResult, err := tx.ExecContext(ctx, `DELETE FROM user_invites WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	deletedRows, err := deleteResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	result.Deleted = deletedRows > 0
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func revokeInviteSubscriptionReward(ctx context.Context, tx *database.Tx, inviterID string, planID string) (bool, int, error) {
	inviterID = strings.TrimSpace(inviterID)
	planID = strings.TrimSpace(planID)
	if inviterID == "" || planID == "" {
		return false, 0, nil
	}
	var durationDays int
	if err := tx.QueryRowContext(ctx, `SELECT duration_days FROM subscription_plans WHERE id=?`, planID).Scan(&durationDays); err != nil {
		if err == sql.ErrNoRows {
			return false, 0, nil
		}
		return false, 0, err
	}
	if durationDays <= 0 {
		return false, 0, nil
	}

	var expiresAt time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT expires_at
		FROM user_subscriptions
		WHERE user_id=?
		FOR UPDATE
	`, inviterID).Scan(&expiresAt)
	if err == sql.ErrNoRows {
		return false, durationDays, nil
	}
	if err != nil {
		return false, 0, err
	}

	now := time.Now()
	newExpiresAt := expiresAt.AddDate(0, 0, -durationDays)
	if !newExpiresAt.After(now) {
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_subscriptions
			SET status='expired', expires_at=?, updated_at=CURRENT_TIMESTAMP
			WHERE user_id=?
		`, newExpiresAt, inviterID); err != nil {
			return false, 0, err
		}
		return true, durationDays, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET expires_at=?, updated_at=CURRENT_TIMESTAMP
		WHERE user_id=?
	`, newExpiresAt, inviterID); err != nil {
		return false, 0, err
	}
	return true, durationDays, nil
}

func (r *Repository) LotteryPrizes(ctx context.Context, activeOnly bool) ([]LotteryPrize, error) {
	monthStart, nextMonth, _, _ := lotteryMonthWindow(time.Now())
	query := `
		SELECT subscription_lottery_prizes.id,
			subscription_lottery_prizes.name,
			COALESCE(subscription_lottery_prizes.prize_type, 'subscription'),
			subscription_lottery_prizes.plan_id,
			subscription_plans.name,
			subscription_plans.duration_days,
			subscription_plans.quota_images,
			subscription_lottery_prizes.weight,
			subscription_lottery_prizes.daily_stock,
			COALESCE(today.used, 0),
			subscription_lottery_prizes.monthly_stock,
			COALESCE(month_used.used, 0),
			subscription_lottery_prizes.sort_order,
			subscription_lottery_prizes.status,
			subscription_lottery_prizes.created_at,
			subscription_lottery_prizes.updated_at
		FROM subscription_lottery_prizes
		LEFT JOIN subscription_plans ON subscription_plans.id = subscription_lottery_prizes.plan_id
		LEFT JOIN (
			SELECT prize_id, COUNT(*) AS used
			FROM subscription_lottery_records
			WHERE draw_date = CURDATE()
			GROUP BY prize_id
		) today ON today.prize_id = subscription_lottery_prizes.id
		LEFT JOIN (
			SELECT prize_id, COUNT(*) AS used
			FROM subscription_lottery_records
			WHERE draw_date >= ? AND draw_date < ? AND COALESCE(prize_type, 'subscription')='subscription'
			GROUP BY prize_id
		) month_used ON month_used.prize_id = subscription_lottery_prizes.id
	`
	query += ` WHERE COALESCE(subscription_lottery_prizes.prize_type, 'subscription')='subscription'`
	if activeOnly {
		query += ` AND subscription_lottery_prizes.status='active' AND subscription_plans.status='active'`
	}
	query += ` ORDER BY subscription_lottery_prizes.sort_order ASC, subscription_lottery_prizes.created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, monthStart, nextMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []LotteryPrize{}
	for rows.Next() {
		item, err := scanLotteryPrize(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeLotteryPrizeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "thanks", "none", "no_prize":
		return "thanks"
	default:
		return "subscription"
	}
}

func isThanksLotteryPrize(item LotteryPrize) bool {
	return normalizeLotteryPrizeType(item.PrizeType) == "thanks"
}

func normalizeLotteryPrizeInput(item LotteryPrize) LotteryPrize {
	item.Name = strings.TrimSpace(item.Name)
	item.PrizeType = "subscription"
	item.PlanID = strings.TrimSpace(item.PlanID)
	if item.Weight <= 0 {
		item.Weight = 1
	}
	if item.DailyStock < 0 {
		item.DailyStock = 0
	}
	if item.MonthlyStock < 0 {
		item.MonthlyStock = 0
	}
	return item
}

func (r *Repository) CreateLotteryPrize(ctx context.Context, item LotteryPrize) (*LotteryPrize, error) {
	item = normalizeLotteryPrizeInput(item)
	if item.ID == "" {
		item.ID = newOperationID()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO subscription_lottery_prizes (id, name, prize_type, plan_id, weight, daily_stock, monthly_stock, sort_order, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.Name, item.PrizeType, item.PlanID, item.Weight, item.DailyStock, item.MonthlyStock, item.SortOrder, defaultStatus(item.Status))
	if err != nil {
		return nil, err
	}
	return r.FindLotteryPrize(ctx, item.ID)
}

func (r *Repository) UpdateLotteryPrize(ctx context.Context, id string, item LotteryPrize) (*LotteryPrize, error) {
	item = normalizeLotteryPrizeInput(item)
	_, err := r.db.ExecContext(ctx, `
		UPDATE subscription_lottery_prizes
		SET name=?, prize_type=?, plan_id=?, weight=?, daily_stock=?, monthly_stock=?, sort_order=?, status=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, item.Name, item.PrizeType, item.PlanID, item.Weight, item.DailyStock, item.MonthlyStock, item.SortOrder, defaultStatus(item.Status), strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return r.FindLotteryPrize(ctx, id)
}

func (r *Repository) FindLotteryPrize(ctx context.Context, id string) (*LotteryPrize, error) {
	monthStart, nextMonth, _, _ := lotteryMonthWindow(time.Now())
	row := r.db.QueryRowContext(ctx, `
		SELECT subscription_lottery_prizes.id,
			subscription_lottery_prizes.name,
			COALESCE(subscription_lottery_prizes.prize_type, 'subscription'),
			subscription_lottery_prizes.plan_id,
			subscription_plans.name,
			subscription_plans.duration_days,
			subscription_plans.quota_images,
			subscription_lottery_prizes.weight,
			subscription_lottery_prizes.daily_stock,
			COALESCE(today.used, 0),
			subscription_lottery_prizes.monthly_stock,
			COALESCE(month_used.used, 0),
			subscription_lottery_prizes.sort_order,
			subscription_lottery_prizes.status,
			subscription_lottery_prizes.created_at,
			subscription_lottery_prizes.updated_at
		FROM subscription_lottery_prizes
		LEFT JOIN subscription_plans ON subscription_plans.id = subscription_lottery_prizes.plan_id
		LEFT JOIN (
			SELECT prize_id, COUNT(*) AS used
			FROM subscription_lottery_records
			WHERE draw_date = CURDATE()
			GROUP BY prize_id
		) today ON today.prize_id = subscription_lottery_prizes.id
		LEFT JOIN (
			SELECT prize_id, COUNT(*) AS used
			FROM subscription_lottery_records
			WHERE draw_date >= ? AND draw_date < ? AND COALESCE(prize_type, 'subscription')='subscription'
			GROUP BY prize_id
		) month_used ON month_used.prize_id = subscription_lottery_prizes.id
		WHERE subscription_lottery_prizes.id=?
		LIMIT 1
	`, monthStart, nextMonth, strings.TrimSpace(id))
	item, err := scanLotteryPrize(row)
	return &item, err
}

func (r *Repository) DeleteLotteryPrize(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM subscription_lottery_prizes WHERE id=?`, strings.TrimSpace(id))
	return affected(result, err)
}

func (r *Repository) LotteryRecords(ctx context.Context, input PageInput) ([]LotteryRecord, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where := []string{}
	args := []any{}
	if input.Keyword != "" {
		keyword := "%" + input.Keyword + "%"
		where = append(where, `(users.email LIKE ? OR subscription_lottery_prizes.name LIKE ? OR subscription_plans.name LIKE ?)`)
		args = append(args, keyword, keyword, keyword)
	}
	whereSQL := buildWhere(where)
	total, err := r.countWithArgs(ctx, `
		SELECT COUNT(*)
		FROM subscription_lottery_records
		LEFT JOIN users ON users.id = subscription_lottery_records.user_id
		LEFT JOIN subscription_lottery_prizes ON subscription_lottery_prizes.id = subscription_lottery_records.prize_id
		LEFT JOIN subscription_plans ON subscription_plans.id = subscription_lottery_records.plan_id
		`+whereSQL, args)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT subscription_lottery_records.id,
			subscription_lottery_records.user_id,
			users.email,
			subscription_lottery_records.prize_id,
			subscription_lottery_prizes.name,
			COALESCE(subscription_lottery_records.prize_type, subscription_lottery_prizes.prize_type, 'subscription'),
			subscription_lottery_records.plan_id,
			subscription_plans.name,
			subscription_plans.duration_days,
			subscription_lottery_records.draw_date,
			subscription_lottery_records.user_ip,
			subscription_lottery_records.created_at
		FROM subscription_lottery_records
		LEFT JOIN users ON users.id = subscription_lottery_records.user_id
		LEFT JOIN subscription_lottery_prizes ON subscription_lottery_prizes.id = subscription_lottery_records.prize_id
		LEFT JOIN subscription_plans ON subscription_plans.id = subscription_lottery_records.plan_id
		`+whereSQL+`
		ORDER BY subscription_lottery_records.created_at DESC
		LIMIT ? OFFSET ?
	`, append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []LotteryRecord{}
	for rows.Next() {
		item, err := scanLotteryRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *Repository) TodayLotteryRecord(ctx context.Context, userID string) (*LotteryRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT subscription_lottery_records.id,
			subscription_lottery_records.user_id,
			users.email,
			subscription_lottery_records.prize_id,
			subscription_lottery_prizes.name,
			COALESCE(subscription_lottery_records.prize_type, subscription_lottery_prizes.prize_type, 'subscription'),
			subscription_lottery_records.plan_id,
			subscription_plans.name,
			subscription_plans.duration_days,
			subscription_lottery_records.draw_date,
			subscription_lottery_records.user_ip,
			subscription_lottery_records.created_at
		FROM subscription_lottery_records
		LEFT JOIN users ON users.id = subscription_lottery_records.user_id
		LEFT JOIN subscription_lottery_prizes ON subscription_lottery_prizes.id = subscription_lottery_records.prize_id
		LEFT JOIN subscription_plans ON subscription_plans.id = subscription_lottery_records.plan_id
		WHERE subscription_lottery_records.user_id=? AND subscription_lottery_records.draw_date = CURDATE()
		LIMIT 1
	`, strings.TrimSpace(userID))
	item, err := scanLotteryRecord(row)
	return &item, err
}

func (r *Repository) DrawSubscriptionLottery(ctx context.Context, userID string, ip string) (*LotteryDrawResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, sql.ErrNoRows
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var activeUserID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=? AND status='active' FOR UPDATE`, userID).Scan(&activeUserID); err != nil {
		return nil, err
	}

	existing, err := lotteryRecordByUserToday(ctx, tx, userID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		prize, _ := r.FindLotteryPrize(ctx, existing.PrizeID)
		won := normalizeLotteryPrizeType(existing.PrizeType) != "thanks"
		result := &LotteryDrawResult{DrawnToday: true, Record: *existing, Won: won, Message: "今天已经抽过了"}
		if prize != nil {
			result.Prize = *prize
		}
		return result, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	prizes, err := availableLotteryPrizes(ctx, tx)
	if err != nil {
		return nil, err
	}
	if len(prizes) == 0 {
		prizes = []LotteryPrize{virtualThanksLotteryPrize()}
	} else {
		monthStart, nextMonth, _, _ := lotteryMonthWindow(time.Now())
		monthDrawCount, err := lotteryMonthDrawCount(ctx, tx, monthStart, nextMonth)
		if err != nil {
			return nil, err
		}
		activeUserCount, err := lotteryActiveUserCount(ctx, tx)
		if err != nil {
			return nil, err
		}
		shouldHit, err := shouldHitLotteryPrize(prizes, time.Now(), monthDrawCount, activeUserCount)
		if err != nil {
			return nil, err
		}
		if !shouldHit {
			prizes = []LotteryPrize{virtualThanksLotteryPrize()}
		}
	}
	selected, err := chooseLotteryPrize(prizes)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	won := !isThanksLotteryPrize(selected)
	message := "恭喜中奖，订阅权益已发放"
	if won {
		if err := grantSubscriptionInTx(ctx, tx, userID, selected.PlanID, selected.DurationDays, now); err != nil {
			return nil, err
		}
	} else {
		selected.PlanID = ""
		message = "谢谢惠顾，明天再来试试"
	}
	recordID := newOperationID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO subscription_lottery_records (id, user_id, prize_id, prize_type, plan_id, draw_date, user_ip)
		VALUES (?, ?, ?, ?, ?, CURDATE(), ?)
	`, recordID, userID, selected.ID, selected.PrizeType, selected.PlanID, strings.TrimSpace(ip)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	record, err := r.findLotteryRecordByID(ctx, recordID)
	if err != nil {
		return nil, err
	}
	return &LotteryDrawResult{
		DrawnToday: false,
		Record:     *record,
		Prize:      selected,
		Won:        won,
		Message:    message,
	}, nil
}

func scanLotteryPrize(row interface{ Scan(dest ...any) error }) (LotteryPrize, error) {
	var item LotteryPrize
	var planName sql.NullString
	var durationDays, quotaImages sql.NullInt64
	var created, updated time.Time
	err := row.Scan(
		&item.ID,
		&item.Name,
		&item.PrizeType,
		&item.PlanID,
		&planName,
		&durationDays,
		&quotaImages,
		&item.Weight,
		&item.DailyStock,
		&item.TodayUsed,
		&item.MonthlyStock,
		&item.MonthUsed,
		&item.SortOrder,
		&item.Status,
		&created,
		&updated,
	)
	if err != nil {
		return item, err
	}
	item.PrizeType = normalizeLotteryPrizeType(item.PrizeType)
	if isThanksLotteryPrize(item) {
		item.Name = "谢谢惠顾"
	}
	item.PlanName = nullString(planName)
	if durationDays.Valid {
		item.DurationDays = int(durationDays.Int64)
	}
	if quotaImages.Valid {
		item.QuotaImages = int(quotaImages.Int64)
	}
	if item.DailyStock <= 0 {
		item.RemainingText = "不限"
	} else {
		remaining := item.DailyStock - item.TodayUsed
		if remaining < 0 {
			remaining = 0
		}
		item.RemainingText = strconv.Itoa(remaining) + "/" + strconv.Itoa(item.DailyStock)
	}
	if item.MonthlyStock <= 0 {
		item.MonthlyText = "本月不限"
	} else {
		monthlyRemaining := item.MonthlyStock - item.MonthUsed
		if monthlyRemaining < 0 {
			monthlyRemaining = 0
		}
		item.MonthlyText = strconv.Itoa(monthlyRemaining) + "/" + strconv.Itoa(item.MonthlyStock)
	}
	item.CreatedAt = appclock.DatabaseTime(created).Format(time.RFC3339)
	item.UpdatedAt = appclock.DatabaseTime(updated).Format(time.RFC3339)
	return item, nil
}

func scanLotteryRecord(row interface{ Scan(dest ...any) error }) (LotteryRecord, error) {
	var item LotteryRecord
	var userEmail, prizeName, planName, userIP sql.NullString
	var durationDays sql.NullInt64
	var drawDate any
	var created time.Time
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&userEmail,
		&item.PrizeID,
		&prizeName,
		&item.PrizeType,
		&item.PlanID,
		&planName,
		&durationDays,
		&drawDate,
		&userIP,
		&created,
	)
	if err != nil {
		return item, err
	}
	item.PrizeType = normalizeLotteryPrizeType(item.PrizeType)
	item.UserEmail = nullString(userEmail)
	if item.PrizeType == "thanks" {
		prizeLabel := "谢谢惠顾"
		item.PrizeName = &prizeLabel
	} else {
		item.PrizeName = nullString(prizeName)
	}
	item.PlanName = nullString(planName)
	if durationDays.Valid {
		item.DurationDays = int(durationDays.Int64)
	}
	item.DrawDate = sqlDateString(drawDate)
	item.UserIP = nullString(userIP)
	item.CreatedAt = appclock.DatabaseTime(created).Format(time.RFC3339)
	return item, nil
}

func (r *Repository) findLotteryRecordByID(ctx context.Context, id string) (*LotteryRecord, error) {
	row := r.db.QueryRowContext(ctx, lotteryRecordSelectSQL()+`
		WHERE subscription_lottery_records.id=?
		LIMIT 1
	`, strings.TrimSpace(id))
	item, err := scanLotteryRecord(row)
	return &item, err
}

func lotteryRecordByUserToday(ctx context.Context, tx *database.Tx, userID string) (*LotteryRecord, error) {
	row := tx.QueryRowContext(ctx, lotteryRecordSelectSQL()+`
		WHERE subscription_lottery_records.user_id=? AND subscription_lottery_records.draw_date = CURDATE()
		LIMIT 1
	`, strings.TrimSpace(userID))
	item, err := scanLotteryRecord(row)
	return &item, err
}

func lotteryRecordSelectSQL() string {
	return `
		SELECT subscription_lottery_records.id,
			subscription_lottery_records.user_id,
			users.email,
			subscription_lottery_records.prize_id,
			subscription_lottery_prizes.name,
			COALESCE(subscription_lottery_records.prize_type, subscription_lottery_prizes.prize_type, 'subscription'),
			subscription_lottery_records.plan_id,
			subscription_plans.name,
			subscription_plans.duration_days,
			subscription_lottery_records.draw_date,
			subscription_lottery_records.user_ip,
			subscription_lottery_records.created_at
		FROM subscription_lottery_records
		LEFT JOIN users ON users.id = subscription_lottery_records.user_id
		LEFT JOIN subscription_lottery_prizes ON subscription_lottery_prizes.id = subscription_lottery_records.prize_id
		LEFT JOIN subscription_plans ON subscription_plans.id = subscription_lottery_records.plan_id
	`
}

func availableLotteryPrizes(ctx context.Context, tx *database.Tx) ([]LotteryPrize, error) {
	lockClause := " FOR UPDATE"
	if database.CurrentDialect() == database.DialectPostgres {
		lockClause = " FOR UPDATE OF subscription_lottery_prizes"
	}
	monthStart, nextMonth, _, _ := lotteryMonthWindow(time.Now())
	rows, err := tx.QueryContext(ctx, `
		SELECT subscription_lottery_prizes.id,
			subscription_lottery_prizes.name,
			COALESCE(subscription_lottery_prizes.prize_type, 'subscription'),
			subscription_lottery_prizes.plan_id,
			subscription_plans.name,
			subscription_plans.duration_days,
			subscription_plans.quota_images,
			subscription_lottery_prizes.weight,
			subscription_lottery_prizes.daily_stock,
			COALESCE(today.used, 0),
			subscription_lottery_prizes.monthly_stock,
			COALESCE(month_used.used, 0),
			subscription_lottery_prizes.sort_order,
			subscription_lottery_prizes.status,
			subscription_lottery_prizes.created_at,
			subscription_lottery_prizes.updated_at
		FROM subscription_lottery_prizes
		LEFT JOIN subscription_plans ON subscription_plans.id = subscription_lottery_prizes.plan_id
		LEFT JOIN (
			SELECT prize_id, COUNT(*) AS used
			FROM subscription_lottery_records
			WHERE draw_date = CURDATE()
			GROUP BY prize_id
		) today ON today.prize_id = subscription_lottery_prizes.id
		LEFT JOIN (
			SELECT prize_id, COUNT(*) AS used
			FROM subscription_lottery_records
			WHERE draw_date >= ? AND draw_date < ? AND COALESCE(prize_type, 'subscription')='subscription'
			GROUP BY prize_id
		) month_used ON month_used.prize_id = subscription_lottery_prizes.id
		WHERE subscription_lottery_prizes.status='active'
			AND subscription_lottery_prizes.weight > 0
			AND COALESCE(subscription_lottery_prizes.prize_type, 'subscription')='subscription'
			AND subscription_plans.status='active'
			AND subscription_plans.duration_days > 0
			AND (subscription_lottery_prizes.monthly_stock <= 0 OR COALESCE(month_used.used, 0) < subscription_lottery_prizes.monthly_stock)
		ORDER BY subscription_lottery_prizes.sort_order ASC, subscription_lottery_prizes.created_at ASC
	`+lockClause, monthStart, nextMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []LotteryPrize{}
	for rows.Next() {
		item, err := scanLotteryPrize(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func lotteryMonthDrawCount(ctx context.Context, tx *database.Tx, monthStart string, nextMonth string) (int, error) {
	var total int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM subscription_lottery_records
		WHERE draw_date >= ? AND draw_date < ?
	`, monthStart, nextMonth).Scan(&total)
	return total, err
}

func lotteryActiveUserCount(ctx context.Context, tx *database.Tx) (int, error) {
	var total int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE status='active'`).Scan(&total)
	return total, err
}

func chooseLotteryPrize(items []LotteryPrize) (LotteryPrize, error) {
	total := 0
	for _, item := range items {
		if item.Weight > 0 {
			total += item.Weight
		}
	}
	if total <= 0 {
		return LotteryPrize{}, ErrNoLotteryPrize
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(total)))
	if err != nil {
		return LotteryPrize{}, err
	}
	target := int(value.Int64())
	current := 0
	for _, item := range items {
		if item.Weight <= 0 {
			continue
		}
		current += item.Weight
		if target < current {
			return item, nil
		}
	}
	return items[len(items)-1], nil
}

func shouldHitLotteryPrize(items []LotteryPrize, now time.Time, monthDrawCount int, activeUserCount int) (bool, error) {
	threshold := lotteryHitThreshold(items, now, monthDrawCount, activeUserCount)
	if threshold <= 0 {
		return false, nil
	}
	if threshold >= lotteryChanceScale {
		return true, nil
	}
	value, err := rand.Int(rand.Reader, big.NewInt(lotteryChanceScale))
	if err != nil {
		return false, err
	}
	return int(value.Int64()) < threshold, nil
}

func lotteryHitThreshold(items []LotteryPrize, now time.Time, monthDrawCount int, activeUserCount int) int {
	remaining := 0
	hasMonthlyLimit := false
	hasUnlimitedPrize := false
	for _, item := range items {
		if item.MonthlyStock <= 0 {
			hasUnlimitedPrize = true
			continue
		}
		hasMonthlyLimit = true
		left := item.MonthlyStock - item.MonthUsed
		if left > 0 {
			remaining += left
		}
	}
	if !hasMonthlyLimit {
		return lotteryChanceScale
	}
	if hasUnlimitedPrize {
		return lotteryChanceScale
	}
	if remaining <= 0 {
		return 0
	}
	_, _, daysInMonth, daysRemaining := lotteryMonthWindow(now)
	if daysInMonth <= 0 {
		daysInMonth = 30
	}
	if daysRemaining <= 0 {
		daysRemaining = 1
	}
	if monthDrawCount < 0 {
		monthDrawCount = 0
	}
	if activeUserCount < 1 {
		activeUserCount = 1
	}
	elapsedDays := daysInMonth - daysRemaining + 1
	if elapsedDays < 1 {
		elapsedDays = 1
	}
	projectedMonthDraws := 0
	if monthDrawCount > 0 {
		projectedMonthDraws = (monthDrawCount*daysInMonth + elapsedDays - 1) / elapsedDays
	}
	baselineDailyDraws := activeUserCount
	if baselineDailyDraws < lotteryBaselineDailyDraws {
		baselineDailyDraws = lotteryBaselineDailyDraws
	}
	baselineMonthDraws := baselineDailyDraws * daysInMonth
	if projectedMonthDraws < baselineMonthDraws {
		projectedMonthDraws = baselineMonthDraws
	}
	estimatedRemainingDraws := projectedMonthDraws - monthDrawCount
	if estimatedRemainingDraws < daysRemaining {
		estimatedRemainingDraws = daysRemaining
	}
	if estimatedRemainingDraws <= remaining {
		return lotteryChanceScale
	}
	threshold := (remaining*lotteryChanceScale + estimatedRemainingDraws - 1) / estimatedRemainingDraws
	if threshold > lotteryChanceScale {
		threshold = lotteryChanceScale
	}
	return threshold
}

func lotteryMonthWindow(now time.Time) (string, string, int, int) {
	location := appclock.ConfigureDefault()
	current := now.In(location)
	monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, location)
	nextMonth := monthStart.AddDate(0, 1, 0)
	dayStart := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, location)
	daysInMonth := int(nextMonth.Sub(monthStart).Hours() / 24)
	daysRemaining := int(nextMonth.Sub(dayStart).Hours() / 24)
	if daysRemaining < 1 {
		daysRemaining = 1
	}
	return monthStart.Format("2006-01-02"), nextMonth.Format("2006-01-02"), daysInMonth, daysRemaining
}

func virtualThanksLotteryPrize() LotteryPrize {
	return LotteryPrize{
		ID:        lotteryAutoThanksPrizeID,
		Name:      "谢谢惠顾",
		PrizeType: "thanks",
		Weight:    1,
		Status:    "active",
	}
}

func grantSubscriptionInTx(ctx context.Context, tx *database.Tx, userID string, planID string, durationDays int, now time.Time) error {
	userID = strings.TrimSpace(userID)
	planID = strings.TrimSpace(planID)
	if userID == "" || planID == "" {
		return sql.ErrNoRows
	}
	if durationDays <= 0 {
		if err := tx.QueryRowContext(ctx, `SELECT duration_days FROM subscription_plans WHERE id=? AND status='active'`, planID).Scan(&durationDays); err != nil {
			return err
		}
	}
	if durationDays <= 0 {
		return ErrNoLotteryPrize
	}
	baseTime := now
	var currentExpires sql.NullTime
	err := tx.QueryRowContext(ctx, `SELECT expires_at FROM user_subscriptions WHERE user_id=? FOR UPDATE`, userID).Scan(&currentExpires)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if currentExpires.Valid && currentExpires.Time.After(now) {
		baseTime = currentExpires.Time
	}
	expiresAt := baseTime.AddDate(0, 0, durationDays)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO user_subscriptions (id, user_id, plan_id, status, started_at, expires_at)
			VALUES (?, ?, ?, 'active', ?, ?)
		`, newOperationID(), userID, planID, now, expiresAt)
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET plan_id=?, status='active', started_at=?, expires_at=?, updated_at=CURRENT_TIMESTAMP
		WHERE user_id=?
	`, planID, now, expiresAt, userID)
	return err
}

func sqlDateString(value any) string {
	switch typed := value.(type) {
	case time.Time:
		return appclock.DatabaseTime(typed).Format("2006-01-02")
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return ""
	}
}

func (r *Repository) Orders(ctx context.Context, input PageInput) ([]RechargeOrder, int, error) {
	_, pageSize, offset := normalizePage(input.Page, input.PageSize)
	where := []string{}
	args := []any{}
	if input.Status != "" && input.Status != "all" {
		where = append(where, "recharge_orders.status=?")
		args = append(args, input.Status)
	}
	whereSQL := buildWhere(where)
	total, err := r.countWithArgs(ctx, `SELECT COUNT(*) FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id `+whereSQL, args)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT recharge_orders.id, recharge_orders.user_id, users.email, recharge_orders.out_trade_no, recharge_orders.trade_no,
			recharge_orders.order_type, recharge_orders.subscription_plan_id, recharge_orders.amount, recharge_orders.credits,
			recharge_orders.status, recharge_orders.pay_url, recharge_orders.qr_code, recharge_orders.paid_at, recharge_orders.created_at, recharge_orders.updated_at
		FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id
		`+whereSQL+` ORDER BY recharge_orders.created_at DESC LIMIT ? OFFSET ?
	`, append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []RechargeOrder{}
	for rows.Next() {
		item, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *Repository) CreateOrder(ctx context.Context, order RechargeOrder) (*RechargeOrder, error) {
	order.Credits = normalizeOperationCreditAmount(order.Credits)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO recharge_orders (id, user_id, out_trade_no, order_type, subscription_plan_id, amount, credits, status, pay_url, qr_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, order.ID, order.UserID, order.OutTradeNo, defaultString(order.OrderType, "recharge"), order.SubscriptionPlanID, order.Amount, order.Credits, defaultString(order.Status, "pending"), order.PayURL, order.QRCode)
	if err != nil {
		return nil, err
	}
	return r.FindOrder(ctx, order.ID)
}

func (r *Repository) FindOrder(ctx context.Context, id string) (*RechargeOrder, error) {
	rows, total, err := r.Orders(ctx, PageInput{Page: 1, PageSize: 1})
	if err != nil {
		return nil, err
	}
	_ = total
	for _, item := range rows {
		if item.ID == id {
			return &item, nil
		}
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT recharge_orders.id, recharge_orders.user_id, users.email, recharge_orders.out_trade_no, recharge_orders.trade_no,
			recharge_orders.order_type, recharge_orders.subscription_plan_id, recharge_orders.amount, recharge_orders.credits,
			recharge_orders.status, recharge_orders.pay_url, recharge_orders.qr_code, recharge_orders.paid_at, recharge_orders.created_at, recharge_orders.updated_at
		FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id WHERE recharge_orders.id=? LIMIT 1
	`, id)
	item, err := scanOrder(row)
	return &item, err
}

func (r *Repository) FindOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*RechargeOrder, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT recharge_orders.id, recharge_orders.user_id, users.email, recharge_orders.out_trade_no, recharge_orders.trade_no,
			recharge_orders.order_type, recharge_orders.subscription_plan_id, recharge_orders.amount, recharge_orders.credits,
			recharge_orders.status, recharge_orders.pay_url, recharge_orders.qr_code, recharge_orders.paid_at, recharge_orders.created_at, recharge_orders.updated_at
		FROM recharge_orders LEFT JOIN users ON users.id=recharge_orders.user_id WHERE recharge_orders.out_trade_no=? LIMIT 1
	`, strings.TrimSpace(outTradeNo))
	item, err := scanOrder(row)
	return &item, err
}

func (r *Repository) CompleteOrder(ctx context.Context, outTradeNo string, tradeNo string) (*RechargeOrder, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var order RechargeOrder
	var email, trade, plan, payURL, qr sql.NullString
	var paid sql.NullTime
	var created, updated time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id, NULL, out_trade_no, trade_no,
			order_type, subscription_plan_id, amount, credits,
			status, pay_url, qr_code, paid_at, created_at, updated_at
		FROM recharge_orders
		WHERE out_trade_no=? FOR UPDATE
	`, strings.TrimSpace(outTradeNo)).Scan(&order.ID, &order.UserID, &email, &order.OutTradeNo, &trade, &order.OrderType, &plan, &order.Amount, &order.Credits, &order.Status, &payURL, &qr, &paid, &created, &updated)
	if err != nil {
		return nil, false, err
	}
	order.UserEmail = nullString(email)
	order.TradeNo = nullString(trade)
	order.SubscriptionPlanID = nullString(plan)
	order.PayURL = nullString(payURL)
	order.QRCode = nullString(qr)
	order.PaidAt = nullTime(paid)
	order.CreatedAt = appclock.DatabaseTime(created).Format(time.RFC3339)
	order.UpdatedAt = appclock.DatabaseTime(updated).Format(time.RFC3339)
	if order.Status == "paid" {
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return &order, false, nil
	}
	isSubscription := order.OrderType == "subscription" && order.SubscriptionPlanID != nil && strings.TrimSpace(*order.SubscriptionPlanID) != ""
	order.Credits = normalizeOperationCreditAmount(order.Credits)
	if !isSubscription && order.Credits <= 0 {
		return nil, false, ErrInvalidRechargeOrder
	}
	var balance float64
	if err := tx.QueryRowContext(ctx, `SELECT credits FROM users WHERE id=? FOR UPDATE`, order.UserID).Scan(&balance); err != nil {
		return nil, false, err
	}
	now := time.Now()
	creditRemark := "支付宝充值 " + order.OutTradeNo
	if isSubscription {
		var planName string
		var durationDays int
		if err := tx.QueryRowContext(ctx, `
			SELECT name, duration_days
			FROM subscription_plans
			WHERE id=?
		`, strings.TrimSpace(*order.SubscriptionPlanID)).Scan(&planName, &durationDays); err != nil {
			return nil, false, err
		}
		if err := grantSubscriptionInTx(ctx, tx, order.UserID, *order.SubscriptionPlanID, durationDays, now); err != nil {
			return nil, false, err
		}
		creditRemark = "订阅套餐赠送 " + planName
	}
	if order.Credits > 0 {
		nextBalance := normalizeOperationCreditAmount(balance + order.Credits)
		if _, err := tx.ExecContext(ctx, `UPDATE users SET credits=? WHERE id=?`, nextBalance, order.UserID); err != nil {
			return nil, false, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO credit_logs (id, user_id, type, amount, balance_after, remark)
			VALUES (?, ?, 'recharge', ?, ?, ?)
		`, newOperationID(), order.UserID, order.Credits, nextBalance, creditRemark); err != nil {
			return nil, false, err
		}
	}
	tradeNo = strings.TrimSpace(tradeNo)
	if _, err := tx.ExecContext(ctx, `UPDATE recharge_orders SET status='paid', trade_no=?, paid_at=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, tradeNo, now, order.ID); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	order.Status = "paid"
	order.TradeNo = &tradeNo
	paidAt := appclock.DatabaseTime(now).Format(time.RFC3339)
	order.PaidAt = &paidAt
	order.UpdatedAt = paidAt
	return &order, true, nil
}

func normalizeOperationCreditAmount(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*10000) / 10000
}

func (r *Repository) count(ctx context.Context, query string) (int, error) {
	return r.countWithArgs(ctx, query, nil)
}

func (r *Repository) countWithArgs(ctx context.Context, query string, args []any) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&total)
	return total, err
}

func normalizePage(page int, pageSize int) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, (page - 1) * pageSize
}

func buildWhere(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

func scanPlan(row interface{ Scan(dest ...any) error }) (SubscriptionPlan, error) {
	item, err := scanPlanWithPrefix(row)
	if item == nil {
		return SubscriptionPlan{}, err
	}
	return *item, err
}

func scanPlanWithPrefix(row interface{ Scan(dest ...any) error }, prefix ...any) (*SubscriptionPlan, error) {
	var item SubscriptionPlan
	var description, providers, models, badge sql.NullString
	var created, updated time.Time
	dest := append(prefix,
		&item.ID, &item.Name, &description, &item.Amount, &item.DurationDays, &item.QuotaImages,
		&item.BonusCredits, &item.DiscountPercent, &providers, &models, &badge,
		&item.SortOrder, &item.Status, &created, &updated,
	)
	err := row.Scan(dest...)
	item.Description = nullString(description)
	item.Badge = nullString(badge)
	item.AllowedProviderIDs = jsonStringList(providers.String)
	item.AllowedModelIDs = jsonStringList(models.String)
	item.CreatedAt = appclock.DatabaseTime(created).Format(time.RFC3339)
	item.UpdatedAt = appclock.DatabaseTime(updated).Format(time.RFC3339)
	return &item, err
}

func scanInvite(row interface{ Scan(dest ...any) error }) (Invite, error) {
	var item Invite
	var inviter, invitee, rewardType, planID, label, ip sql.NullString
	var created time.Time
	err := row.Scan(&item.ID, &item.InviterID, &inviter, &item.InviteeID, &invitee, &item.RewardCredits, &rewardType, &planID, &label, &ip, &created)
	item.InviterEmail = nullString(inviter)
	item.InviteeEmail = nullString(invitee)
	item.RewardType = "subscription"
	if rewardType.Valid && strings.TrimSpace(rewardType.String) != "" {
		item.RewardType = rewardType.String
	}
	item.RewardPlanID = nullString(planID)
	item.RewardLabel = nullString(label)
	item.InviteeIP = nullString(ip)
	item.CreatedAt = appclock.DatabaseTime(created).Format(time.RFC3339)
	return item, err
}

func scanOrder(row interface{ Scan(dest ...any) error }) (RechargeOrder, error) {
	var item RechargeOrder
	var email, trade, plan, payURL, qr sql.NullString
	var paid sql.NullTime
	var created, updated time.Time
	err := row.Scan(&item.ID, &item.UserID, &email, &item.OutTradeNo, &trade, &item.OrderType, &plan, &item.Amount, &item.Credits, &item.Status, &payURL, &qr, &paid, &created, &updated)
	item.UserEmail = nullString(email)
	item.TradeNo = nullString(trade)
	item.SubscriptionPlanID = nullString(plan)
	item.PayURL = nullString(payURL)
	item.QRCode = nullString(qr)
	item.PaidAt = nullTime(paid)
	item.CreatedAt = appclock.DatabaseTime(created).Format(time.RFC3339)
	item.UpdatedAt = appclock.DatabaseTime(updated).Format(time.RFC3339)
	return item, err
}

func nullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	text := value.String
	return &text
}

func nullTime(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	text := appclock.DatabaseTime(value.Time).Format(time.RFC3339)
	return &text
}

func jsonStringList(value string) []string {
	var items []string
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return []string{}
	}
	return items
}

func defaultStatus(value string) string {
	if value == "disabled" || value == "used" {
		return value
	}
	return "active"
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func affected(result sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}
