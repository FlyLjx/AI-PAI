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
	for _, want := range []string{"AI-PAI API 中转站", "自动发送，请勿直接回复", "立即验证邮箱", "按钮无法打开时，请复制此链接"} {
		if !strings.Contains(body, want) {
			t.Fatalf("mail HTML does not contain %q", want)
		}
	}
	for _, unwanted := range []string{"linear-gradient", "border-radius:999px", "box-shadow:"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("mail HTML still contains decorative style %q", unwanted)
		}
	}
	if strings.Contains(body, "生图站") {
		t.Fatal("mail HTML still contains legacy image-site wording")
	}
}
