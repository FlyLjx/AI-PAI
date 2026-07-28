package httpserver

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type smtpSettings struct {
	Enabled     bool
	Host        string
	Port        int
	Secure      bool
	User        string
	Password    string
	FromName    string
	FromAddress string
	SiteName    string
}

type mailAction struct {
	Text string
	URL  string
}

const smtpDialTimeout = 15 * time.Second

func (s smtpSettings) validate() error {
	if !s.Enabled {
		return newAppError(400, "邮件服务未启用")
	}
	if s.Host == "" || s.User == "" || s.Password == "" {
		return newAppError(400, "邮件服务未配置完整")
	}
	return nil
}

func smtpSettingsFromMap(values map[string]any) smtpSettings {
	return smtpSettings{
		Enabled:     anyBool(values["emailEnabled"]),
		Host:        strings.TrimSpace(anyString(values["emailHost"])),
		Port:        anyInt(values["emailPort"], 465),
		Secure:      anyBool(values["emailSecure"]),
		User:        strings.TrimSpace(anyString(values["emailUser"])),
		Password:    anyString(values["emailPassword"]),
		FromName:    strings.TrimSpace(anyString(values["emailFromName"])),
		FromAddress: strings.TrimSpace(anyString(values["emailFromAddress"])),
		SiteName:    strings.TrimSpace(anyString(values["siteName"])),
	}
}

func displayBrandName(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return "AI-PAI"
	}
	normalized := strings.ReplaceAll(text, "AIπ", "AI-PAI")
	if strings.EqualFold(normalized, "ai-pai") || strings.EqualFold(normalized, "AI PAI") {
		return "AI-PAI"
	}
	return normalized
}

func emailBrandName(value string) string {
	brand := displayBrandName(value)
	if strings.EqualFold(brand, "AI-PAI") ||
		strings.Contains(brand, "AI生图站") ||
		strings.Contains(brand, "在线生图站") ||
		strings.Contains(brand, "AI 生图站") {
		return "AI-PAI API 中转站"
	}
	return brand
}

func mailDisplayFromName(settings smtpSettings) string {
	fromName := settings.FromName
	if strings.TrimSpace(fromName) == "" {
		fromName = settings.SiteName
	}
	return emailBrandName(fromName)
}

func sendSMTPMail(settings smtpSettings, to string, subject string, text string, actions ...mailAction) error {
	if err := settings.validate(); err != nil {
		return err
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return newAppError(400, "收件邮箱为空")
	}
	fromAddress := settings.FromAddress
	if fromAddress == "" {
		fromAddress = settings.User
	}
	fromName := mailDisplayFromName(settings)
	addr := net.JoinHostPort(settings.Host, strconv.Itoa(settings.Port))
	auth := smtp.PlainAuth("", settings.User, settings.Password, settings.Host)
	action := mailAction{}
	if len(actions) > 0 {
		action = actions[0]
	}
	message := buildMailMessage(fromName, fromAddress, to, subject, text, action)
	if settings.Secure {
		client, err := smtpDialTLS(settings, addr)
		if err != nil {
			return err
		}
		defer client.Close()
		if err := client.Auth(auth); err != nil {
			return newAppError(502, smtpAuthErrorMessage(settings, err))
		}
		return smtpSend(client, fromAddress, to, message)
	}
	client, err := smtpDialPlain(settings, addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.StartTLS(&tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12}); err == nil {
		if authErr := client.Auth(auth); authErr != nil {
			return newAppError(502, smtpAuthErrorMessage(settings, authErr))
		}
	} else if authErr := client.Auth(auth); authErr != nil {
		return newAppError(502, smtpAuthErrorMessage(settings, authErr))
	}
	return smtpSend(client, fromAddress, to, message)
}

func smtpDialTLS(settings smtpSettings, addr string) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: smtpDialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp4", addr, &tls.Config{ServerName: settings.Host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return nil, newAppError(502, "邮件服务器连接失败（"+addr+"，IPv4）："+err.Error())
	}
	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		_ = conn.Close()
		return nil, newAppError(502, "邮件客户端初始化失败："+err.Error())
	}
	return client, nil
}

func smtpDialPlain(settings smtpSettings, addr string) (*smtp.Client, error) {
	conn, err := net.DialTimeout("tcp4", addr, smtpDialTimeout)
	if err != nil {
		return nil, newAppError(502, "邮件服务器连接失败（"+addr+"，IPv4）："+err.Error())
	}
	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		_ = conn.Close()
		return nil, newAppError(502, "邮件客户端初始化失败："+err.Error())
	}
	return client, nil
}

func smtpAuthErrorMessage(settings smtpSettings, err error) string {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	host := strings.ToLower(settings.Host)
	if strings.Contains(detail, "535") || strings.Contains(host, "qq.com") {
		return "邮件登录失败：SMTP 账号或授权码不正确，或邮箱未开启 SMTP 服务。QQ 邮箱请在邮箱设置中开启 POP3/SMTP，并使用生成的“授权码”，不要填写 QQ 登录密码。原始错误：" + detail
	}
	return "邮件登录失败：" + detail
}

func smtpSend(client *smtp.Client, from string, to string, message []byte) error {
	if err := client.Mail(from); err != nil {
		return newAppError(502, "邮件发件人被拒绝："+err.Error())
	}
	if err := client.Rcpt(to); err != nil {
		return newAppError(502, "邮件收件人被拒绝："+err.Error())
	}
	writer, err := client.Data()
	if err != nil {
		return newAppError(502, "邮件内容发送失败："+err.Error())
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return newAppError(502, "邮件内容发送失败："+err.Error())
	}
	if err := writer.Close(); err != nil {
		return newAppError(502, "邮件发送失败："+err.Error())
	}
	return client.Quit()
}

func buildMailMessage(fromName string, fromAddress string, to string, subject string, text string, action mailAction) []byte {
	boundary := "aipi-mail-" + newID()
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)
	encodedFromName := mime.QEncoding.Encode("utf-8", fromName)
	htmlBody := buildMailHTML(fromName, subject, text, action)
	var buffer bytes.Buffer
	buffer.WriteString("From: " + encodedFromName + " <" + fromAddress + ">\r\n")
	buffer.WriteString("To: " + to + "\r\n")
	buffer.WriteString("Subject: " + encodedSubject + "\r\n")
	buffer.WriteString("MIME-Version: 1.0\r\n")
	buffer.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n")
	writeMailPart(&buffer, boundary, "text/plain; charset=utf-8", text)
	writeMailPart(&buffer, boundary, "text/html; charset=utf-8", htmlBody)
	buffer.WriteString("--" + boundary + "--\r\n")
	return buffer.Bytes()
}

func buildMailHTML(fromName string, subject string, text string, action mailAction) string {
	brand := strings.TrimSpace(fromName)
	brand = emailBrandName(brand)
	actionHTML := ""
	copyLinkHTML := ""
	actionURL := strings.TrimSpace(action.URL)
	if actionURL != "" {
		actionText := strings.TrimSpace(action.Text)
		if actionText == "" {
			actionText = "立即查看"
		}
		escapedURL := html.EscapeString(actionURL)
		actionHTML = `<a href="` + escapedURL + `" style="display:inline-block;margin-top:20px;padding:10px 16px;border-radius:6px;background:#047857;color:#ffffff;font-size:14px;line-height:1.4;text-decoration:none;font-weight:700;">` + html.EscapeString(actionText) + `</a>`
		copyLinkHTML = `<div style="margin-top:18px;color:#6b7280;font-size:12px;line-height:1.7;">
                  <div>按钮无法打开时，请复制此链接：</div>
                  <a href="` + escapedURL + `" style="color:#047857;text-decoration:none;word-break:break-all;">` + escapedURL + `</a>
                </div>`
	}
	return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#f5f6f5;font-family:Arial,'Microsoft YaHei',sans-serif;color:#1f2937;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f5f6f5;padding:24px 12px;">
      <tr>
        <td align="center">
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:600px;background:#ffffff;border:1px solid #e5e7eb;border-radius:8px;">
            <tr>
              <td style="padding:24px;">
                <div style="color:#047857;font-size:13px;line-height:1.4;font-weight:700;">` + html.EscapeString(brand) + `</div>
                <h1 style="margin:10px 0 0;color:#111827;font-size:20px;line-height:1.4;font-weight:700;">` + html.EscapeString(subject) + `</h1>
                <div style="margin-top:20px;color:#374151;font-size:14px;line-height:1.75;white-space:pre-wrap;">` + html.EscapeString(text) + `</div>
                ` + actionHTML + `
                ` + copyLinkHTML + `
				<div style="margin-top:28px;padding-top:14px;border-top:1px solid #e5e7eb;color:#9ca3af;font-size:11px;line-height:1.6;">此邮件由 ` + html.EscapeString(brand) + ` 自动发送，请勿直接回复。</div>
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`
}

func writeMailPart(buffer *bytes.Buffer, boundary string, contentType string, body string) {
	buffer.WriteString("--" + boundary + "\r\n")
	buffer.WriteString("Content-Type: " + contentType + "\r\n")
	buffer.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		buffer.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	buffer.WriteString(encoded + "\r\n\r\n")
}

func anyBool(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return strings.EqualFold(strings.TrimSpace(item), "true") || strings.TrimSpace(item) == "1"
	case float64:
		return item != 0
	case int:
		return item != 0
	default:
		return false
	}
}

func anyInt(value any, fallback int) int {
	switch item := value.(type) {
	case int:
		return item
	case float64:
		return int(item)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(item)); err == nil {
			return parsed
		}
	}
	return fallback
}

func anyFloat(value any, fallback float64) float64 {
	switch item := value.(type) {
	case float64:
		return item
	case int:
		return float64(item)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(item), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func formatMailFailure(email string, err error) map[string]string {
	message := "发送失败"
	if err != nil {
		message = err.Error()
	}
	return map[string]string{"email": email, "message": message}
}

func smtpSummary(total int, success int, failed int) string {
	return fmt.Sprintf("已处理 %d 个收件人，成功 %d 个，失败 %d 个", total, success, failed)
}
