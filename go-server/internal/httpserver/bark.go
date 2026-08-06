package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBarkServer = "https://api.day.app"
	barkPushTimeout   = 15 * time.Second
)

type barkSettings struct {
	Enabled        bool
	ServerURL      string
	DeviceKey      string
	Group          string
	Sound          string
	NotifyError    bool
	NotifyRecharge bool
	NotifyUpstream bool
	NotifySystem   bool
}

type barkSender func(context.Context, barkSettings, string, string, string) error

type barkPushRequest struct {
	DeviceKey string `json:"device_key"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Group     string `json:"group,omitempty"`
	Sound     string `json:"sound,omitempty"`
	Level     string `json:"level,omitempty"`
	URL       string `json:"url,omitempty"`
}

type barkPushResponse struct {
	Code    *int   `json:"code"`
	Message string `json:"message"`
}

func barkSettingsFromMap(values map[string]any) barkSettings {
	server := strings.TrimRight(strings.TrimSpace(anyString(values["barkServerUrl"])), "/")
	if server == "" {
		server = defaultBarkServer
	}
	deviceKey := strings.TrimSpace(anyString(values["barkDeviceKey"]))
	if deviceKey == "" {
		deviceKey = strings.TrimSpace(anyString(values["barkKey"]))
	}
	return barkSettings{
		Enabled:        anyBool(values["barkEnabled"]),
		ServerURL:      server,
		DeviceKey:      deviceKey,
		Group:          strings.TrimSpace(anyString(values["barkGroup"])),
		Sound:          strings.TrimSpace(anyString(values["barkSound"])),
		NotifyError:    anyBoolDefault(values["barkNotifyError"], true),
		NotifyRecharge: anyBoolDefault(values["barkNotifyRecharge"], true),
		NotifyUpstream: anyBoolDefault(values["barkNotifyUpstream"], true),
		NotifySystem:   anyBool(values["barkNotifySystem"]),
	}
}

func anyBoolDefault(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return anyBool(value)
}

func (s barkSettings) eventEnabled(category string) bool {
	if !s.Enabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "recharge_success":
		return s.NotifyRecharge
	case "upstream_alert", "upstream_recovery":
		return s.NotifyUpstream
	case "openai_image_alert", "openai_image_recovery":
		return s.NotifyError
	case "system":
		return s.NotifySystem
	default:
		return false
	}
}

func (s barkSettings) validate() error {
	if !s.Enabled {
		return nil
	}
	return s.validateConfigured()
}

func (s barkSettings) validateConfigured() error {
	if strings.TrimSpace(s.DeviceKey) == "" {
		return newAppError(http.StatusBadRequest, "Bark Device Key 未配置")
	}
	if _, err := s.endpoint(); err != nil {
		return err
	}
	return nil
}

func (s barkSettings) endpoint() (string, error) {
	server := strings.TrimSpace(s.ServerURL)
	if server == "" {
		server = defaultBarkServer
	}
	parsed, err := url.Parse(server)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return "", newAppError(http.StatusBadRequest, "Bark 服务地址不正确")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		path = "/push"
	} else if !strings.HasSuffix(path, "/push") {
		path += "/push"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String(), nil
}

func sendBarkNotification(ctx context.Context, settings barkSettings, title string, body string, actionURL string) error {
	if err := settings.validateConfigured(); err != nil {
		return err
	}
	endpoint, err := settings.endpoint()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(barkPushRequest{
		DeviceKey: strings.TrimSpace(settings.DeviceKey),
		Title:     strings.TrimSpace(title),
		Body:      body,
		Group:     strings.TrimSpace(settings.Group),
		Sound:     strings.TrimSpace(settings.Sound),
		Level:     "active",
		URL:       strings.TrimSpace(actionURL),
	})
	if err != nil {
		return fmt.Errorf("Bark 通知内容编码失败：%w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("Bark 请求创建失败：%w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: barkPushTimeout}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Bark 服务连接失败：%w", err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if readErr != nil {
		return fmt.Errorf("Bark 响应读取失败：%w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Bark 推送失败（HTTP %d）：%s", response.StatusCode, barkResponseMessage(responseBody))
	}
	var result barkPushResponse
	if json.Unmarshal(responseBody, &result) == nil && result.Code != nil && *result.Code != http.StatusOK {
		return fmt.Errorf("Bark 推送失败（code %d）：%s", *result.Code, strings.TrimSpace(result.Message))
	}
	return nil
}

func barkResponseMessage(value []byte) string {
	text := strings.TrimSpace(string(value))
	if text == "" {
		return "服务未返回错误信息"
	}
	var result barkPushResponse
	if json.Unmarshal(value, &result) == nil && strings.TrimSpace(result.Message) != "" {
		return strings.TrimSpace(result.Message)
	}
	if len(text) > 500 {
		return text[:500]
	}
	return text
}
