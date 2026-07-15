package settings

type Settings map[string]any

var Defaults = Settings{
	"siteName":                  "AI-PAI",
	"logoText":                  "AI-PAI",
	"creditName":                "余额",
	"frontendUrl":               "http://localhost:5173",
	"backendUrl":                "http://localhost:3001",
	"supportEnabled":            true,
	"supportTitle":              "联系客服",
	"supportDescription":        "遇到订阅、生成或账号问题，可以通过下面方式联系管理员。",
	"supportWechat":             "",
	"supportQq":                 "",
	"supportGroupNumber":        "",
	"supportGroupUrl":           "",
	"supportEmail":              "",
	"supportUrl":                "",
	"supportQrCodeUrl":          "",
	"rechargeEnabled":           true,
	"rechargeRate":              float64(1),
	"rechargeMinAmount":         float64(1),
	"rechargePresets":           "10,30,50,100",
	"inviteEnabled":             true,
	"inviteRewardType":          "subscription",
	"inviteRewardPlanId":        "",
	"freeHourlyGenerationQuota": float64(2),
	"freeDailyGenerationQuota":  float64(5),
	"freeGenerationQuota":       float64(10),
	"taskTimeoutMinutes":        float64(3),
	"streamGenerationEnabled":   false,
	"alipayAppId":               "",
	"alipayPrivateKey":          "",
	"alipayPublicKey":           "",
	"alipayGateway":             "https://openapi.alipay.com/gateway.do",
	"registerMode":              "open",
	"emailEnabled":              false,
	"emailHost":                 "",
	"emailPort":                 float64(465),
	"emailSecure":               true,
	"emailUser":                 "",
	"emailPassword":             "",
	"emailFromName":             "AI-PAI",
	"emailFromAddress":          "",
	"registerEmailVerification": false,
	"accountPoolEndpoint":       "https://free-api.yccc.me/api/accounts",
	"accountPoolApiKey":         "",
	"accountPoolAuthHeader":     "Authorization",
}

var publicKeys = map[string]bool{
	"siteName":                  true,
	"logoText":                  true,
	"creditName":                true,
	"frontendUrl":               true,
	"backendUrl":                true,
	"supportEnabled":            true,
	"supportTitle":              true,
	"supportDescription":        true,
	"supportWechat":             true,
	"supportQq":                 true,
	"supportGroupNumber":        true,
	"supportGroupUrl":           true,
	"supportEmail":              true,
	"supportUrl":                true,
	"supportQrCodeUrl":          true,
	"rechargeEnabled":           true,
	"rechargeRate":              true,
	"rechargeMinAmount":         true,
	"rechargePresets":           true,
	"inviteEnabled":             true,
	"inviteRewardType":          true,
	"inviteRewardPlanId":        true,
	"freeHourlyGenerationQuota": true,
	"freeDailyGenerationQuota":  true,
	"freeGenerationQuota":       true,
	"taskTimeoutMinutes":        true,
	"streamGenerationEnabled":   true,
	"registerMode":              true,
	"registerEmailVerification": true,
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
