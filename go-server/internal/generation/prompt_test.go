package generation

import (
	"strings"
	"testing"
)

func TestBuildUpstreamPromptIncludesSelectedLandscapeRatio(t *testing.T) {
	prompt := buildUpstreamPrompt("城市夜景", "1536x864", "1k", true)
	for _, expected := range []string{"城市夜景", "比例 16:9", "输出尺寸 1536x864", "清晰度 1K", "严格按照该比例和尺寸构图"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt %q does not contain %q", prompt, expected)
		}
	}
}

func TestBuildUpstreamPromptLeavesDisabledModelUnchanged(t *testing.T) {
	if prompt := buildUpstreamPrompt("  城市夜景  ", "1536x864", "1k", false); prompt != "城市夜景" {
		t.Fatalf("prompt = %q", prompt)
	}
}
