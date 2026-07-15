package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/auth"
	"aipi-go/internal/users"
)

func (r *Router) adminLogin(w http.ResponseWriter, req *http.Request) {
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
	if input.Email == "" || len(input.Password) < 6 {
		writeError(w, newAppError(http.StatusBadRequest, "请输入管理员账号和密码"))
		return
	}
	if !strings.Contains(input.Email, "@") {
		input.Email += "@local.com"
	}

	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByEmail(ctx, input.Email)
	if errors.Is(err, sql.ErrNoRows) || user == nil || !auth.VerifyPassword(input.Password, user.PasswordHash) {
		writeError(w, newAppError(http.StatusUnauthorized, "管理员账号或密码错误"))
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	if user.Role != "admin" {
		writeError(w, newAppError(http.StatusForbidden, "当前账号不是管理员"))
		return
	}
	if user.Status != "active" {
		writeError(w, newAppError(http.StatusForbidden, "管理员账号已被禁用"))
		return
	}
	token, err := r.tokens.CreateAdminToken(user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user": map[string]string{
				"id":    user.ID,
				"email": user.Email,
				"role":  user.Role,
			},
			"token": token,
		},
	})
}

func (r *Router) adminSession(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	payload, err := r.requireAdmin(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"userId":    payload.UserID,
			"expiresAt": time.UnixMilli(payload.Exp).Format(time.RFC3339),
		},
	})
}

func (r *Router) requireAdmin(req *http.Request) (*auth.TokenPayload, error) {
	token := bearerToken(req)
	if token == "" {
		return nil, newAppError(http.StatusUnauthorized, "请先登录后台")
	}
	payload, err := r.tokens.ParseAdminToken(token)
	if err != nil {
		return nil, newAppError(http.StatusUnauthorized, "后台登录已失效，请重新登录")
	}
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(ctx, payload.UserID)
	if errors.Is(err, sql.ErrNoRows) || user == nil || user.Role != "admin" || user.Status != "active" {
		return nil, newAppError(http.StatusForbidden, "后台权限不足")
	}
	if err != nil {
		return nil, err
	}
	return payload, nil
}
