package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/models"
	"aipi-go/internal/users"
)

func bearerToken(req *http.Request) string {
	header := req.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(req.URL.Query().Get("token"))
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func requestIP(req *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(req.Header.Get(header))
		if value == "" {
			continue
		}
		return strings.TrimSpace(strings.Split(value, ",")[0])
	}
	return req.RemoteAddr
}

func queryInt(req *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(req.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return fallback
		}
		n = n*10 + int(ch-'0')
	}
	if n == 0 {
		return fallback
	}
	return n
}

func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	value := hex.EncodeToString(bytes[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		value[0:8],
		value[8:12],
		value[12:16],
		value[16:20],
		value[20:32],
	)
}

func mergeUserToken(user users.PublicUser, token string) map[string]any {
	return map[string]any{
		"id":              user.ID,
		"email":           user.Email,
		"credits":         user.Credits,
		"inviteCode":      user.InviteCode,
		"role":            user.Role,
		"status":          user.Status,
		"emailVerifiedAt": user.EmailVerifiedAt,
		"createdAt":       user.CreatedAt,
		"updatedAt":       user.UpdatedAt,
		"subscription":    user.Subscription,
		"token":           token,
	}
}

func sizeTierEnabled(tiers []string, target string) bool {
	if len(tiers) == 0 {
		return true
	}
	for _, tier := range tiers {
		if tier == target {
			return true
		}
	}
	return false
}

func modelPriceForTier(model models.Model, tier string) float64 {
	switch tier {
	case "4k":
		return model.Price4K
	case "2k":
		return model.Price2K
	default:
		return model.Price1K
	}
}

func defaultImageSize(tier string) string {
	switch tier {
	case "4k":
		return "3072x3072"
	case "2k":
		return "2048x2048"
	default:
		return "1024x1024"
	}
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func parseFloat(value string) (float64, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
