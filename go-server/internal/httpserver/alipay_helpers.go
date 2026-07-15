package httpserver

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type alipaySettings struct {
	AppID      string
	PrivateKey string
	PublicKey  string
	Gateway    string
	BackendURL string
	SiteName   string
}

type alipayPayment struct {
	QRCode string
}

type alipayQueryResult struct {
	Paid        bool
	TradeNo     string
	TradeStatus string
}

func alipaySettingsFromMap(values map[string]any) alipaySettings {
	return alipaySettings{
		AppID:      strings.TrimSpace(anyString(values["alipayAppId"])),
		PrivateKey: strings.TrimSpace(anyString(values["alipayPrivateKey"])),
		PublicKey:  strings.TrimSpace(anyString(values["alipayPublicKey"])),
		Gateway:    defaultString(strings.TrimSpace(anyString(values["alipayGateway"])), "https://openapi.alipay.com/gateway.do"),
		BackendURL: strings.TrimSpace(anyString(values["backendUrl"])),
		SiteName:   strings.TrimSpace(anyString(values["siteName"])),
	}
}

func (s alipaySettings) validate() error {
	if s.AppID == "" || s.PrivateKey == "" || s.PublicKey == "" {
		return newAppError(http.StatusBadRequest, "支付宝当面付未配置完整")
	}
	if s.Gateway == "" {
		return newAppError(http.StatusBadRequest, "支付宝网关未配置")
	}
	return nil
}

func createAlipayPrecreateOrder(ctx context.Context, settings alipaySettings, outTradeNo string, amount float64, subject string, notifyOrigin string) (alipayPayment, error) {
	if err := settings.validate(); err != nil {
		return alipayPayment{}, err
	}
	notifyURL := buildAlipayNotifyURL(settings, notifyOrigin)
	bizContent, _ := json.Marshal(map[string]string{
		"out_trade_no": outTradeNo,
		"total_amount": fmt.Sprintf("%.2f", amount),
		"subject":      defaultString(subject, displayBrandName(settings.SiteName)+"订阅"),
	})
	payload, err := callAlipay(ctx, settings, "alipay.trade.precreate", notifyURL, string(bizContent))
	if err != nil {
		return alipayPayment{}, err
	}
	response := nestedMap(payload, "alipay_trade_precreate_response")
	if response == nil {
		return alipayPayment{}, newAppError(http.StatusBadGateway, "支付宝预创建订单响应异常")
	}
	if fmt.Sprint(response["code"]) != "10000" {
		return alipayPayment{}, newAppError(http.StatusBadGateway, alipayErrorMessage(response, "支付宝预创建订单失败"))
	}
	qrCode := strings.TrimSpace(fmt.Sprint(response["qr_code"]))
	if qrCode == "" || qrCode == "<nil>" {
		return alipayPayment{}, newAppError(http.StatusBadGateway, "支付宝预创建订单未返回二维码")
	}
	return alipayPayment{QRCode: qrCode}, nil
}

func queryAlipayOrder(ctx context.Context, settings alipaySettings, outTradeNo string) (alipayQueryResult, error) {
	if err := settings.validate(); err != nil {
		return alipayQueryResult{}, err
	}
	bizContent, _ := json.Marshal(map[string]string{"out_trade_no": strings.TrimSpace(outTradeNo)})
	payload, err := callAlipay(ctx, settings, "alipay.trade.query", "", string(bizContent))
	if err != nil {
		return alipayQueryResult{}, err
	}
	response := nestedMap(payload, "alipay_trade_query_response")
	if response == nil {
		return alipayQueryResult{}, newAppError(http.StatusBadGateway, "支付宝订单查询响应异常")
	}
	if fmt.Sprint(response["code"]) != "10000" {
		return alipayQueryResult{TradeStatus: alipayErrorMessage(response, "支付宝订单未支付")}, nil
	}
	status := strings.TrimSpace(fmt.Sprint(response["trade_status"]))
	return alipayQueryResult{
		Paid:        status == "TRADE_SUCCESS" || status == "TRADE_FINISHED",
		TradeNo:     strings.TrimSpace(fmt.Sprint(response["trade_no"])),
		TradeStatus: status,
	}, nil
}

func verifyAlipayNotify(settings alipaySettings, values url.Values) error {
	if err := settings.validate(); err != nil {
		return err
	}
	sign := strings.TrimSpace(values.Get("sign"))
	if sign == "" {
		return newAppError(http.StatusBadRequest, "支付宝通知缺少签名")
	}
	publicKey, err := parseAlipayPublicKey(settings.PublicKey)
	if err != nil {
		return newAppError(http.StatusBadRequest, "支付宝公钥解析失败："+err.Error())
	}
	signature, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return newAppError(http.StatusBadRequest, "支付宝通知签名格式不正确")
	}
	signed := alipayCanonicalString(values)
	digest := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return newAppError(http.StatusBadRequest, "支付宝通知验签失败")
	}
	return nil
}

func callAlipay(ctx context.Context, settings alipaySettings, method string, notifyURL string, bizContent string) (map[string]any, error) {
	params := url.Values{}
	params.Set("app_id", settings.AppID)
	params.Set("method", method)
	params.Set("charset", "utf-8")
	params.Set("sign_type", "RSA2")
	params.Set("timestamp", time.Now().Format("2006-01-02 15:04:05"))
	params.Set("version", "1.0")
	params.Set("biz_content", bizContent)
	if notifyURL != "" {
		params.Set("notify_url", notifyURL)
	}
	sign, err := signAlipayParams(settings.PrivateKey, params)
	if err != nil {
		return nil, newAppError(http.StatusBadRequest, "支付宝应用私钥解析失败："+err.Error())
	}
	params.Set("sign", sign)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.Gateway, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, newAppError(http.StatusBadGateway, "支付宝网关连接失败："+err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newAppError(http.StatusBadGateway, fmt.Sprintf("支付宝网关调用失败：%d", resp.StatusCode))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, newAppError(http.StatusBadGateway, "支付宝网关返回非 JSON")
	}
	return payload, nil
}

func signAlipayParams(privateKeyText string, params url.Values) (string, error) {
	privateKey, err := parseAlipayPrivateKey(privateKeyText)
	if err != nil {
		return "", err
	}
	signed := alipayCanonicalString(params)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func alipayCanonicalString(params url.Values) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key == "sign" {
			continue
		}
		if len(params[key]) == 0 || strings.TrimSpace(params[key][0]) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params.Get(key))
	}
	return strings.Join(parts, "&")
}

func parseAlipayPrivateKey(value string) (*rsa.PrivateKey, error) {
	block, err := pemOrBase64Block(value, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		if parsed, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes); parseErr == nil {
			return parsed, nil
		}
		return nil, err
	}
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("不是 RSA 私钥")
	}
	return privateKey, nil
}

func parseAlipayPublicKey(value string) (*rsa.PublicKey, error) {
	block, err := pemOrBase64Block(value, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if cert, certErr := x509.ParseCertificate(block.Bytes); certErr == nil {
			if publicKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
				return publicKey, nil
			}
		}
		return nil, err
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("不是 RSA 公钥")
	}
	return publicKey, nil
}

func pemOrBase64Block(value string, label string) (*pem.Block, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil, errors.New("内容为空")
	}
	if block, _ := pem.Decode([]byte(text)); block != nil {
		return block, nil
	}
	cleaned := strings.NewReplacer("\r", "", "\n", "", " ", "").Replace(text)
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, err
	}
	return &pem.Block{Type: label, Bytes: decoded}, nil
}

func buildAlipayNotifyURL(settings alipaySettings, origin string) string {
	base := strings.TrimRight(strings.TrimSpace(origin), "/")
	if base == "" {
		base = strings.TrimRight(settings.BackendURL, "/")
	}
	if base == "" {
		base = "http://localhost:3001"
	}
	return base + "/api/recharge/alipay/notify"
}

func requestOrigin(req *http.Request) string {
	proto := firstForwardedHeaderValue(req.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if req.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := firstForwardedHeaderValue(req.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = req.Host
	}
	if host == "" {
		return ""
	}
	return proto + "://" + host
}

func firstForwardedHeaderValue(value string) string {
	return strings.TrimSpace(strings.Split(value, ",")[0])
}

func nestedMap(payload map[string]any, key string) map[string]any {
	value, ok := payload[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func alipayErrorMessage(response map[string]any, fallback string) string {
	for _, key := range []string{"sub_msg", "subMsg", "msg"} {
		value := strings.TrimSpace(fmt.Sprint(response[key]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return fallback
}
