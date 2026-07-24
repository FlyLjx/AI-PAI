package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/auth"
	"aipi-go/internal/operations"
	"aipi-go/internal/settings"
	"aipi-go/internal/tasks"
	"aipi-go/internal/users"
)

func (r *Router) userLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	if input.Email == "" || input.Password == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请输入邮箱和密码"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByEmail(ctx, input.Email)
	if errors.Is(err, sql.ErrNoRows) || user == nil || !auth.VerifyPassword(input.Password, user.PasswordHash) {
		writeError(w, newAppError(http.StatusUnauthorized, "邮箱或密码错误"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if user.Status != "active" {
		writeError(w, newAppError(http.StatusForbidden, "用户已被禁用"))
		return
	}
	user = r.settleInviteRewards(ctx, user)
	token, err := r.tokens.CreateUserToken(user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": mergeUserToken(r.publicUserWithSubscription(ctx, user), token),
	})
}

func (r *Router) publicUserWithSubscription(ctx context.Context, user *users.User) users.PublicUser {
	if user == nil || user.ID == "" {
		return users.PublicUser{}
	}
	publicUser := users.ToPublicUser(user)
	subscription, err := r.currentSubscriptionEntitlement(ctx, user.ID)
	if err == nil {
		publicUser.Subscription = subscription
	}
	return publicUser
}

func (r *Router) publishCurrentUser(ctx context.Context, userID string) {
	if r.userHub == nil || strings.TrimSpace(userID) == "" {
		return
	}
	userCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(userCtx, strings.TrimSpace(userID))
	if err != nil || user == nil {
		return
	}
	r.userHub.PublishUserData(user.ID, r.publicUserWithSubscription(userCtx, user))
}

func (r *Router) userProfile(w http.ResponseWriter, req *http.Request) {
	path := strings.Trim(strings.TrimPrefix(req.URL.Path, "/api/users/"), "/")
	if strings.HasSuffix(path, "/public-details") {
		r.userDetails(w, req, strings.TrimSuffix(path, "/public-details"), true)
		return
	}
	if strings.HasSuffix(path, "/details") {
		r.userDetails(w, req, strings.TrimSuffix(path, "/details"), false)
		return
	}
	if strings.HasSuffix(path, "/password") {
		r.changeUserPassword(w, req, strings.TrimSuffix(path, "/password"))
		return
	}
	if strings.HasSuffix(path, "/verify-email") {
		r.verifyUserEmailByAdmin(w, req, strings.TrimSuffix(path, "/verify-email"))
		return
	}
	if strings.HasSuffix(path, "/resend-verification") {
		r.resendEmailVerification(w, req, strings.TrimSuffix(path, "/resend-verification"))
		return
	}
	if strings.HasSuffix(path, "/email") {
		r.changeUserEmail(w, req, strings.TrimSuffix(path, "/email"))
		return
	}
	if strings.HasSuffix(path, "/status") {
		r.updateUserStatus(w, req, strings.TrimSuffix(path, "/status"))
		return
	}
	if strings.HasSuffix(path, "/credit-logs") {
		r.userCreditLogs(w, req, strings.TrimSuffix(path, "/credit-logs"))
		return
	}
	if strings.HasSuffix(path, "/balance") {
		r.updateUserBalance(w, req, strings.TrimSuffix(path, "/balance"))
		return
	}
	if strings.HasSuffix(path, "/subscription") {
		r.grantUserSubscription(w, req, strings.TrimSuffix(path, "/subscription"))
		return
	}
	if !strings.Contains(strings.Trim(path, "/"), "/") {
		switch req.Method {
		case http.MethodPatch:
			r.updateUser(w, req, strings.Trim(path, "/"))
			return
		case http.MethodDelete:
			r.deleteUser(w, req, strings.Trim(path, "/"))
			return
		}
	}
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimSuffix(path, "/profile")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if _, err := r.requireFrontUser(req, id); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || user == nil {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if user.Status != "active" {
		writeError(w, newAppError(http.StatusForbidden, "用户已被禁用"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": r.publicUserWithSubscription(ctx, user)})
}

func (r *Router) userDetails(w http.ResponseWriter, req *http.Request, id string, public bool) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	id = strings.Trim(id, "/")
	if !public {
		if _, err := r.requireAdmin(req); err != nil {
			writeError(w, err)
			return
		}
	} else if _, err := r.requireFrontUser(req, id); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	taskPage := queryInt(req, "taskPage", 1)
	taskPageSize := queryInt(req, "taskPageSize", 10)
	taskItems, taskTotal, err := tasks.NewRepository(r.db).FindByUserID(ctx, id, taskPage, taskPageSize)
	if err != nil {
		writeError(w, err)
		return
	}
	publicTasks := make([]tasks.PublicTask, 0, len(taskItems))
	for index := range taskItems {
		publicTasks = append(publicTasks, tasks.ToPublic(&taskItems[index]))
	}
	publicUser := r.publicUserWithSubscription(ctx, user)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"user":            publicUser,
		"tasks":           publicTasks,
		"tasksPagination": map[string]any{"total": taskTotal, "page": taskPage, "pageSize": taskPageSize},
	}})
}

func (r *Router) changeUserPassword(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPatch {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID      string `json:"userId"`
		OldPassword string `json:"oldPassword"`
		Password    string `json:"password"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	id = strings.Trim(id, "/")
	if _, err := r.requireFrontUser(req, id); err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(input.UserID) != "" && strings.TrimSpace(input.UserID) != id {
		writeError(w, newAppError(http.StatusForbidden, "只能修改自己的密码"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := users.NewRepository(r.db)
	user, err := repo.FindByID(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if !auth.VerifyPassword(input.OldPassword, user.PasswordHash) {
		writeError(w, newAppError(http.StatusBadRequest, "当前密码不正确"))
		return
	}
	updated, err := repo.UpdatePassword(ctx, id, auth.HashPassword(input.Password))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": r.publicUserWithSubscription(ctx, updated)})
}

func (r *Router) updateUserStatus(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPatch {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	if input.Status != "active" && input.Status != "disabled" {
		writeError(w, newAppError(http.StatusBadRequest, "状态不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(ctx, strings.Trim(id, "/"))
	if err != nil {
		writeError(w, err)
		return
	}
	user.Status = input.Status
	updated, err := users.NewRepository(r.db).Update(ctx, user.ID, *user)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": r.publicUserWithSubscription(ctx, updated)})
}

func (r *Router) verifyUserEmailByAdmin(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	updated, err := users.NewRepository(r.db).MarkEmailVerified(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if updated == nil {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	updated = r.settleInviteRewards(ctx, updated)
	data := r.publicUserWithSubscription(ctx, updated)
	r.publishCurrentUser(context.Background(), id)
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) updateUserBalance(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPatch {
		writeMethodNotAllowed(w)
		return
	}
	admin, err := r.requireAdmin(req)
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Balance *float64 `json:"balance"`
		Remark  string   `json:"remark"`
	}
	if err := decodeCompatJSON(req, &input); err != nil || input.Balance == nil {
		writeError(w, newAppError(http.StatusBadRequest, "请填写调整后的余额"))
		return
	}
	id = strings.Trim(id, "/")
	input.Remark = strings.TrimSpace(input.Remark)
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if input.Remark == "" || len([]rune(input.Remark)) > 120 {
		writeError(w, newAppError(http.StatusBadRequest, "请填写 1-120 字的调整备注"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	remark := "管理员 " + admin.UserID + "：" + input.Remark
	updated, err := users.NewRepository(r.db).SetCredits(ctx, id, *input.Balance, remark)
	if errors.Is(err, users.ErrInvalidCredits) {
		writeError(w, newAppError(http.StatusBadRequest, err.Error()))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	r.publishCurrentUser(context.Background(), id)
	writeJSON(w, http.StatusOK, map[string]any{"data": users.ToPublicUser(updated)})
}

func (r *Router) userCreditLogs(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}

	logType := strings.TrimSpace(req.URL.Query().Get("type"))
	switch logType {
	case "", "all":
		logType = ""
	case "deduct", "recharge", "manual_adjust", "invite_reward", "invite_rebate":
	default:
		writeError(w, newAppError(http.StatusBadRequest, "积分明细类型不正确"))
		return
	}
	page := queryInt(req, "page", 1)
	pageSize := queryInt(req, "pageSize", 20)
	if pageSize > 100 {
		pageSize = 100
	}

	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := users.NewRepository(r.db)
	if _, err := repo.FindByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	} else if err != nil {
		writeError(w, err)
		return
	}
	items, total, err := repo.ListCreditLogs(ctx, id, logType, page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":       items,
		"pagination": map[string]any{"total": total, "page": page, "pageSize": pageSize},
	})
}

func (r *Router) updateUser(w http.ResponseWriter, req *http.Request, id string) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		Status   string `json:"status"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := users.NewRepository(r.db)
	user, err := repo.FindByID(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(input.Email) != "" {
		user.Email = strings.TrimSpace(input.Email)
	}
	if input.Role == "admin" || input.Role == "user" {
		user.Role = input.Role
	}
	if input.Status == "active" || input.Status == "disabled" {
		user.Status = input.Status
	}
	if strings.TrimSpace(input.Password) != "" {
		if _, err := repo.UpdatePassword(ctx, id, auth.HashPassword(input.Password)); err != nil {
			writeError(w, err)
			return
		}
	}
	updated, err := repo.Update(ctx, id, *user)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": r.publicUserWithSubscription(ctx, updated)})
}

func (r *Router) deleteUser(w http.ResponseWriter, req *http.Request, id string) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	deleted, err := users.NewRepository(r.db).Delete(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": deleted}})
}

func (r *Router) listUsers(w http.ResponseWriter, req *http.Request) {
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	if req.Method == http.MethodPost {
		r.createUser(w, req, true)
		return
	}
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, err := users.NewRepository(r.db).FindAll(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	data := make([]users.PublicUser, 0, len(items))
	for index := range items {
		item := items[index]
		data = append(data, r.publicUserWithSubscription(ctx, &item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) grantUserSubscription(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		GrantType    string `json:"grantType"`
		PlanID       string `json:"planId"`
		Name         string `json:"name"`
		DurationDays int    `json:"durationDays"`
		QuotaImages  int    `json:"quotaImages"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	id = strings.Trim(id, "/")
	input.GrantType = strings.ToLower(strings.TrimSpace(input.GrantType))
	input.PlanID = strings.TrimSpace(input.PlanID)
	if input.GrantType == "" {
		input.GrantType = "plan"
	}
	if id == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请选择用户"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := operations.NewRepository(r.db)
	var grantErr error
	switch input.GrantType {
	case "custom":
		grantErr = repo.GrantCustomSubscription(ctx, id, operations.CustomSubscriptionGrant{
			Name: input.Name, DurationDays: input.DurationDays, QuotaImages: input.QuotaImages,
		})
	case "plan":
		if input.PlanID == "" {
			writeError(w, newAppError(http.StatusBadRequest, "请选择订阅套餐"))
			return
		}
		grantErr = repo.GrantSubscription(ctx, id, input.PlanID)
	default:
		writeError(w, newAppError(http.StatusBadRequest, "发放方式不正确"))
		return
	}
	if grantErr != nil {
		if errors.Is(grantErr, operations.ErrInvalidCustomSubscription) {
			writeError(w, newAppError(http.StatusBadRequest, "自定义订阅需要 1-3650 天有效期和大于 0 的图片额度"))
			return
		}
		if errors.Is(grantErr, sql.ErrNoRows) {
			writeError(w, newAppError(http.StatusNotFound, "用户或订阅套餐不存在"))
			return
		}
		writeError(w, grantErr)
		return
	}
	user, err := users.NewRepository(r.db).FindByID(ctx, id)
	if err != nil {
		writeError(w, err)
		return
	}
	data := r.publicUserWithSubscription(ctx, user)
	r.publishCurrentUser(context.Background(), id)
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) userActivityRanking(w http.ResponseWriter, req *http.Request) {
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
	items, err := users.NewRepository(r.db).ActivityRanking(ctx, queryInt(req, "days", 7), queryInt(req, "limit", 10))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) adminConsumptionRanking(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.requireAdmin(req); err != nil {
		writeError(w, err)
		return
	}
	days := 30
	if rawDays := strings.TrimSpace(req.URL.Query().Get("days")); rawDays != "" {
		if strings.EqualFold(rawDays, "all") {
			days = 0
		} else if parsedDays, err := strconv.Atoi(rawDays); err == nil {
			if parsedDays < 0 {
				days = 0
			} else {
				days = parsedDays
			}
		}
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, err := users.NewRepository(r.db).ConsumptionRanking(ctx, days, queryInt(req, "limit", 10))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) registerUser(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	r.createUser(w, req, false)
}

func (r *Router) createUser(w http.ResponseWriter, req *http.Request, admin bool) {
	var input struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		Role           string `json:"role"`
		InviteCode     string `json:"inviteCode"`
		DeviceID       string `json:"deviceId"`
		ChallengeToken string `json:"challengeToken"`
		Website        string `json:"website"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" || len(input.Password) < 6 {
		writeError(w, newAppError(http.StatusBadRequest, "请输入邮箱和至少 6 位密码"))
		return
	}
	role := "user"
	if admin && input.Role == "admin" {
		role = "admin"
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	repo := users.NewRepository(r.db)
	settingValues := settings.Settings{}
	fingerprint := registrationFingerprintForRequest(req, input.DeviceID)
	inviteConfig := inviteProgramConfig{}
	if !admin {
		if values, err := settings.NewRepository(r.db).Get(ctx); err == nil {
			settingValues = values
		} else {
			writeError(w, err)
			return
		}
		if strings.EqualFold(strings.TrimSpace(anyString(settingValues["registerMode"])), "closed") {
			writeError(w, newAppError(http.StatusForbidden, "当前暂未开放注册"))
			return
		}
		if strings.TrimSpace(input.Website) != "" {
			writeError(w, newAppError(http.StatusTooManyRequests, "注册请求未通过安全校验"))
			return
		}
		var err error
		fingerprint, err = r.validateRegistrationRisk(ctx, req, input.ChallengeToken, input.DeviceID, registrationRiskConfigFromSettings(settingValues))
		if err != nil {
			writeError(w, err)
			return
		}
		inviteConfig = inviteProgramConfigFromSettings(settingValues)
	}
	if existing, err := repo.FindByEmail(ctx, email); err == nil && existing != nil {
		writeError(w, newAppError(http.StatusConflict, "邮箱已存在"))
		return
	}
	emailVerificationRequired := !admin && anyBool(settingValues["registerEmailVerification"])
	var inviter *users.User
	inviteCodeInput := strings.TrimSpace(input.InviteCode)
	if !admin && inviteCodeInput != "" {
		inviteCode := users.NormalizeInviteCode(inviteCodeInput)
		if inviteCode == "" {
			writeError(w, newAppError(http.StatusBadRequest, "邀请码格式不正确"))
			return
		}
		if !inviteConfig.Enabled {
			writeError(w, newAppError(http.StatusForbidden, "邀请活动暂未开放"))
			return
		}
		if !inviteRewardSpecConfigured(inviteConfig.InviterReward) || !inviteRewardSpecConfigured(inviteConfig.InviteeReward) {
			writeError(w, newAppError(http.StatusServiceUnavailable, "邀请奖励尚未配置完整，请联系管理员"))
			return
		}
		if !anyBool(settingValues["emailEnabled"]) {
			writeError(w, newAppError(http.StatusServiceUnavailable, "邀请注册需要先启用邮箱验证服务"))
			return
		}
		var err error
		inviter, err = repo.FindByInviteCode(ctx, inviteCode)
		if errors.Is(err, sql.ErrNoRows) || inviter == nil || inviter.Status != "active" || inviter.EmailVerifiedAt == nil || strings.EqualFold(inviter.Email, email) {
			writeError(w, newAppError(http.StatusBadRequest, "邀请码无效或暂不可用"))
			return
		}
		if err != nil {
			writeError(w, err)
			return
		}
		emailVerificationRequired = true
	}
	now := time.Now()
	var emailVerifiedAt *time.Time
	if !emailVerificationRequired {
		emailVerifiedAt = &now
	}
	userID := newID()
	operationRepo := operations.NewRepository(r.db)
	var binding *operations.InviteRewardResult
	var err error
	if inviter != nil {
		binding, err = operationRepo.BindInvite(ctx, operations.InviteBindingInput{
			InviterID:     inviter.ID,
			InviteeID:     userID,
			InviteeIP:     fingerprint.IP,
			IPHash:        fingerprint.IPHash,
			DeviceHash:    fingerprint.DeviceHash,
			InviterReward: inviteConfig.InviterReward,
			InviteeReward: inviteConfig.InviteeReward,
			Risk:          inviteConfig.Risk,
		})
		if err != nil {
			writeError(w, err)
			return
		}
	}
	invitedBy := ""
	invitedIP := ""
	if inviter != nil {
		invitedBy = inviter.ID
		invitedIP = fingerprint.IP
	}
	user, err := repo.Create(ctx, users.User{
		ID:              userID,
		Email:           email,
		InvitedBy:       invitedBy,
		InvitedIP:       invitedIP,
		PasswordHash:    auth.HashPassword(input.Password),
		Credits:         0,
		Role:            role,
		Status:          "active",
		EmailVerifiedAt: emailVerifiedAt,
	})
	if err != nil {
		if binding != nil {
			_ = operationRepo.RemovePendingInvite(context.Background(), userID)
		}
		writeError(w, err)
		return
	}
	if !admin {
		if err := r.recordRegistrationFingerprint(ctx, user.ID, fingerprint); err != nil {
			if binding != nil {
				_ = operationRepo.RemovePendingInvite(context.Background(), user.ID)
			}
			_, _ = repo.Delete(context.Background(), user.ID)
			writeError(w, err)
			return
		}
	}
	if admin {
		writeJSON(w, http.StatusCreated, map[string]any{"data": users.ToPublicUser(user)})
		return
	}
	if emailVerificationRequired {
		verificationData, err := r.sendRegistrationVerification(ctx, req, user, settingValues)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"data": verificationData})
		return
	}
	token, _ := r.tokens.CreateUserToken(user.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"data": mergeUserToken(users.ToPublicUser(user), token)})
}
