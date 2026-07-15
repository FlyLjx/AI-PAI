package httpserver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/auth"
	"aipi-go/internal/settings"
	"aipi-go/internal/users"
)

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
			body := "你正在重置 AI-PAI 账户密码，请在 2 小时内打开以下链接完成操作：\n\n" + resetURL + "\n\n如果不是你本人操作，请忽略这封邮件。"
			if err := sendSMTPMail(smtpConfig, user.Email, "重置 AI-PAI 账户密码", body); err != nil {
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
	token, err := r.createUserEmailToken(ctx, user.ID, "verify_email", 24*time.Hour)
	if err != nil {
		return nil, err
	}
	verifyURL := absoluteURL(req, "/?verifyEmailToken="+token)
	message := "注册成功，请前往邮箱完成验证后再登录。"
	sent := false
	if settingValues == nil {
		settingValues = map[string]any{}
	}
	smtpConfig := smtpSettingsFromMap(settingValues)
	if smtpConfig.validate() == nil {
		siteName := strings.TrimSpace(anyString(settingValues["siteName"]))
		siteName = displayBrandName(siteName)
		body := "你正在注册 " + siteName + " 账号，请在 24 小时内打开以下链接完成邮箱验证：\n\n" + verifyURL + "\n\n如果不是你本人操作，请忽略这封邮件。"
		if err := sendSMTPMail(smtpConfig, user.Email, "验证 "+siteName+" 账号邮箱", body, mailAction{Text: "立即验证邮箱", URL: verifyURL}); err != nil {
			message = "注册成功，但验证邮件发送失败：" + err.Error()
		} else {
			message = "注册成功，验证邮件已发送，请查收后完成验证。"
			sent = true
		}
	} else {
		message = "注册成功，验证链接已生成；配置邮件服务后可自动发送。"
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

func hashUserEmailToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
