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
	fromName := settings.FromName
	if fromName == "" {
		fromName = settings.SiteName
	}
	fromName = displayBrandName(fromName)
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
	brand = displayBrandName(brand)
	actionHTML := ""
	copyLinkHTML := ""
	actionURL := strings.TrimSpace(action.URL)
	if actionURL != "" {
		actionText := strings.TrimSpace(action.Text)
		if actionText == "" {
			actionText = "立即查看"
		}
		escapedURL := html.EscapeString(actionURL)
		actionHTML = `<a href="` + escapedURL + `" style="display:inline-block;margin-top:24px;padding:12px 20px;border-radius:999px;background:#167947;color:#ffffff;text-decoration:none;font-weight:800;">` + html.EscapeString(actionText) + `</a>`
		copyLinkHTML = `<div style="margin-top:18px;padding:14px;border-radius:14px;background:#f3faf5;border:1px solid #d9eadf;">
                  <div style="margin-bottom:8px;color:#567064;font-size:12px;font-weight:700;">如果按钮无法打开，请复制以下链接到浏览器访问：</div>
                  <div style="color:#126238;font-size:13px;line-height:1.6;word-break:break-all;">` + escapedURL + `</div>
                </div>`
	}
	return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#f3f7f4;font-family:Arial,'Microsoft YaHei',sans-serif;color:#172033;">
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f3f7f4;padding:28px 12px;">
      <tr>
        <td align="center">
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:640px;background:#ffffff;border-radius:22px;overflow:hidden;border:1px solid #dfeee5;box-shadow:0 18px 48px rgba(21,91,54,.10);">
            <tr>
              <td style="padding:26px 28px 8px;background:linear-gradient(135deg,#f8fff9,#e7f7ed);">
                <div style="display:inline-block;padding:7px 12px;border-radius:999px;background:#dff5e8;color:#126238;font-size:12px;font-weight:800;letter-spacing:.04em;">` + html.EscapeString(brand) + ` 通知</div>
                <h1 style="margin:18px 0 0;color:#14231b;font-size:26px;line-height:1.28;font-weight:900;">` + html.EscapeString(subject) + `</h1>
              </td>
            </tr>
            <tr>
              <td style="padding:22px 28px 30px;">
                <div style="font-size:15px;line-height:1.9;white-space:pre-wrap;color:#26352d;">` + html.EscapeString(text) + `</div>
                ` + actionHTML + `
                ` + copyLinkHTML + `
                <div style="margin-top:30px;padding-top:16px;border-top:1px solid #e2eee7;color:#7a8980;font-size:12px;line-height:1.7;">这是一封来自 ` + html.EscapeString(brand) + ` 的服务通知邮件，请勿直接回复。</div>
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
