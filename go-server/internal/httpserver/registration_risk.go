package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"aipi-go/internal/settings"
)

const registrationChallengeTTL = 20 * time.Minute

type registrationRiskConfig struct {
	Enabled               bool
	MaxPerIP24h           int
	MaxPerDevice24h       int
	ChallengeMinSeconds   int
	ChallengeMaxPerIPHour int
}

type registrationFingerprint struct {
	IP            string
	IPHash        string
	DeviceHash    string
	UserAgentHash string
}

func registrationRiskConfigFromSettings(values settings.Settings) registrationRiskConfig {
	return registrationRiskConfig{
		Enabled:               anyBool(values["registrationRiskEnabled"]),
		MaxPerIP24h:           positiveSettingInt(values["registrationRiskMaxPerIP24h"], 5),
		MaxPerDevice24h:       positiveSettingInt(values["registrationRiskMaxPerDevice24h"], 2),
		ChallengeMinSeconds:   positiveSettingInt(values["registrationChallengeMinSeconds"], 2),
		ChallengeMaxPerIPHour: positiveSettingInt(values["registrationChallengeMaxPerIPHour"], 30),
	}
}

func positiveSettingInt(value any, fallback int) int {
	number := int(anyFloat(value, float64(fallback)))
	if number < 1 {
		return fallback
	}
	return number
}

func (r *Router) registrationChallenge(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	values, err := settings.NewRepository(r.db).Get(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	config := registrationRiskConfigFromSettings(values)
	if !config.Enabled {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"required": false}})
		return
	}
	fingerprint := registrationFingerprintForRequest(req, "")
	var recent int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM registration_challenges
		WHERE ip_hash=? AND created_at>=DATE_SUB(NOW(), INTERVAL 1 HOUR)
	`, fingerprint.IPHash).Scan(&recent); err != nil {
		writeError(w, err)
		return
	}
	if recent >= config.ChallengeMaxPerIPHour {
		writeError(w, newAppError(http.StatusTooManyRequests, "注册请求过于频繁，请稍后再试"))
		return
	}
	token, err := randomRegistrationToken()
	if err != nil {
		writeError(w, err)
		return
	}
	now := time.Now()
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO registration_challenges (token_hash, ip_hash, expires_at)
		VALUES (?, ?, ?)
	`, hashRegistrationValue(token), fingerprint.IPHash, now.Add(registrationChallengeTTL)); err != nil {
		writeError(w, err)
		return
	}
	_, _ = r.db.ExecContext(ctx, `DELETE FROM registration_challenges WHERE expires_at<NOW() OR used_at<DATE_SUB(NOW(), INTERVAL 1 DAY)`)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"required":         true,
		"token":            token,
		"minDelaySeconds":  config.ChallengeMinSeconds,
		"expiresInSeconds": int(registrationChallengeTTL.Seconds()),
	}})
}

func (r *Router) validateRegistrationRisk(ctx context.Context, req *http.Request, challengeToken string, deviceID string, config registrationRiskConfig) (registrationFingerprint, error) {
	fingerprint := registrationFingerprintForRequest(req, deviceID)
	if !config.Enabled {
		return fingerprint, nil
	}
	challengeToken = strings.TrimSpace(challengeToken)
	if challengeToken == "" {
		return fingerprint, newAppError(http.StatusTooManyRequests, "注册安全校验已失效，请刷新页面后重试")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fingerprint, err
	}
	defer tx.Rollback()
	var ipHash string
	var createdAt, expiresAt time.Time
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT ip_hash, created_at, expires_at, used_at
		FROM registration_challenges
		WHERE token_hash=?
		FOR UPDATE
	`, hashRegistrationValue(challengeToken)).Scan(&ipHash, &createdAt, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return fingerprint, newAppError(http.StatusTooManyRequests, "注册安全校验已失效，请刷新页面后重试")
	}
	if err != nil {
		return fingerprint, err
	}
	now := time.Now()
	if usedAt.Valid || expiresAt.Before(now) || ipHash != fingerprint.IPHash {
		return fingerprint, newAppError(http.StatusTooManyRequests, "注册安全校验已失效，请刷新页面后重试")
	}
	if now.Before(createdAt.Add(time.Duration(config.ChallengeMinSeconds) * time.Second)) {
		return fingerprint, newAppError(http.StatusTooManyRequests, "操作过快，请稍后再提交注册")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE registration_challenges SET used_at=? WHERE token_hash=? AND used_at IS NULL`, now, hashRegistrationValue(challengeToken)); err != nil {
		return fingerprint, err
	}
	if err := tx.Commit(); err != nil {
		return fingerprint, err
	}

	checks := []struct {
		limit  int
		column string
		value  string
	}{
		{config.MaxPerIP24h, "ip_hash", fingerprint.IPHash},
		{config.MaxPerDevice24h, "device_hash", fingerprint.DeviceHash},
	}
	for _, check := range checks {
		if check.limit <= 0 || check.value == "" {
			continue
		}
		query := `SELECT COUNT(*) FROM user_registration_fingerprints WHERE ` + check.column + `=? AND created_at>=DATE_SUB(NOW(), INTERVAL 1 DAY)`
		var total int
		if err := r.db.QueryRowContext(ctx, query, check.value).Scan(&total); err != nil {
			return fingerprint, err
		}
		if total >= check.limit {
			return fingerprint, newAppError(http.StatusTooManyRequests, "当前网络或设备的注册次数已达到今日上限")
		}
	}
	return fingerprint, nil
}

func (r *Router) recordRegistrationFingerprint(ctx context.Context, userID string, fingerprint registrationFingerprint) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_registration_fingerprints (user_id, ip_hash, device_hash, user_agent_hash)
		VALUES (?, ?, ?, ?)
	`, strings.TrimSpace(userID), fingerprint.IPHash, optionalRegistrationValue(fingerprint.DeviceHash), optionalRegistrationValue(fingerprint.UserAgentHash))
	return err
}

func registrationFingerprintForRequest(req *http.Request, deviceID string) registrationFingerprint {
	ip := normalizedRegistrationIP(requestIP(req))
	deviceID = strings.TrimSpace(deviceID)
	userAgent := strings.TrimSpace(req.UserAgent())
	return registrationFingerprint{
		IP:            ip,
		IPHash:        hashRegistrationValue("ip:" + ip),
		DeviceHash:    hashOptionalRegistrationValue("device:" + deviceID),
		UserAgentHash: hashOptionalRegistrationValue("ua:" + userAgent),
	}
}

func normalizedRegistrationIP(value string) string {
	value = strings.TrimSpace(strings.Split(value, ",")[0])
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.Trim(value, "[]")
	if parsed := net.ParseIP(value); parsed != nil {
		return parsed.String()
	}
	return value
}

func hashOptionalRegistrationValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasSuffix(value, ":") {
		return ""
	}
	return hashRegistrationValue(value)
}

func hashRegistrationValue(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func randomRegistrationToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func optionalRegistrationValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
