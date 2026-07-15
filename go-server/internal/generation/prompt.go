package generation

import (
	"fmt"
	"regexp"
	"strings"
)

func buildUpstreamPrompt(prompt string, size string, sizeTier string, appendSize bool, transparentBackground bool) string {
	transparentInstruction := ""
	if transparentBackground {
		transparentInstruction = "背景要求：输出透明背景 PNG，不要添加纯色底、渐变底、相框或额外背景。"
	}
	if !appendSize {
		return strings.TrimSpace(strings.Join(nonEmptyStrings(prompt, transparentInstruction), "\n\n"))
	}
	ratio := sizeRatio(size)
	return strings.TrimSpace(strings.Join(nonEmptyStrings(
		prompt,
		"画面尺寸要求：比例 "+ratio+"，输出尺寸 "+size+"，清晰度 "+strings.ToUpper(sizeTier)+"。请严格按照该比例和尺寸构图，不要生成其他画幅。",
		transparentInstruction,
	), "\n"))
}

func nonEmptyStrings(values ...string) []string {
	result := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, strings.TrimSpace(value))
		}
	}
	return result
}

func sizeRatio(size string) string {
	match := regexp.MustCompile(`^(\d+)x(\d+)$`).FindStringSubmatch(size)
	if len(match) != 3 {
		return "按所选尺寸"
	}
	w := atoi(match[1])
	h := atoi(match[2])
	if w == 0 || h == 0 {
		return "按所选尺寸"
	}
	g := gcd(w, h)
	return fmt.Sprintf("%d:%d", w/g, h/g)
}

func gcd(a int, b int) int {
	if b == 0 {
		return a
	}
	return gcd(b, a%b)
}

func atoi(value string) int {
	n := 0
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
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
