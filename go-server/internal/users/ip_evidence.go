package users

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"aipi-go/internal/database"
)

const (
	IPSourceLogin = "login"
	IPSourceAPI   = "api"
)

func (r *Repository) RecordIPEvidence(ctx context.Context, userID string, sourceType string, ipAddress string, apiKeyID string) error {
	userID = strings.TrimSpace(userID)
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	ipAddress = strings.TrimSpace(ipAddress)
	apiKeyID = strings.TrimSpace(apiKeyID)
	if userID == "" || ipAddress == "" || (sourceType != IPSourceLogin && sourceType != IPSourceAPI) {
		return nil
	}

	sum := sha256.Sum256([]byte("ip:" + ipAddress))
	ipHash := hex.EncodeToString(sum[:])
	var keyValue any
	if apiKeyID != "" {
		keyValue = apiKeyID
	}

	if database.CurrentDialect() == database.DialectPostgres {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO user_ip_evidence
				(user_id, ip_hash, ip_address, source_type, api_key_id, first_seen_at, last_seen_at, hit_count)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
			ON CONFLICT (user_id, ip_hash, source_type) DO UPDATE SET
				ip_address=EXCLUDED.ip_address,
				api_key_id=EXCLUDED.api_key_id,
				last_seen_at=CURRENT_TIMESTAMP,
				hit_count=user_ip_evidence.hit_count + 1
		`, userID, ipHash, ipAddress, sourceType, keyValue)
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_ip_evidence
			(user_id, ip_hash, ip_address, source_type, api_key_id, first_seen_at, last_seen_at, hit_count)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
		ON DUPLICATE KEY UPDATE
			ip_address=VALUES(ip_address),
			api_key_id=VALUES(api_key_id),
			last_seen_at=CURRENT_TIMESTAMP,
			hit_count=hit_count + 1
	`, userID, ipHash, ipAddress, sourceType, keyValue)
	return err
}
