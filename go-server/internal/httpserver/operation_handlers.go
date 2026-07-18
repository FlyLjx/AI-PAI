package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/operations"
	"aipi-go/internal/settings"
	"aipi-go/internal/users"

	qrcode "github.com/skip2/go-qrcode"
)

func (r *Router) dashboard(w http.ResponseWriter, req *http.Request) {
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
	data, err := operations.NewRepository(r.db).DashboardSummary(ctx, queryInt(req, "limit", 8))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) plans(w http.ResponseWriter, req *http.Request) {
	r.planCollection(w, req, true)
}

func (r *Router) adminPlans(w http.ResponseWriter, req *http.Request) {
	r.planCollection(w, req, false)
}

func (r *Router) planCollection(w http.ResponseWriter, req *http.Request, public bool) {
	if !public {
		if _, err := r.requireAdmin(req); err != nil {
			writeError(w, err)
			return
		}
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := operations.NewRepository(r.db)
	switch req.Method {
	case http.MethodGet:
		items, err := repo.Plans(ctx, public)
		if err != nil {
			writeError(w, err)
			return
		}
		if public {
			w.Header().Set("Cache-Control", "public, max-age=30")
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items})
	case http.MethodPost:
		if public {
			writeMethodNotAllowed(w)
			return
		}
		var input operations.SubscriptionPlan
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		input.DiscountPercent = 0
		input.ID = newID()
		item, err := repo.SavePlan(ctx, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": item})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) planByID(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	id := strings.TrimPrefix(req.URL.Path, "/api/subscriptions/plans/")
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := operations.NewRepository(r.db)
	switch req.Method {
	case http.MethodPatch:
		var input operations.SubscriptionPlan
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		input.DiscountPercent = 0
		input.ID = id
		item, err := repo.SavePlan(ctx, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": item})
	case http.MethodDelete:
		deleted, err := repo.DeletePlan(ctx, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": deleted}})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) currentSubscription(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID, err := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	data, err := r.currentSubscriptionEntitlement(ctx, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) invites(w http.ResponseWriter, req *http.Request) {
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
	items, total, err := operations.NewRepository(r.db).Invites(ctx, operationPage(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated(items, total, req))
}

func (r *Router) inviteSummary(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID, authErr := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if authErr != nil {
		writeError(w, authErr)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := operations.NewRepository(r.db).InviteSummary(ctx, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	if userID != "" {
		userRepo := users.NewRepository(r.db)
		if inviteCode, err := userRepo.EnsureInviteCode(ctx, userID); err == nil {
			data["inviteCode"] = inviteCode
		}
	}
	config := inviteProgramConfigFromSettings(values)
	operationRepo := operations.NewRepository(r.db)
	inviterPlanName := ""
	if config.InviterReward.Type == "subscription" && config.InviterReward.PlanID != "" {
		if plan, err := operationRepo.FindPlan(ctx, config.InviterReward.PlanID); err == nil && plan != nil {
			inviterPlanName = plan.Name
		}
	}
	inviteePlanName := ""
	if config.InviteeReward.Type == "subscription" && config.InviteeReward.PlanID != "" {
		if plan, err := operationRepo.FindPlan(ctx, config.InviteeReward.PlanID); err == nil && plan != nil {
			inviteePlanName = plan.Name
		}
	}
	data["enabled"] = config.Enabled && inviteRewardSpecConfigured(config.InviterReward) && inviteRewardSpecConfigured(config.InviteeReward)
	data["rewardType"] = config.InviterReward.Type
	data["rewardPlanId"] = config.InviterReward.PlanID
	data["rewardPlanName"] = inviterPlanName
	data["rewardText"] = inviteRewardDescription(config.InviterReward, inviterPlanName)
	data["inviteeRewardText"] = inviteRewardDescription(config.InviteeReward, inviteePlanName)
	data["inviterReward"] = map[string]any{
		"type": config.InviterReward.Type, "credits": config.InviterReward.Credits,
		"planId": config.InviterReward.PlanID, "planName": inviterPlanName,
	}
	data["inviteeReward"] = map[string]any{
		"type": config.InviteeReward.Type, "credits": config.InviteeReward.Credits,
		"planId": config.InviteeReward.PlanID, "planName": inviteePlanName,
	}
	rebateConfig := inviteRechargeRebateConfigFromSettings(values)
	data["rechargeRebateEnabled"] = rebateConfig.Enabled
	data["rechargeRebatePercent"] = rebateConfig.Percent
	data["rebateIncludeSubscriptions"] = rebateConfig.IncludeSubscriptions
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) inviteByID(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	id := strings.TrimPrefix(req.URL.Path, "/api/invites/")
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	result, err := operations.NewRepository(r.db).DeleteInvite(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if result != nil && result.Deleted && result.InviterID != "" {
		r.publishCurrentUser(context.Background(), result.InviterID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (r *Router) subscriptionLottery(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID := strings.TrimSpace(req.URL.Query().Get("userId"))
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := operations.NewRepository(r.db)
	prizes, err := repo.LotteryPrizes(ctx, true)
	if err != nil {
		writeError(w, err)
		return
	}
	var todayRecord *operations.LotteryRecord
	if userID != "" {
		record, err := repo.TodayLotteryRecord(ctx, userID)
		if err == nil {
			todayRecord = record
		} else if !errors.Is(err, sql.ErrNoRows) {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"prizes":      prizes,
		"todayRecord": todayRecord,
	}})
}

func (r *Router) subscriptionLotteryDraw(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID string `json:"userId"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	input.UserID = strings.TrimSpace(input.UserID)
	if input.UserID == "" {
		writeError(w, newAppError(http.StatusBadRequest, "缺少用户信息"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 12*time.Second)
	defer cancel()
	result, err := operations.NewRepository(r.db).DrawSubscriptionLottery(ctx, input.UserID, requestIP(req))
	if errors.Is(err, operations.ErrNoLotteryPrize) {
		writeError(w, newAppError(http.StatusBadRequest, "暂无可用抽奖奖品"))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在或已被禁用"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	user, err := users.NewRepository(r.db).FindByID(ctx, input.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	publicUser := r.publicUserWithSubscription(ctx, user)
	if !result.DrawnToday {
		r.publishCurrentUser(context.Background(), input.UserID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"result": result,
		"user":   publicUser,
	}})
}

func (r *Router) lotteryPrizes(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := operations.NewRepository(r.db)
	switch req.Method {
	case http.MethodGet:
		items, err := repo.LotteryPrizes(ctx, false)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": items})
	case http.MethodPost:
		var input operations.LotteryPrize
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		if err := r.validateLotteryPrizeInput(ctx, input); err != nil {
			writeError(w, err)
			return
		}
		item, err := repo.CreateLotteryPrize(ctx, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": item})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) lotteryPrizeByID(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	id := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/lottery/prizes/"), "/")
	if id == "" {
		writeError(w, newAppError(http.StatusNotFound, "抽奖奖品不存在"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := operations.NewRepository(r.db)
	switch req.Method {
	case http.MethodPatch:
		var input operations.LotteryPrize
		if err := decodeCompatJSON(req, &input); err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
			return
		}
		if err := r.validateLotteryPrizeInput(ctx, input); err != nil {
			writeError(w, err)
			return
		}
		item, err := repo.UpdateLotteryPrize(ctx, id, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": item})
	case http.MethodDelete:
		deleted, err := repo.DeleteLotteryPrize(ctx, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": deleted}})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) lotteryRecords(w http.ResponseWriter, req *http.Request) {
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
	items, total, err := operations.NewRepository(r.db).LotteryRecords(ctx, operationPage(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated(items, total, req))
}

func (r *Router) validateLotteryPrizeInput(ctx context.Context, input operations.LotteryPrize) error {
	switch strings.ToLower(strings.TrimSpace(input.PrizeType)) {
	case "thanks", "none", "no_prize":
		return newAppError(http.StatusBadRequest, "谢谢惠顾由系统自动生成，无需配置")
	}
	if strings.TrimSpace(input.Name) == "" {
		return newAppError(http.StatusBadRequest, "请填写奖品名称")
	}
	if strings.TrimSpace(input.PlanID) == "" {
		return newAppError(http.StatusBadRequest, "请选择订阅套餐")
	}
	plan, err := operations.NewRepository(r.db).FindPlan(ctx, input.PlanID)
	if errors.Is(err, sql.ErrNoRows) || plan == nil || plan.Status != "active" {
		return newAppError(http.StatusBadRequest, "订阅套餐不存在或未启用")
	}
	if err != nil {
		return err
	}
	if plan.DurationDays <= 0 {
		return newAppError(http.StatusBadRequest, "订阅套餐有效期不正确")
	}
	return nil
}

func (r *Router) recharge(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID             string  `json:"userId"`
		Amount             float64 `json:"amount"`
		SubscriptionPlanID *string `json:"subscriptionPlanId"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	userID, err := r.requireFrontUser(req, input.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	input.UserID = userID
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	if !anyBool(values["rechargeEnabled"]) {
		writeError(w, newAppError(http.StatusForbidden, "充值暂未开放"))
		return
	}
	user, err := users.NewRepository(r.db).FindByID(ctx, input.UserID)
	if errors.Is(err, sql.ErrNoRows) || user == nil || user.Status != "active" {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在或已被禁用"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	repo := operations.NewRepository(r.db)
	amount := input.Amount
	credits := float64(0)
	subject := anyString(values["siteName"]) + "余额充值"
	orderType := "recharge"
	var subscriptionPlanID *string
	if input.SubscriptionPlanID != nil && strings.TrimSpace(*input.SubscriptionPlanID) != "" {
		planID := strings.TrimSpace(*input.SubscriptionPlanID)
		plan, findErr := repo.FindPlan(ctx, planID)
		if errors.Is(findErr, sql.ErrNoRows) || plan == nil || plan.Status != "active" || operations.IsAdminCustomSubscriptionPlan(plan) {
			writeError(w, newAppError(http.StatusNotFound, "订阅套餐不存在或已下架"))
			return
		}
		if findErr != nil {
			writeError(w, findErr)
			return
		}
		amount = plan.Amount
		credits = plan.BonusCredits
		subject = anyString(values["siteName"]) + plan.Name
		orderType = "subscription"
		subscriptionPlanID = &planID
	} else {
		minimum := anyFloat(values["rechargeMinAmount"], 1)
		if amount < minimum {
			writeError(w, newAppError(http.StatusBadRequest, fmt.Sprintf("余额充值最低 %.2f 元", minimum)))
			return
		}
		credits = normalizedCreditAmount(amount * anyFloat(values["rechargeRate"], 10))
		if credits <= 0 {
			writeError(w, newAppError(http.StatusBadRequest, "充值兑换比例不正确"))
			return
		}
	}
	orderID := newID()
	outTradeNo := "AIPI" + time.Now().Format("20060102150405") + strings.ToUpper(strings.ReplaceAll(orderID[:8], "-", ""))
	payment, err := createAlipayPrecreateOrder(ctx, alipaySettingsFromMap(values), outTradeNo, amount, subject, requestOrigin(req))
	if err != nil {
		writeError(w, err)
		return
	}
	payURL := payment.QRCode
	order, err := repo.CreateOrder(ctx, operations.RechargeOrder{
		ID: orderID, UserID: input.UserID, OutTradeNo: outTradeNo,
		OrderType: orderType, SubscriptionPlanID: subscriptionPlanID,
		Amount: amount, Credits: credits, Status: "pending",
		PayURL: &payURL, QRCode: &payURL,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": order})
}

func (r *Router) rechargeQRCode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	text := strings.TrimSpace(req.URL.Query().Get("text"))
	if text == "" {
		writeError(w, newAppError(http.StatusBadRequest, "缺少二维码内容"))
		return
	}
	png, err := qrcode.Encode(text, qrcode.Medium, 260)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (r *Router) alipayNotify(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if err := req.ParseForm(); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "支付宝通知参数不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 12*time.Second)
	defer cancel()
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := verifyAlipayNotify(alipaySettingsFromMap(values), req.PostForm); err != nil {
		writeError(w, err)
		return
	}
	outTradeNo := strings.TrimSpace(req.FormValue("out_trade_no"))
	tradeNo := strings.TrimSpace(req.FormValue("trade_no"))
	tradeStatus := strings.TrimSpace(req.FormValue("trade_status"))
	if outTradeNo == "" {
		var input map[string]any
		if err := decodeCompatJSON(req, &input); err == nil {
			outTradeNo = strings.TrimSpace(fmt.Sprint(input["out_trade_no"]))
			tradeNo = strings.TrimSpace(fmt.Sprint(input["trade_no"]))
			tradeStatus = strings.TrimSpace(fmt.Sprint(input["trade_status"]))
		}
	}
	if outTradeNo == "" {
		writeError(w, newAppError(http.StatusBadRequest, "缺少商户订单号"))
		return
	}
	if tradeStatus != "" && tradeStatus != "TRADE_SUCCESS" && tradeStatus != "TRADE_FINISHED" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("success"))
		return
	}
	order, changed, err := operations.NewRepository(r.db).CompleteOrder(ctx, outTradeNo, tradeNo, inviteRechargeRebateConfigFromSettings(values))
	if errors.Is(err, operations.ErrRechargeOrderClosed) {
		if r.logger != nil {
			r.logger.Warn("ignored payment callback for closed order", "outTradeNo", outTradeNo, "tradeNo", tradeNo)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("success"))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "订阅订单不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if changed {
		r.publishCurrentUser(context.Background(), order.UserID)
		if order.InviteRebate != nil {
			r.publishCurrentUser(context.Background(), order.InviteRebate.InviterID)
		}
		r.notifyRechargeSuccess(order)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}

func (r *Router) rechargeOrders(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, total, err := operations.NewRepository(r.db).Orders(ctx, operationPage(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated(items, total, req))
}

func (r *Router) rechargeHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID, err := r.requireFrontUser(req, req.URL.Query().Get("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	input := operationPage(req)
	input.UserID = userID
	items, total, err := operations.NewRepository(r.db).Orders(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, paginated(items, total, req))
}

func (r *Router) rechargeByID(w http.ResponseWriter, req *http.Request) {
	id := strings.TrimPrefix(req.URL.Path, "/api/recharge/")
	if strings.HasSuffix(id, "/sync") {
		r.syncRechargeOrder(w, req, strings.TrimSuffix(id, "/sync"))
		return
	}
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID, err := r.requireFrontUser(req, "")
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	order, err := operations.NewRepository(r.db).FindOrder(ctx, strings.Trim(id, "/"))
	if err != nil {
		writeError(w, err)
		return
	}
	if order.UserID != userID {
		writeError(w, newAppError(http.StatusForbidden, "只能查看自己的充值订单"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": order})
}

func (r *Router) syncRechargeOrder(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID string `json:"userId"`
	}
	_ = decodeCompatJSON(req, &input)
	userID, err := r.requireFrontUser(req, "")
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	repo := operations.NewRepository(r.db)
	order, err := repo.FindOrder(ctx, strings.Trim(id, "/"))
	if errors.Is(err, sql.ErrNoRows) || order == nil {
		writeError(w, newAppError(http.StatusNotFound, "订阅订单不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if order.UserID != userID {
		writeError(w, newAppError(http.StatusForbidden, "只能同步自己的充值订单"))
		return
	}
	if strings.TrimSpace(input.UserID) != "" && strings.TrimSpace(input.UserID) != order.UserID {
		writeError(w, newAppError(http.StatusNotFound, "订阅订单不存在"))
		return
	}
	if order.Status != "pending" {
		writeJSON(w, http.StatusOK, map[string]any{"data": order})
		return
	}
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	queried, err := queryAlipayOrder(ctx, alipaySettingsFromMap(values), order.OutTradeNo)
	if err != nil {
		writeError(w, err)
		return
	}
	if !queried.Paid {
		writeJSON(w, http.StatusOK, map[string]any{"data": order})
		return
	}
	paidOrder, changed, err := repo.CompleteOrder(ctx, order.OutTradeNo, queried.TradeNo, inviteRechargeRebateConfigFromSettings(values))
	if errors.Is(err, operations.ErrRechargeOrderClosed) {
		closedOrder, findErr := repo.FindOrder(ctx, order.ID)
		if findErr != nil {
			writeError(w, findErr)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": closedOrder})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if changed {
		r.publishCurrentUser(context.Background(), paidOrder.UserID)
		if paidOrder.InviteRebate != nil {
			r.publishCurrentUser(context.Background(), paidOrder.InviteRebate.InviterID)
		}
		r.notifyRechargeSuccess(paidOrder)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": paidOrder})
}

func operationPage(req *http.Request) operations.PageInput {
	return operations.PageInput{
		Page:     queryInt(req, "page", 1),
		PageSize: queryInt(req, "pageSize", queryInt(req, "limit", 20)),
		Keyword:  strings.TrimSpace(req.URL.Query().Get("keyword")),
		Status:   strings.TrimSpace(req.URL.Query().Get("status")),
	}
}

func paginated(items any, total int, req *http.Request) map[string]any {
	page := queryInt(req, "page", 1)
	pageSize := queryInt(req, "pageSize", queryInt(req, "limit", 20))
	return map[string]any{
		"data": items,
		"pagination": map[string]any{
			"total": total, "page": page, "pageSize": pageSize,
		},
	}
}
