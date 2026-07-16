package httpserver

import (
	"strings"
	"testing"
)

func TestEmailBrandNameNormalizesLegacyImageSiteNames(t *testing.T) {
	tests := []string{
		"AIπ - AI生图站",
		"AIπ - 在线生图站",
		"AI-PAI",
		"",
	}
	for _, value := range tests {
		if got := emailBrandName(value); got != "AI-PAI API 中转站" {
			t.Fatalf("emailBrandName(%q) = %q, want %q", value, got, "AI-PAI API 中转站")
		}
	}
}

func TestBuildMailHTMLUsesRelayBrandForLegacySettings(t *testing.T) {
	body := buildMailHTML(
		"AIπ - AI生图站",
		"验证 AI-PAI API 中转站账户邮箱",
		"请完成邮箱验证。",
		mailAction{Text: "立即验证邮箱", URL: "http://127.0.0.1:3000/?verifyEmailToken=test"},
	)
	for _, want := range []string{"AI-PAI API 中转站 · 账户通知", "账户与服务通知邮件", "立即验证邮箱"} {
		if !strings.Contains(body, want) {
			t.Fatalf("mail HTML does not contain %q", want)
		}
	}
	if strings.Contains(body, "生图站") {
		t.Fatal("mail HTML still contains legacy image-site wording")
	}
}
