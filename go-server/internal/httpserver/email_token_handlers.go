package httpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/auth"
	"aipi-go/internal/settings"
	"aipi-go/internal/users"
)

const emailChangePurpose = "change_email"
const emailVerificationResendCooldown = time.Minute

func (r *Router) verifyEmail(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	userID, err := r.consumeUserEmailToken(req.Context(), input.Token, "verify_email")
	if err != nil {
		writeError(w, err)
		return
	}
	userRepo := users.NewRepository(r.db)
	user, err := userRepo.MarkEmailVerified(req.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	user = r.settleInviteRewards(req.Context(), user)
	r.publishCurrentUser(context.Background(), user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"data": r.publicUserWithSubscription(req.Context(), user)})
}

func (r *Router) resendEmailVerification(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if _, err := r.requireFrontUser(req, id); err != nil {
		writeError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if user == nil {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在"))
		return
	}
	if user.EmailVerifiedAt != nil {
		writeError(w, newAppError(http.StatusConflict, "邮箱已经完成验证"))
		return
	}

	var lastCreatedAt time.Time
	err = r.db.QueryRowContext(ctx, `
		SELECT created_at
		FROM user_email_tokens
		WHERE user_id = ? AND purpose = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, id, "verify_email").Scan(&lastCreatedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeError(w, err)
		return
	}
	if err == nil {
		remaining := time.Until(lastCreatedAt.Add(emailVerificationResendCooldown))
		if remaining > 0 {
			seconds := int(remaining/time.Second) + 1
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeError(w, newAppError(http.StatusTooManyRequests, "验证邮件发送过于频繁，请稍后再试"))
			return
		}
	}

	settingValues, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := r.sendEmailVerification(ctx, req, user, settingValues, false)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) changeUserEmail(w http.ResponseWriter, req *http.Request, id string) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID   string `json:"userId"`
		Password string `json:"password"`
		Email    string `json:"email"`
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
		writeError(w, newAppError(http.StatusForbidden, "只能修改自己的邮箱"))
		return
	}
	newEmail, err := normalizeAccountEmail(input.Email)
	if err != nil {
		writeError(w, newAppError(http.StatusBadRequest, err.Error()))
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
	if !auth.VerifyPassword(input.Password, user.PasswordHash) {
		writeError(w, newAppError(http.StatusBadRequest, "当前密码不正确"))
		return
	}
	if strings.EqualFold(user.Email, newEmail) {
		writeError(w, newAppError(http.StatusBadRequest, "新邮箱不能与当前邮箱相同"))
		return
	}
	if existing, findErr := repo.FindByEmail(ctx, newEmail); findErr == nil && existing != nil {
		writeError(w, newAppError(http.StatusConflict, "该邮箱已被其他账户使用"))
		return
	} else if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
		writeError(w, findErr)
		return
	}

	token, err := r.createEmailChangeToken(ctx, user.ID, newEmail, 2*time.Hour)
	if err != nil {
		writeError(w, err)
		return
	}
	verificationURL := absoluteURL(req, "/?changeEmailToken="+url.QueryEscape(token))
	message := "验证链接已生成；配置邮件服务后可自动发送。"
	sent := false
	if settingValues, settingsErr := settings.NewRepository(r.db).Get(ctx); settingsErr == nil {
		smtpConfig := smtpSettingsFromMap(settingValues)
		if smtpConfig.validate() == nil {
			siteName := emailBrandName(anyString(settingValues["siteName"]))
			body := "你正在将 " + siteName + " 账户的登录邮箱修改为此邮箱。请在 2 小时内打开以下链接完成验证：\n\n" + verificationURL + "\n\n如果不是你本人操作，请忽略这封邮件，原邮箱不会改变。"
			if sendErr := sendSMTPMail(smtpConfig, newEmail, "确认修改 "+siteName+" 登录邮箱", body, mailAction{Text: "确认修改邮箱", URL: verificationURL}); sendErr != nil {
				message = "验证链接已生成，但邮件发送失败：" + sendErr.Error()
			} else {
				message = "验证邮件已发送到新邮箱，请在 2 小时内完成确认。"
				sent = true
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"sent":            sent,
		"email":           newEmail,
		"verificationUrl": verificationURL,
		"message":         message,
	}})
}

func (r *Router) verifyEmailChange(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	newEmail, err := emailFromChangeToken(input.Token)
	if err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "链接已失效，请重新操作"))
		return
	}
	userID, err := r.consumeEmailChangeToken(req.Context(), input.Token, newEmail)
	if err != nil {
		writeError(w, err)
		return
	}
	repo := users.NewRepository(r.db)
	user, err := repo.FindByID(req.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	r.publishCurrentUser(context.Background(), user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"data": r.publicUserWithSubscription(req.Context(), user)})
}

func (r *Router) forgotPassword(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		Email string `json:"email"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	email := strings.TrimSpace(input.Email)
	if email == "" {
		writeError(w, newAppError(http.StatusBadRequest, "请输入邮箱"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) || user == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"sent": true}})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	token, err := r.createUserEmailToken(ctx, user.ID, "reset_password", 2*time.Hour)
	if err != nil {
		writeError(w, err)
		return
	}
	resetURL := absoluteURL(req, "/?resetPasswordToken="+token)
	message := "密码重置链接已生成；配置邮件服务后可自动发送。"
	if settingValues, err := settings.NewRepository(r.db).Get(ctx); err == nil {
		smtpConfig := smtpSettingsFromMap(settingValues)
		if smtpConfig.validate() == nil {
			siteName := emailBrandName(anyString(settingValues["siteName"]))
			body := "你正在重置 " + siteName + " 账户密码，请在 2 小时内打开以下链接完成操作：\n\n" + resetURL + "\n\n如果不是你本人操作，请忽略这封邮件。"
			if err := sendSMTPMail(smtpConfig, user.Email, "重置 "+siteName+" 账户密码", body, mailAction{Text: "立即重置密码", URL: resetURL}); err != nil {
				message = "密码重置链接已生成，但邮件发送失败：" + err.Error()
			} else {
				message = "密码重置邮件已发送，请查收。"
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"sent":     true,
		"resetUrl": resetURL,
		"message":  message,
	}})
}

func (r *Router) resetPassword(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	if len(input.Password) < 6 {
		writeError(w, newAppError(http.StatusBadRequest, "密码至少 6 位"))
		return
	}
	userID, err := r.consumeUserEmailToken(req.Context(), input.Token, "reset_password")
	if err != nil {
		writeError(w, err)
		return
	}
	user, err := users.NewRepository(r.db).UpdatePassword(req.Context(), userID, auth.HashPassword(input.Password))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users.ToPublicUser(user)})
}

func (r *Router) sendRegistrationVerification(ctx context.Context, req *http.Request, user *users.User, settingValues map[string]any) (map[string]any, error) {
	return r.sendEmailVerification(ctx, req, user, settingValues, true)
}

func (r *Router) sendEmailVerification(ctx context.Context, req *http.Request, user *users.User, settingValues map[string]any, registration bool) (map[string]any, error) {
	token, err := r.createUserEmailToken(ctx, user.ID, "verify_email", 24*time.Hour)
	if err != nil {
		return nil, err
	}
	verifyURL := absoluteURL(req, "/?verifyEmailToken="+token)
	message := "验证链接已生成；配置邮件服务后可自动发送。"
	if registration {
		message = "注册成功，" + message
	}
	sent := false
	if settingValues == nil {
		settingValues = map[string]any{}
	}
	smtpConfig := smtpSettingsFromMap(settingValues)
	if smtpConfig.validate() == nil {
		siteName := emailBrandName(anyString(settingValues["siteName"]))
		body := "你正在验证 " + siteName + " 账户邮箱，请在 24 小时内打开以下链接完成验证：\n\n" + verifyURL + "\n\n如果不是你本人操作，请忽略这封邮件。"
		if err := sendSMTPMail(smtpConfig, user.Email, "验证 "+siteName+" 账户邮箱", body, mailAction{Text: "立即验证邮箱", URL: verifyURL}); err != nil {
			message = "验证邮件发送失败：" + err.Error()
			if registration {
				message = "注册成功，但" + message
			}
		} else {
			message = "验证邮件已重新发送，请查收后完成验证。"
			if registration {
				message = "注册成功，验证邮件已发送，请查收后完成验证。"
			}
			sent = true
		}
	}
	return map[string]any{
		"verificationRequired": true,
		"email":                user.Email,
		"sent":                 sent,
		"verificationUrl":      verifyURL,
		"message":              message,
	}, nil
}

func (r *Router) createUserEmailToken(ctx context.Context, userID string, purpose string, ttl time.Duration) (string, error) {
	token := newID() + newID()
	hash := hashUserEmailToken(token)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_email_tokens (token_hash, user_id, purpose, expires_at)
		VALUES (?, ?, ?, ?)
	`, hash, userID, purpose, time.Now().Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (r *Router) createEmailChangeToken(ctx context.Context, userID string, email string, ttl time.Duration) (string, error) {
	token := base64.RawURLEncoding.EncodeToString([]byte(email)) + "." + newID() + newID()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE user_email_tokens
		SET used_at = NOW()
		WHERE user_id = ? AND purpose = ? AND used_at IS NULL
	`, userID, emailChangePurpose); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_email_tokens (token_hash, user_id, purpose, expires_at)
		VALUES (?, ?, ?, ?)
	`, hashUserEmailToken(token), userID, emailChangePurpose, time.Now().Add(ttl)); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

func (r *Router) consumeUserEmailToken(ctx context.Context, token string, purpose string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", newAppError(http.StatusBadRequest, "链接已失效，请重新操作")
	}
	hash := hashUserEmailToken(token)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var userID string
	var expiresAt time.Time
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, expires_at, used_at
		FROM user_email_tokens
		WHERE token_hash = ? AND purpose = ?
		FOR UPDATE
	`, hash, purpose).Scan(&userID, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", newAppError(http.StatusBadRequest, "链接已失效，请重新操作")
	}
	if err != nil {
		return "", err
	}
	if usedAt.Valid || expiresAt.Before(time.Now()) {
		return "", newAppError(http.StatusBadRequest, "链接已失效，请重新操作")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_email_tokens SET used_at = NOW() WHERE token_hash = ?`, hash); err != nil {
		return "", err
	}
	return userID, tx.Commit()
}

func (r *Router) consumeEmailChangeToken(ctx context.Context, token string, email string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", newAppError(http.StatusBadRequest, "链接已失效，请重新操作")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var userID string
	var expiresAt time.Time
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, expires_at, used_at
		FROM user_email_tokens
		WHERE token_hash = ? AND purpose = ?
		FOR UPDATE
	`, hashUserEmailToken(token), emailChangePurpose).Scan(&userID, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", newAppError(http.StatusBadRequest, "链接已失效，请重新操作")
	}
	if err != nil {
		return "", err
	}
	if usedAt.Valid || expiresAt.Before(time.Now()) {
		return "", newAppError(http.StatusBadRequest, "链接已失效，请重新操作")
	}

	var existingUserID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ? AND id <> ? LIMIT 1`, email, userID).Scan(&existingUserID)
	if err == nil {
		return "", newAppError(http.StatusConflict, "该邮箱已被其他账户使用")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET email = ?, email_verified_at = NOW() WHERE id = ?`, email, userID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_email_tokens SET used_at = NOW() WHERE token_hash = ?`, hashUserEmailToken(token)); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return userID, nil
}

func hashUserEmailToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeAccountEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" {
		return "", errors.New("请输入新邮箱")
	}
	if len(email) > 120 {
		return "", errors.New("邮箱长度不能超过 120 个字符")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", errors.New("请输入有效的邮箱地址")
	}
	return email, nil
}

func emailFromChangeToken(token string) (string, error) {
	parts := strings.SplitN(strings.TrimSpace(token), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("invalid email change token")
	}
	rawEmail, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	email, err := normalizeAccountEmail(string(rawEmail))
	if err != nil || email != string(rawEmail) {
		return "", errors.New("invalid email change token")
	}
	return email, nil
}
