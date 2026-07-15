package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/apikeys"
	"aipi-go/internal/config"
	"aipi-go/internal/users"
)

func (r *Router) oauthClient(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	client, err := r.oauthFindClient(req.URL.Query().Get("client_id"), req.URL.Query().Get("redirect_uri"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": client.ID, "name": client.Name, "redirectUri": client.RedirectURI}})
}

func (r *Router) oauthAuthorize(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserToken    string `json:"userToken"`
		ClientID     string `json:"client_id"`
		RedirectURI  string `json:"redirect_uri"`
		ResponseType string `json:"response_type"`
		State        string `json:"state"`
		Scope        string `json:"scope"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	client, err := r.oauthFindClient(input.ClientID, input.RedirectURI)
	if err != nil {
		writeError(w, err)
		return
	}
	payload, err := r.tokens.ParseUserToken(strings.TrimSpace(input.UserToken))
	if err != nil {
		writeError(w, newAppError(http.StatusUnauthorized, "请先登录"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	if _, err := users.NewRepository(r.db).FindByID(ctx, payload.UserID); err != nil {
		writeError(w, err)
		return
	}
	code := opaqueToken("aipi-code")
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO oauth_authorization_codes (code, client_id, user_id, redirect_uri, scope, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, code, client.ID, payload.UserID, client.RedirectURI, input.Scope, time.Now().Add(5*time.Minute)); err != nil {
		writeError(w, err)
		return
	}
	redirectURL := client.RedirectURI
	sep := "?"
	if strings.Contains(redirectURL, "?") {
		sep = "&"
	}
	redirectURL += sep + "code=" + urlQueryEscape(code)
	if input.State != "" {
		redirectURL += "&state=" + urlQueryEscape(input.State)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"redirectUrl": redirectURL, "code": code}})
}

func (r *Router) oauthToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		GrantType    string `json:"grant_type"`
		Code         string `json:"code"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RedirectURI  string `json:"redirect_uri"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	client, err := r.oauthFindClient(input.ClientID, input.RedirectURI)
	if err != nil {
		writeError(w, err)
		return
	}
	if client.Secret != strings.TrimSpace(input.ClientSecret) {
		writeError(w, newAppError(http.StatusUnauthorized, "client_secret 不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, err)
		return
	}
	defer tx.Rollback()
	var userID, redirectURI string
	var expiresAt time.Time
	var usedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id, redirect_uri, expires_at, used_at
		FROM oauth_authorization_codes
		WHERE code = ? AND client_id = ?
		FOR UPDATE
	`, input.Code, client.ID).Scan(&userID, &redirectURI, &expiresAt, &usedAt); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "授权码无效或已过期"))
		return
	}
	if usedAt.Valid || expiresAt.Before(time.Now()) || redirectURI != client.RedirectURI {
		writeError(w, newAppError(http.StatusBadRequest, "授权码无效或已过期"))
		return
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_authorization_codes SET used_at = NOW() WHERE code = ?`, input.Code); err != nil {
		writeError(w, err)
		return
	}
	accessToken := opaqueToken("aipi-oauth")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO oauth_access_tokens (token_hash, client_id, user_id, expires_at)
		VALUES (?, ?, ?, ?)
	`, hashOpaque(accessToken), client.ID, userID, time.Now().Add(24*time.Hour)); err != nil {
		writeError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": accessToken, "token_type": "Bearer", "expires_in": 86400})
}

func (r *Router) oauthMe(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	token := bearerToken(req)
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	var userID string
	var expiresAt time.Time
	if err := r.db.QueryRowContext(ctx, `SELECT user_id, expires_at FROM oauth_access_tokens WHERE token_hash=? LIMIT 1`, hashOpaque(token)).Scan(&userID, &expiresAt); err != nil {
		writeError(w, newAppError(http.StatusUnauthorized, "OAuth token 无效或已过期"))
		return
	}
	if expiresAt.Before(time.Now()) {
		writeError(w, newAppError(http.StatusUnauthorized, "OAuth token 无效或已过期"))
		return
	}
	user, err := users.NewRepository(r.db).FindByID(ctx, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	keyService := apikeys.NewService(apikeys.NewRepository(r.db), users.NewRepository(r.db))
	keys, err := keyService.ListUserKeys(ctx, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	var selected *apikeys.PublicAPIKey
	for index := range keys {
		if keys[index].Status == "active" {
			selected = &keys[index]
			break
		}
	}
	if selected == nil {
		created, err := keyService.CreateUserKey(ctx, userID, "Canvas OAuth")
		if err != nil {
			writeError(w, err)
			return
		}
		selected = created
	}
	plain := ""
	if selected.Key != nil {
		plain = *selected.Key
	} else if selected.KeyPlain != nil {
		plain = *selected.KeyPlain
	}
	if plain == "" {
		writeError(w, newAppError(http.StatusConflict, "当前授权配置不可用，请联系管理员处理"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"user": users.ToPublicUser(user),
		"apiKey": map[string]any{
			"id": selected.ID, "name": selected.Name, "keyPrefix": selected.KeyPrefix, "key": plain,
		},
	}})
}

func (r *Router) oauthFindClient(clientID string, redirectURI string) (*config.OAuthClient, error) {
	clientID = strings.TrimSpace(clientID)
	redirectURI = strings.TrimSpace(redirectURI)
	for index := range r.cfg.OAuthClients {
		client := &r.cfg.OAuthClients[index]
		if client.ID != clientID {
			continue
		}
		if client.RedirectURI != redirectURI {
			return nil, newAppError(http.StatusBadRequest, "redirect_uri 不匹配")
		}
		return client, nil
	}
	return nil, newAppError(http.StatusBadRequest, "OAuth client 不存在")
}

func opaqueToken(prefix string) string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return prefix + "-" + newID()
	}
	return prefix + "-" + base64.RawURLEncoding.EncodeToString(bytes)
}

func hashOpaque(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(" ", "%20", "+", "%2B", "&", "%26", "=", "%3D", "?", "%3F", "#", "%23")
	return replacer.Replace(value)
}

var _ = errors.Is
