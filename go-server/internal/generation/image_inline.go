package generation

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxInlineEditImageBytes = 20 * 1024 * 1024

type inlineEditImage struct {
	Original    string
	DataURL     string
	Base64      string
	ContentType string
}

func inlineEditImageData(ctx context.Context, value string) (inlineEditImage, error) {
	original := strings.TrimSpace(value)
	if original == "" {
		return inlineEditImage{}, errors.New("图片地址为空")
	}
	if strings.HasPrefix(strings.ToLower(original), "data:") {
		return inlineImageDataURL(original)
	}
	if image, ok := inlineRawBase64Image(original); ok {
		image.Original = original
		return image, nil
	}
	if !strings.HasPrefix(original, "http://") && !strings.HasPrefix(original, "https://") {
		return inlineEditImage{}, fmt.Errorf("参考图地址不正确：%s", trimLong(original, 80))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, original, nil)
	if err != nil {
		return inlineEditImage{}, fmt.Errorf("参考图地址不正确：%w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return inlineEditImage{}, fmt.Errorf("参考图读取失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return inlineEditImage{}, fmt.Errorf("参考图读取失败：HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxInlineEditImageBytes+1))
	if err != nil {
		return inlineEditImage{}, fmt.Errorf("参考图读取失败：%w", err)
	}
	if len(body) > maxInlineEditImageBytes {
		return inlineEditImage{}, errors.New("参考图超过 20MB，请换一张更小的图片")
	}
	contentType := imageContentType(resp.Header.Get("Content-Type"), body)
	if !strings.HasPrefix(contentType, "image/") {
		return inlineEditImage{}, errors.New("参考图读取失败：返回内容不是图片")
	}
	encoded := base64.StdEncoding.EncodeToString(body)
	return inlineEditImage{
		Original:    original,
		DataURL:     "data:" + contentType + ";base64," + encoded,
		Base64:      encoded,
		ContentType: contentType,
	}, nil
}

func inlineImageDataURL(value string) (inlineEditImage, error) {
	header, payload, ok := strings.Cut(strings.TrimSpace(value), ",")
	if !ok {
		return inlineEditImage{}, errors.New("图片 data URL 格式不正确")
	}
	header = strings.ToLower(strings.TrimSpace(header))
	if !strings.HasPrefix(header, "data:image/") || !strings.Contains(header, ";base64") {
		return inlineEditImage{}, errors.New("图片 data URL 必须是 image/* base64")
	}
	contentType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
	encoded := compactBase64(payload)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return inlineEditImage{}, errors.New("图片 base64 数据不正确")
	}
	if len(decoded) > maxInlineEditImageBytes {
		return inlineEditImage{}, errors.New("参考图超过 20MB，请换一张更小的图片")
	}
	return inlineEditImage{
		Original:    value,
		DataURL:     "data:" + contentType + ";base64," + encoded,
		Base64:      encoded,
		ContentType: contentType,
	}, nil
}

func inlineRawBase64Image(value string) (inlineEditImage, bool) {
	encoded := compactBase64(value)
	if len(encoded) < 24 || strings.Contains(encoded, "://") || strings.Contains(encoded, ",") {
		return inlineEditImage{}, false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) == 0 || len(decoded) > maxInlineEditImageBytes {
		return inlineEditImage{}, false
	}
	contentType := imageContentType("", decoded)
	if !strings.HasPrefix(contentType, "image/") {
		return inlineEditImage{}, false
	}
	return inlineEditImage{
		DataURL:     "data:" + contentType + ";base64," + encoded,
		Base64:      encoded,
		ContentType: contentType,
	}, true
}

func compactBase64(value string) string {
	replacer := strings.NewReplacer("\r", "", "\n", "", "\t", "", " ", "")
	return replacer.Replace(strings.TrimSpace(value))
}

func imageContentType(header string, body []byte) string {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(header, ";")[0]))
	if strings.HasPrefix(contentType, "image/") {
		return contentType
	}
	return strings.ToLower(http.DetectContentType(body))
}
