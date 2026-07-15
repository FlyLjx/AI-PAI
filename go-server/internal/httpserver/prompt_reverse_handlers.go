package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"aipi-go/internal/models"
	"aipi-go/internal/providers"
	"aipi-go/internal/users"
)

func (r *Router) promptReverse(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var input struct {
		UserID   string `json:"userId"`
		ModelID  string `json:"modelId"`
		ImageURL string `json:"imageUrl"`
		Language string `json:"language"`
	}
	if err := decodeCompatJSON(req, &input); err != nil {
		writeError(w, newAppError(http.StatusBadRequest, "请求参数不正确"))
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 90*time.Second)
	defer cancel()
	user, err := users.NewRepository(r.db).FindByID(ctx, strings.TrimSpace(input.UserID))
	if err != nil || user.Status != "active" {
		writeError(w, newAppError(http.StatusNotFound, "用户不存在或已禁用"))
		return
	}
	model, err := models.NewRepository(r.db).FindByID(ctx, strings.TrimSpace(input.ModelID))
	if err != nil || model.Status != "active" {
		writeError(w, newAppError(http.StatusNotFound, "模型不存在或已禁用"))
		return
	}
	provider, err := providers.NewRepository(r.db).FindByID(ctx, model.ProviderID)
	if err != nil || provider.Status != "active" {
		writeError(w, newAppError(http.StatusBadRequest, "接口配置不存在或已禁用"))
		return
	}
	dataURL, err := readImageAsDataURL(ctx, strings.TrimSpace(input.ImageURL))
	if err != nil {
		writeError(w, err)
		return
	}
	language := "zh"
	if input.Language == "en" {
		language = "en"
	}
	systemPrompt := "你是专业的图片提示词反推助手。请分析用户上传的图片，并输出一段适合 AI 生图模型使用的中文提示词。只输出提示词正文，不要解释。"
	userText := "请把这张图片反推出一段高质量生图提示词。"
	if language == "en" {
		systemPrompt = "You are an image prompt reverse-engineering assistant. Return only one polished image-generation prompt."
		userText = "Reverse this image into a detailed prompt for image generation."
	}
	reverseModel := strings.TrimSpace(os.Getenv("PROMPT_REVERSE_MODEL"))
	if reverseModel == "" {
		reverseModel = model.ModelName
	}
	body := map[string]any{
		"model": reverseModel,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": []map[string]any{
				{"type": "text", "text": userText},
				{"type": "image_url", "image_url": map[string]any{"url": dataURL}},
			}},
		},
		"temperature": 0.35,
		"stream":      false,
	}
	result, _, err := postOpenAIJSON(ctx, *provider, "chat/completions", body)
	if err != nil {
		writeError(w, newAppError(upstreamStatus(err), "上游提示词反推失败："+err.Error()))
		return
	}
	var payload any
	_ = json.Unmarshal(result, &payload)
	prompt := strings.TrimSpace(extractText(payload))
	if prompt == "" {
		writeError(w, newAppError(http.StatusBadGateway, "上游未返回有效提示词"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"prompt":            prompt,
		"modelId":           model.ID,
		"modelName":         reverseModel,
		"providerModelName": model.DisplayName,
	}})
}

func readImageAsDataURL(ctx context.Context, imageURL string) (string, error) {
	if strings.HasPrefix(imageURL, "data:image/") {
		return imageURL, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", newAppError(http.StatusBadRequest, "图片地址不正确")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", newAppError(http.StatusBadGateway, "图片读取失败："+err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", newAppError(resp.StatusCode, "图片读取失败")
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "image/png"
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 12*1024*1024))
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}
