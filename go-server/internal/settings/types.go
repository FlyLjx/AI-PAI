package settings

import (
	"math"
	"strings"
	"time"
)

const (
	MinTaskTimeoutMinutes     = 1
	MaxTaskTimeoutMinutes     = 120
	DefaultTaskTimeoutMinutes = 5
)

const DefaultImageSafetyKeywords = "色情,淫秽,裸体,裸身,裸照,裸女,裸男,性爱,性交,性行为,乳房,乳头,阴部,生殖器,自慰,口交,肛交,强奸,乱伦,porn,pornography,nude,nudity,nsfw,explicit sex,sexual intercourse,genitals,breasts,nipple,nipples,vagina,penis,masturbation,oral sex,anal sex,rape,incest"

const (
	maxImageSafetyKeywords    = 300
	maxImageSafetyKeywordSize = 120
	maxImageSafetyValueSize   = 24000
)

// TaskTimeout converts the persisted API timeout setting to a duration. The
// fallback is intentionally the historical five-minute timeout so a missing
// or malformed setting never leaves a task without a deadline.
func TaskTimeout(values Settings) time.Duration {
	minutes, ok := values["taskTimeoutMinutes"].(float64)
	if !ok || math.IsNaN(minutes) || math.IsInf(minutes, 0) || minutes < MinTaskTimeoutMinutes || minutes > MaxTaskTimeoutMinutes {
		return DefaultTaskTimeoutMinutes * time.Minute
	}
	return time.Duration(minutes * float64(time.Minute))
}

type ImageSafetyConfig struct {
	Enabled  bool
	Keywords []string
}

func NormalizeImageSafetyKeywords(value any) (string, error) {
	raw, ok := value.(string)
	if !ok {
		return "", ErrInvalidImageSafetySettings
	}
	if len(raw) > maxImageSafetyValueSize {
		return "", ErrInvalidImageSafetySettings
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		keyword := strings.TrimSpace(part)
		if keyword == "" {
			continue
		}
		if len([]rune(keyword)) > maxImageSafetyKeywordSize {
			return "", ErrInvalidImageSafetySettings
		}
		key := strings.ToLower(keyword)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, keyword)
		if len(result) > maxImageSafetyKeywords {
			return "", ErrInvalidImageSafetySettings
		}
	}
	return strings.Join(result, ", "), nil
}

func ImageSafetyConfigFromSettings(values Settings) ImageSafetyConfig {
	keywords, _ := NormalizeImageSafetyKeywords(values["imageSafetyKeywords"])
	parts := strings.FieldsFunc(keywords, func(r rune) bool {
		return r == ','
	})
	result := ImageSafetyConfig{
		Enabled:  values["imageSafetyEnabled"] == true,
		Keywords: make([]string, 0, len(parts)),
	}
	for _, keyword := range parts {
		if value := strings.ToLower(strings.TrimSpace(keyword)); value != "" {
			result.Keywords = append(result.Keywords, value)
		}
	}
	return result
}

func (config ImageSafetyConfig) Match(prompt string) string {
	if !config.Enabled {
		return ""
	}
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	if prompt == "" {
		return ""
	}
	for _, keyword := range config.Keywords {
		if keyword != "" && strings.Contains(prompt, keyword) {
			return keyword
		}
	}
	return ""
}

type Settings map[string]any

var Defaults = Settings{
	"siteName":                             "AI-PAI",
	"logoText":                             "AI-PAI",
	"creditName":                           "余额",
	"frontendUrl":                          "https://ai.yccc.me",
	"backendUrl":                           "http://localhost:3001",
	"supportEnabled":                       true,
	"supportTitle":                         "联系客服",
	"supportDescription":                   "遇到订阅、生成或账号问题，可以通过下面方式联系管理员。",
	"supportWechat":                        "",
	"supportQq":                            "",
	"supportGroupNumber":                   "",
	"supportGroupUrl":                      "",
	"supportEmail":                         "",
	"supportUrl":                           "",
	"supportQrCodeUrl":                     "",
	"rechargeEnabled":                      true,
	"rechargeRate":                         float64(10),
	"rechargeMinAmount":                    float64(1),
	"rechargePresets":                      "10,30,50,100",
	"subscriptionAccessUserId":             "",
	"subscriptionAccessUserIds":            "",
	"subscriptionAccessInitialized":        false,
	"inviteEnabled":                        false,
	"inviteRewardType":                     "subscription",
	"inviteRewardPlanId":                   "",
	"inviteInviterRewardType":              "subscription",
	"inviteInviterRewardCredits":           float64(0),
	"inviteInviterRewardPlanId":            "",
	"inviteInviteeRewardType":              "balance",
	"inviteInviteeRewardCredits":           float64(0),
	"inviteInviteeRewardPlanId":            "",
	"inviteRechargeRebateEnabled":          false,
	"inviteRechargeRebatePercent":          float64(5),
	"inviteRebateIncludeSubscriptions":     false,
	"inviteRiskEnabled":                    true,
	"inviteRiskManualReview":               true,
	"inviteRiskBlockSameIP":                true,
	"inviteRiskBlockSameDevice":            true,
	"inviteRiskMaxPerIP24h":                float64(2),
	"inviteRiskMaxPerDevice24h":            float64(1),
	"inviteRiskMaxPerInviter24h":           float64(10),
	"registrationRiskEnabled":              true,
	"registrationRiskMaxPerIP24h":          float64(5),
	"registrationRiskMaxPerDevice24h":      float64(2),
	"registrationChallengeMinSeconds":      float64(2),
	"registrationChallengeMaxPerIPHour":    float64(30),
	"taskTimeoutMinutes":                   float64(DefaultTaskTimeoutMinutes),
	"imageSafetyEnabled":                   true,
	"imageSafetyKeywords":                  DefaultImageSafetyKeywords,
	"systemLogAutoCleanupEnabled":          false,
	"systemLogRetentionDays":               float64(30),
	"taskImageAutoCleanupEnabled":          true,
	"taskImageRetentionDays":               float64(1),
	"streamGenerationEnabled":              false,
	"dynamicConcurrencyEnabled":            true,
	"dynamicConcurrencyWindowValue":        float64(1),
	"dynamicConcurrencyWindowUnit":         "hour",
	"dynamicConcurrencyRequestStep":        float64(50),
	"dynamicConcurrencyIncrement":          float64(5),
	"alipayAppId":                          "",
	"alipayPrivateKey":                     "",
	"alipayPublicKey":                      "",
	"alipayGateway":                        "https://openapi.alipay.com/gateway.do",
	"registerMode":                         "open",
	"emailEnabled":                         false,
	"emailHost":                            "",
	"emailPort":                            float64(465),
	"emailSecure":                          true,
	"emailUser":                            "",
	"emailPassword":                        "",
	"emailFromName":                        "AI-PAI",
	"emailFromAddress":                     "",
	"adminNotificationEmails":              "",
	"barkEnabled":                          false,
	"barkServerUrl":                        "https://api.day.app",
	"barkDeviceKey":                        "",
	"barkGroup":                            "AI-PAI",
	"barkSound":                            "",
	"barkNotifyError":                      true,
	"barkNotifyRecharge":                   true,
	"barkNotifyUpstream":                   true,
	"barkNotifySystem":                     false,
	"adminRechargeNotificationEnabled":     true,
	"adminUpstreamNotificationEnabled":     true,
	"adminOpenAIStatusNotificationEnabled": true,
	"adminUpstreamCheckIntervalMinutes":    float64(5),
	"upstreamMaintenanceEnabled":           false,
	"registerEmailVerification":            false,
}

var publicKeys = map[string]bool{
	"siteName":                   true,
	"logoText":                   true,
	"creditName":                 true,
	"frontendUrl":                true,
	"backendUrl":                 true,
	"supportEnabled":             true,
	"supportTitle":               true,
	"supportDescription":         true,
	"supportWechat":              true,
	"supportQq":                  true,
	"supportGroupNumber":         true,
	"supportGroupUrl":            true,
	"supportEmail":               true,
	"supportUrl":                 true,
	"supportQrCodeUrl":           true,
	"rechargeEnabled":            true,
	"rechargeRate":               true,
	"rechargeMinAmount":          true,
	"rechargePresets":            true,
	"inviteEnabled":              true,
	"inviteRewardType":           true,
	"inviteRewardPlanId":         true,
	"inviteInviterRewardType":    true,
	"inviteInviterRewardCredits": true,
	"inviteInviterRewardPlanId":  true,
	"inviteInviteeRewardType":    true,
	"inviteInviteeRewardCredits": true,
	"inviteInviteeRewardPlanId":  true,
	"taskTimeoutMinutes":         true,
	"streamGenerationEnabled":    true,
	"registerMode":               true,
	"registerEmailVerification":  true,
}

func Public(settings Settings) Settings {
	result := Settings{}
	for key, value := range settings {
		if publicKeys[key] {
			result[key] = value
		}
	}
	return result
}
