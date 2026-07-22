package httpserver

import (
	"context"
	"encoding/xml"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const openAIStatusFeedEndpoint = "https://status.openai.com/feed.rss"

var (
	openAIStatusTagPattern = regexp.MustCompile(`(?is)<[^>]+>`)
	openAIStatusBRPattern  = regexp.MustCompile(`(?i)<br\s*/?>`)
	openAIStatusLIPattern  = regexp.MustCompile(`(?is)<li>\s*(.*?)\s*</li>`)
	openAIStatusRe         = regexp.MustCompile(`(?is)Status:\s*([^<\n\r]+)`)
)

type openAIStatusRSS struct {
	Channel openAIStatusChannel `xml:"channel"`
}

type openAIStatusChannel struct {
	Title         string             `xml:"title"`
	Link          string             `xml:"link"`
	Description   string             `xml:"description"`
	LastBuildDate string             `xml:"lastBuildDate"`
	Items         []openAIStatusItem `xml:"item"`
}

type openAIStatusItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Content     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

type openAIImageIncident struct {
	Title              string                    `json:"title"`
	Link               string                    `json:"link"`
	GUID               string                    `json:"guid"`
	PubDate            string                    `json:"pubDate"`
	PublishedAt        string                    `json:"publishedAt,omitempty"`
	Status             string                    `json:"status"`
	StatusLabel        string                    `json:"statusLabel"`
	Severity           string                    `json:"severity"`
	Summary            string                    `json:"summary"`
	AffectedComponents []openAIAffectedComponent `json:"affectedComponents"`
}

type openAIAffectedComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Label  string `json:"label"`
}

func (r *Router) openAIStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, map[string]any{"data": fetchOpenAIImageStatusSnapshot(ctx)})
}

func fetchOpenAIImageStatusSnapshot(ctx context.Context) map[string]any {
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, openAIStatusFeedEndpoint, nil)
	if err != nil {
		return openAIStatusFallback("请求创建失败："+err.Error(), 0)
	}
	upstreamReq.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.8")
	upstreamReq.Header.Set("User-Agent", "AI-PAI status subscriber")
	resp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		return openAIStatusFallback("OpenAI 状态源连接失败："+err.Error(), 0)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return openAIStatusFallback("OpenAI 状态源返回异常："+strings.TrimSpace(string(body)), resp.StatusCode)
	}
	var feed openAIStatusRSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return openAIStatusFallback("OpenAI RSS 格式解析失败："+err.Error(), resp.StatusCode)
	}
	incidents := make([]openAIImageIncident, 0, 8)
	for _, item := range feed.Channel.Items {
		if !openAIImageItem(item) {
			continue
		}
		incidents = append(incidents, openAIImageIncidentFromItem(item))
	}
	sort.SliceStable(incidents, func(i, j int) bool {
		left, leftOK := parseRSSDate(incidents[i].PubDate)
		right, rightOK := parseRSSDate(incidents[j].PubDate)
		if leftOK && rightOK {
			return left.After(right)
		}
		return i < j
	})
	latest := any(nil)
	status := "operational"
	statusLabel := "Image 正常"
	severity := "ok"
	summary := "OpenAI 状态源暂未发现正在影响 Image / Image Generation 的事件。"
	components := []openAIAffectedComponent{}
	if len(incidents) > 0 {
		item := incidents[0]
		latest = item
		status, severity = classifyOpenAIImageIncident(item)
		statusLabel = openAIOverallStatusLabel(status)
		summary = item.Summary
		if strings.EqualFold(item.Status, "resolved") {
			summary = "最近一次 Image 相关事件已恢复：" + item.Title
		}
		components = item.AffectedComponents
	}
	return map[string]any{
		"reachable":            true,
		"status":               status,
		"statusLabel":          statusLabel,
		"severity":             severity,
		"summary":              summary,
		"source":               openAIStatusFeedEndpoint,
		"feedTitle":            strings.TrimSpace(feed.Channel.Title),
		"feedLink":             strings.TrimSpace(feed.Channel.Link),
		"lastBuildDate":        strings.TrimSpace(feed.Channel.LastBuildDate),
		"fetchedAt":            time.Now().Format(time.RFC3339),
		"upstream_status_code": resp.StatusCode,
		"latestImageIncident":  latest,
		"imageIncidents":       incidents,
		"affectedComponents":   components,
		"totalImageIncidents":  len(incidents),
	}
}

func openAIStatusFallback(message string, upstreamStatusCode int) map[string]any {
	return map[string]any{
		"reachable":            false,
		"status":               "unreachable",
		"statusLabel":          "状态源不可达",
		"severity":             "critical",
		"summary":              strings.TrimSpace(message),
		"source":               openAIStatusFeedEndpoint,
		"fetchedAt":            time.Now().Format(time.RFC3339),
		"upstream_status_code": upstreamStatusCode,
		"latestImageIncident":  nil,
		"imageIncidents":       []openAIImageIncident{},
		"affectedComponents":   []openAIAffectedComponent{},
		"totalImageIncidents":  0,
		"error":                strings.TrimSpace(message),
	}
}

func openAIImageItem(item openAIStatusItem) bool {
	text := strings.ToLower(item.Title + " " + item.Description + " " + item.Content)
	return strings.Contains(text, "image")
}

func openAIImageIncidentFromItem(item openAIStatusItem) openAIImageIncident {
	content := item.Content
	if strings.TrimSpace(content) == "" {
		content = item.Description
	}
	status := extractOpenAIIncidentStatus(content)
	components := extractOpenAIAffectedComponents(content)
	publishedAt := ""
	if parsed, ok := parseRSSDate(item.PubDate); ok {
		publishedAt = parsed.Format(time.RFC3339)
	}
	return openAIImageIncident{
		Title:              strings.TrimSpace(item.Title),
		Link:               normalizeOpenAIStatusLink(item.Link),
		GUID:               normalizeOpenAIStatusLink(item.GUID),
		PubDate:            strings.TrimSpace(item.PubDate),
		PublishedAt:        publishedAt,
		Status:             strings.ToLower(strings.TrimSpace(status)),
		StatusLabel:        openAIIncidentStatusLabel(status),
		Severity:           openAIIncidentSeverity(status, components),
		Summary:            extractOpenAIIncidentSummary(content),
		AffectedComponents: components,
	}
}

func extractOpenAIIncidentStatus(content string) string {
	matches := openAIStatusRe.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(stripOpenAIHTML(matches[1]))
	}
	return "unknown"
}

func extractOpenAIAffectedComponents(content string) []openAIAffectedComponent {
	matches := openAIStatusLIPattern.FindAllStringSubmatch(content, -1)
	result := make([]openAIAffectedComponent, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		text := stripOpenAIHTML(match[1])
		name, status := splitOpenAIComponent(text)
		if name == "" {
			continue
		}
		result = append(result, openAIAffectedComponent{
			Name:   name,
			Status: strings.ToLower(status),
			Label:  openAIComponentStatusLabel(status),
		})
	}
	return result
}

func splitOpenAIComponent(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if closeIndex := strings.LastIndex(value, ")"); closeIndex == len(value)-1 {
		if openIndex := strings.LastIndex(value[:closeIndex], "("); openIndex > 0 {
			name := strings.TrimSpace(value[:openIndex])
			status := strings.TrimSpace(value[openIndex+1 : closeIndex])
			return name, status
		}
	}
	return value, ""
}

func extractOpenAIIncidentSummary(content string) string {
	text := stripOpenAIHTML(content)
	lower := strings.ToLower(text)
	if index := strings.Index(lower, "affected components"); index >= 0 {
		text = strings.TrimSpace(text[:index])
	}
	status := extractOpenAIIncidentStatus(content)
	prefix := "Status: " + status
	if strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix)) {
		text = strings.TrimSpace(text[len(prefix):])
	}
	text = strings.Trim(text, " -—：:;；")
	if text == "" {
		text = "OpenAI 状态源未提供更多说明。"
	}
	return text
}

func stripOpenAIHTML(value string) string {
	value = openAIStatusBRPattern.ReplaceAllString(value, "\n")
	value = strings.ReplaceAll(value, "</li>", "\n")
	value = openAIStatusTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func classifyOpenAIImageIncident(item openAIImageIncident) (string, string) {
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "resolved" {
		return "operational", "ok"
	}
	componentText := ""
	for _, component := range item.AffectedComponents {
		componentText += " " + component.Status + " " + component.Name
	}
	text := strings.ToLower(item.Title + " " + item.Summary + " " + componentText)
	switch {
	case strings.Contains(text, "major outage") || strings.Contains(text, "unavailable") || strings.Contains(text, "down"):
		return "outage", "critical"
	case strings.Contains(text, "partial outage") || strings.Contains(text, "elevated error") || strings.Contains(text, "increased error") || strings.Contains(text, "failing"):
		return "partial_outage", "critical"
	case strings.Contains(status, "monitor") || strings.Contains(status, "investigat") || strings.Contains(status, "identified") || strings.Contains(text, "degraded"):
		return "degraded", "warning"
	default:
		return "monitoring", "warning"
	}
}

func openAIIncidentSeverity(status string, components []openAIAffectedComponent) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "resolved" {
		return "ok"
	}
	for _, component := range components {
		statusText := strings.ToLower(component.Status)
		if strings.Contains(statusText, "outage") || strings.Contains(statusText, "down") || strings.Contains(statusText, "unavailable") {
			return "critical"
		}
	}
	return "warning"
}

func openAIOverallStatusLabel(status string) string {
	switch status {
	case "operational":
		return "Image 正常"
	case "outage":
		return "Image 不可用"
	case "partial_outage":
		return "Image 部分中断"
	case "degraded":
		return "Image 运行波动"
	case "unreachable":
		return "状态源不可达"
	default:
		return "Image 状态监控中"
	}
}

func openAIIncidentStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "resolved":
		return "已恢复"
	case "monitoring":
		return "监控恢复中"
	case "identified":
		return "已定位"
	case "investigating":
		return "调查中"
	default:
		return defaultString(strings.TrimSpace(status), "未知")
	}
}

func openAIComponentStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "operational":
		return "正常"
	case "partial outage":
		return "部分中断"
	case "major outage":
		return "严重中断"
	case "degraded performance":
		return "性能下降"
	default:
		return defaultString(strings.TrimSpace(status), "未知")
	}
}

func normalizeOpenAIStatusLink(value string) string {
	value = strings.TrimSpace(value)
	return strings.Replace(value, "https://status.openai.com//", "https://status.openai.com/", 1)
}

func parseRSSDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
