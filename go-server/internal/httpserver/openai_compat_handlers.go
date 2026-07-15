package httpserver

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/database"
	"aipi-go/internal/generation"
	"aipi-go/internal/models"
	"aipi-go/internal/tasks"
	"aipi-go/internal/users"
)

type compatImageInput struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	N              int      `json:"n"`
	Size           string   `json:"size"`
	AspectRatio    string   `json:"aspect_ratio"`
	Ratio          string   `json:"ratio"`
	SizeTier       string   `json:"size_tier"`
	Resolution     string   `json:"resolution"`
	Quality        string   `json:"quality"`
	ResponseFormat string   `json:"response_format"`
	Background     string   `json:"background"`
	OutputFormat   string   `json:"output_format"`
	ReferenceURLs  []string `json:"referenceImages"`
	Image          any      `json:"image"`
	ImageURL       any      `json:"image_url"`
	Mask           any      `json:"mask"`
}

const (
	compatTaskClientWaitTimeout = 8 * time.Minute
	compatTaskLogWaitTimeout    = 30 * time.Minute
)

type compatTaskResult struct {
	urls       []string
	message    string
	statusCode int
	err        error
}

func (r *Router) compatModels(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if _, err := r.authenticateAPIKey(req); err != nil {
		writeCompatAuthError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	items, err := models.NewRepository(r.db).FindAll(ctx)
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}
	data := []map[string]any{}
	for _, item := range uniqueCompatModels(items) {
		data = append(data, map[string]any{
			"id":       item.DisplayName,
			"object":   "model",
			"created":  item.CreatedAt.Unix(),
			"owned_by": "AI PAI",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   data,
		"meta": map[string]any{
			"total_count":  len(items),
			"unique_count": len(data),
		},
	})
}

func (r *Router) compatImageGenerations(w http.ResponseWriter, req *http.Request) {
	r.compatImageRequest(w, req, false)
}

func (r *Router) compatImageEdits(w http.ResponseWriter, req *http.Request) {
	r.compatImageRequest(w, req, true)
}

func (r *Router) compatImageRequest(w http.ResponseWriter, req *http.Request, isEdit bool) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	auth, err := r.authenticateAPIKey(req)
	if err != nil {
		writeCompatAuthError(w, err)
		return
	}
	var input compatImageInput
	if err := decodeCompatImageInput(req, &input, isEdit); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "请求参数不正确", "invalid_request_error")
		return
	}
	input.Model = strings.TrimSpace(input.Model)
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.N == 0 {
		input.N = 1
	}
	if input.Model == "" || input.Prompt == "" {
		writeOpenAIError(w, http.StatusBadRequest, "缺少模型或提示词", "invalid_request_error")
		return
	}
	if input.N < 1 || input.N > 10 {
		writeOpenAIError(w, http.StatusBadRequest, "生成数量必须在 1 到 10 之间", "invalid_request_error")
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	model, err := models.NewRepository(r.db).FindActiveByNameOrDisplayName(ctx, input.Model)
	if errors.Is(err, sql.ErrNoRows) || model == nil {
		writeOpenAIError(w, http.StatusNotFound, "模型不存在或已禁用", "invalid_request_error")
		return
	}
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, err.Error(), "api_error")
		return
	}
	size, sizeTier := resolveCompatSize(input, model.ModelName)
	if !sizeTierEnabled(model.EnabledSizeTiers, sizeTier) {
		writeOpenAIError(w, http.StatusBadRequest, "当前模型未开放 "+strings.ToUpper(sizeTier)+" 清晰度", "invalid_request_error")
		return
	}
	outputFormat := normalizeOutputFormat(input.OutputFormat)
	transparent := strings.EqualFold(input.Background, "transparent") || outputFormat == "png"
	referencePayload := compatReferencePayload(req, input.ReferenceURLs)
	if isEdit {
		referencePayload = compatEditReferencePayload(req, input)
		if referencePayload == nil {
			writeOpenAIError(w, http.StatusBadRequest, "图片编辑缺少参考图", "invalid_request_error")
			return
		}
	}
	accessLogID := newID()
	var savedTask *tasks.Task
	if err := r.withUserGenerationLock(ctx, auth.User.ID, func(tx *database.Tx) error {
		costCredits, err := r.generationBillingQuote(ctx, tx, auth.User.ID, *model, sizeTier, input.N)
		if err != nil {
			return err
		}
		task := tasks.Task{
			ID:                    newID(),
			UserID:                auth.User.ID,
			ModelID:               model.ID,
			ProviderID:            model.ProviderID,
			Capability:            model.Capability,
			Prompt:                input.Prompt,
			ReferenceImageURL:     referencePayload,
			SizeTier:              sizeTier,
			Size:                  &size,
			OutputFormat:          effectiveOutputFormat(outputFormat, transparent),
			TransparentBackground: transparent,
			Quantity:              input.N,
			UserIP:                requestIP(req),
			CostCredits:           costCredits,
			ModelCostCredits:      0,
			RemainingCredits:      0,
			DurationSeconds:       0,
			Status:                tasks.StatusQueued,
			PublicStatus:          "private",
		}
		savedTask, err = tasks.NewRepository(r.db).CreateWithTx(ctx, tx, task)
		if err != nil {
			return err
		}
		_, err = apiaccess.NewRepository(r.db).CreateLogWithTx(ctx, tx, apiaccess.UsageLog{
			ID:             accessLogID,
			UserID:         auth.User.ID,
			APIKeyID:       auth.APIKey.ID,
			TaskID:         &savedTask.ID,
			Endpoint:       req.URL.Path,
			Model:          input.Model,
			Prompt:         input.Prompt,
			Size:           size,
			Quality:        defaultString(input.Quality, sizeTier),
			Quantity:       input.N,
			ImageCount:     0,
			ResponseFormat: defaultString(input.ResponseFormat, "url"),
			Status:         "queued",
		})
		return err
	}); err != nil {
		status := http.StatusInternalServerError
		errorType := "api_error"
		message := err.Error()
		var appErr appError
		if errors.As(err, &appErr) {
			status = appErr.status
			errorType = "insufficient_quota"
			if status == http.StatusPaymentRequired {
				message = "用户 Key 额度不足"
				logCtx, logCancel := context.WithTimeout(context.Background(), 5*time.Second)
				errorMessage := message
				_, _ = apiaccess.NewRepository(r.db).CreateLog(logCtx, apiaccess.UsageLog{
					ID:             accessLogID,
					UserID:         auth.User.ID,
					APIKeyID:       auth.APIKey.ID,
					Endpoint:       req.URL.Path,
					Model:          input.Model,
					Prompt:         input.Prompt,
					Size:           size,
					Quality:        defaultString(input.Quality, sizeTier),
					Quantity:       input.N,
					ImageCount:     0,
					ResponseFormat: defaultString(input.ResponseFormat, "url"),
					Status:         "failed",
					ErrorMessage:   &errorMessage,
				})
				logCancel()
			}
		}
		writeOpenAIError(w, status, message, errorType)
		return
	}
	r.queue.EnqueueScoped(savedTask.ID, generation.APIKeyConcurrencyScope(auth.APIKey.ID), auth.APIKey.ConcurrencyLimit)

	resultCh := make(chan compatTaskResult, 1)
	go func() {
		resultCh <- r.finalizeCompatTaskLog(accessLogID, savedTask.ID)
	}()

	var result compatTaskResult
	select {
	case result = <-resultCh:
	case <-req.Context().Done():
		return
	case <-time.After(compatTaskClientWaitTimeout):
		writeOpenAIError(w, http.StatusGatewayTimeout, "图片生成超时", "api_error")
		return
	}

	if result.err != nil {
		status := result.statusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		message := result.message
		if message == "" {
			message = result.err.Error()
		}
		writeOpenAIError(w, status, message, "api_error")
		return
	}

	data := make([]map[string]string, 0, len(result.urls))
	for _, url := range result.urls {
		if strings.HasPrefix(url, "/") {
			url = absoluteURL(req, url)
		}
		data = append(data, map[string]string{"url": url})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	})
}

func (r *Router) finalizeCompatTaskLog(accessLogID string, taskID string) compatTaskResult {
	ctx, cancel := context.WithTimeout(context.Background(), compatTaskLogWaitTimeout)
	defer cancel()
	finalTask, err := r.waitForCompatTask(ctx, taskID)
	if err != nil {
		message := compatTaskWaitErrorMessage(err)
		r.finishCompatAccessLog(accessLogID, "failed", 0, message)
		return compatTaskResult{
			message:    message,
			statusCode: http.StatusGatewayTimeout,
			err:        err,
		}
	}
	if finalTask.Status != tasks.StatusSuccess {
		message := "图片生成失败"
		if finalTask.ErrorMessage != nil && *finalTask.ErrorMessage != "" {
			message = *finalTask.ErrorMessage
		}
		r.finishCompatAccessLog(accessLogID, "failed", 0, message)
		return compatTaskResult{
			message:    message,
			statusCode: http.StatusInternalServerError,
			err:        errors.New(message),
		}
	}
	urls := tasks.ToPublic(finalTask).ResultURLs
	r.finishCompatAccessLog(accessLogID, "success", len(urls), "")
	return compatTaskResult{urls: urls}
}

func (r *Router) finishCompatAccessLog(accessLogID string, status string, imageCount int, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = apiaccess.NewRepository(r.db).FinishLog(ctx, accessLogID, status, imageCount, message)
}

func compatTaskWaitErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "图片生成超时"
	}
	return err.Error()
}

func (r *Router) authenticateAPIKey(req *http.Request) (*apiaccess.Authenticated, error) {
	token := bearerToken(req)
	ctx, cancel := context.WithTimeout(req.Context(), 8*time.Second)
	defer cancel()
	return apiaccess.NewService(apiaccess.NewRepository(r.db), users.NewRepository(r.db)).Authenticate(ctx, token)
}

func (r *Router) waitForCompatTask(ctx context.Context, id string) (*tasks.Task, error) {
	ticker := time.NewTicker(1200 * time.Millisecond)
	defer ticker.Stop()
	repo := tasks.NewRepository(r.db)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task, err := repo.FindByID(context.Background(), id)
			if err != nil {
				return nil, err
			}
			if task.Status == tasks.StatusSuccess || task.Status == tasks.StatusFailed || task.Status == tasks.StatusCanceled {
				return task, nil
			}
		}
	}
}

func writeCompatAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, apiaccess.ErrMissingKey) || errors.Is(err, apiaccess.ErrInvalidKey) {
		writeOpenAIError(w, http.StatusUnauthorized, err.Error(), "invalid_api_key")
		return
	}
	writeOpenAIError(w, http.StatusUnauthorized, err.Error(), "invalid_api_key")
}

func writeOpenAIError(w http.ResponseWriter, status int, message string, errorType string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType,
			"param":   nil,
			"code":    nil,
		},
	})
}

func uniqueCompatModels(items []models.Model) []models.Model {
	seen := map[string]bool{}
	result := []models.Model{}
	for _, item := range items {
		if item.Status != "active" || item.ProviderStatus == nil || *item.ProviderStatus != "active" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(item.DisplayName))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func resolveCompatSize(input compatImageInput, modelName string) (string, string) {
	tier := normalizeSizeTier(input.SizeTier)
	if tier == "" {
		tier = normalizeSizeTier(input.Resolution)
	}
	if tier == "" {
		tier = normalizeSizeTier(input.Quality)
	}
	if input.Size != "" {
		if tier == "" {
			tier = sizeToTier(input.Size)
		}
		return input.Size, tier
	}
	ratio := normalizeRatio(input.AspectRatio)
	if ratio == "" {
		ratio = normalizeRatio(input.Ratio)
	}
	if ratio == "" {
		ratio = ratioFromModelName(modelName)
	}
	if ratio == "" {
		ratio = "1:1"
	}
	if tier == "" {
		tier = tierFromModelName(modelName)
	}
	if tier == "" {
		tier = "1k"
	}
	return compatSizeForRatio(ratio, tier), tier
}

func normalizeSizeTier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "1k" || value == "2k" || value == "4k" {
		return value
	}
	return ""
}

func sizeToTier(size string) string {
	parts := strings.Split(strings.ToLower(size), "x")
	maxSide := 0
	for _, part := range parts {
		n, _ := strconv.Atoi(strings.TrimSpace(part))
		if n > maxSide {
			maxSide = n
		}
	}
	if maxSide >= 3000 {
		return "4k"
	}
	if maxSide >= 2000 {
		return "2k"
	}
	return "1k"
}

func normalizeRatio(value string) string {
	value = strings.NewReplacer("×", ":", "x", ":", "X", ":", "*", ":").Replace(strings.TrimSpace(value))
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return ""
	}
	left, errLeft := strconv.Atoi(strings.TrimSpace(parts[0]))
	right, errRight := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errLeft != nil || errRight != nil || left <= 0 || right <= 0 {
		return ""
	}
	return strconv.Itoa(left) + ":" + strconv.Itoa(right)
}

func tierFromModelName(value string) string {
	lower := strings.ToLower(value)
	for _, tier := range []string{"4k", "2k", "1k"} {
		if strings.Contains(lower, "-"+tier) || strings.Contains(lower, "_"+tier) || strings.Contains(lower, " "+tier) {
			return tier
		}
	}
	return ""
}

func ratioFromModelName(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i := len(parts) - 1; i >= 0; i-- {
		if ratio := normalizeRatio(parts[i]); ratio != "" {
			return ratio
		}
	}
	return ""
}

func compatSizeForRatio(ratio string, tier string) string {
	sizes := map[string]map[string]string{
		"1:1":  {"1k": "1024x1024", "2k": "2048x2048", "4k": "3072x3072"},
		"16:9": {"1k": "1536x864", "2k": "2048x1152", "4k": "3072x1728"},
		"9:16": {"1k": "864x1536", "2k": "1152x2048", "4k": "1728x3072"},
		"4:3":  {"1k": "1536x1152", "2k": "2048x1536", "4k": "3072x2304"},
		"3:4":  {"1k": "1152x1536", "2k": "1536x2048", "4k": "2304x3072"},
		"3:2":  {"1k": "1536x1024", "2k": "2048x1360", "4k": "3072x2048"},
		"2:3":  {"1k": "1024x1536", "2k": "1360x2048", "4k": "2048x3072"},
	}
	if sizes[ratio] == nil || sizes[ratio][tier] == "" {
		return sizes["1:1"][tier]
	}
	return sizes[ratio][tier]
}

func normalizeOutputFormat(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "jpg" {
		return "jpeg"
	}
	if value == "jpeg" || value == "png" || value == "webp" {
		return value
	}
	return "jpeg"
}

func compatReferencePayload(req *http.Request, urls []string) *string {
	cleaned := []string{}
	for _, url := range urls {
		if strings.TrimSpace(url) != "" {
			cleaned = appendUniqueReferencePayload(cleaned, compatReferenceValue(req, strings.TrimSpace(url)))
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	value := strings.Join(cleaned, "\n")
	return &value
}

func compatEditReferencePayload(req *http.Request, input compatImageInput) *string {
	items := extractCompatImageURLs(input.ImageURL)
	if len(items) == 0 {
		items = extractCompatImageURLs(input.Image)
	}
	if len(input.ReferenceURLs) > 0 {
		items = append(items, input.ReferenceURLs...)
	}
	mask := firstCompatImageURL(input.Mask)
	cleaned := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			cleaned = appendUniqueReferencePayload(cleaned, compatReferenceValue(req, item))
		}
	}
	if strings.TrimSpace(mask) != "" {
		cleaned = appendUniqueReferencePayload(cleaned, "mask:"+compatReferenceValue(req, strings.TrimSpace(mask)))
	}
	if len(cleaned) == 0 {
		return nil
	}
	bytes, _ := json.Marshal(cleaned)
	value := string(bytes)
	return &value
}

func extractCompatImageURLs(value any) []string {
	if value == nil {
		return nil
	}
	switch item := value.(type) {
	case string:
		if strings.TrimSpace(item) == "" {
			return nil
		}
		return []string{strings.TrimSpace(item)}
	case []any:
		result := []string{}
		for _, child := range item {
			result = append(result, extractCompatImageURLs(child)...)
		}
		return result
	case map[string]any:
		for _, key := range []string{"url", "image_url", "imageUrl", "b64_json", "base64"} {
			if result := extractCompatImageURLs(item[key]); len(result) > 0 {
				return result
			}
		}
		if nested, ok := item["image_url"].(map[string]any); ok {
			return extractCompatImageURLs(nested["url"])
		}
	}
	return nil
}

func firstCompatImageURL(value any) string {
	items := extractCompatImageURLs(value)
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func decodeCompatImageInput(req *http.Request, target *compatImageInput, isEdit bool) error {
	contentType := strings.ToLower(req.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "multipart/form-data") {
		return decodeCompatJSON(req, target)
	}
	if err := req.ParseMultipartForm(64 << 20); err != nil {
		return err
	}
	target.Model = req.FormValue("model")
	target.Prompt = req.FormValue("prompt")
	target.Size = req.FormValue("size")
	target.AspectRatio = req.FormValue("aspect_ratio")
	target.Ratio = req.FormValue("ratio")
	target.SizeTier = req.FormValue("size_tier")
	target.Resolution = req.FormValue("resolution")
	target.Quality = req.FormValue("quality")
	target.ResponseFormat = req.FormValue("response_format")
	target.Background = req.FormValue("background")
	target.OutputFormat = req.FormValue("output_format")
	target.N = compatFormInt(req.FormValue("n"), 0)
	if !isEdit || req.MultipartForm == nil {
		return nil
	}
	for _, field := range []string{"image", "images"} {
		for _, header := range req.MultipartForm.File[field] {
			dataURL, err := multipartImageDataURL(header)
			if err != nil {
				return err
			}
			target.ReferenceURLs = append(target.ReferenceURLs, dataURL)
		}
	}
	if headers := req.MultipartForm.File["mask"]; len(headers) > 0 {
		dataURL, err := multipartImageDataURL(headers[0])
		if err != nil {
			return err
		}
		target.Mask = dataURL
	}
	return nil
}

func compatFormInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func multipartImageDataURL(header *multipart.FileHeader) (string, error) {
	file, err := header.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 20*1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(body) > 20*1024*1024 {
		return "", errors.New("参考图超过 20MB")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	if !strings.HasPrefix(contentType, "image/") {
		contentType = strings.ToLower(http.DetectContentType(body))
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", errors.New("上传文件不是图片")
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func compatReferenceValue(req *http.Request, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "data:") {
		return value
	}
	if looksLikeBase64Image(value) {
		return value
	}
	return absoluteURL(req, value)
}

func looksLikeBase64Image(value string) bool {
	compact := strings.NewReplacer("\r", "", "\n", "", "\t", "", " ", "").Replace(strings.TrimSpace(value))
	if len(compact) < 32 || strings.Contains(compact, "://") || strings.Contains(compact, ",") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(compact)
	if err != nil || len(decoded) < 8 {
		return false
	}
	contentType := http.DetectContentType(decoded)
	return strings.HasPrefix(strings.ToLower(contentType), "image/")
}

func decodeCompatJSON(req *http.Request, target any) error {
	return json.NewDecoder(req.Body).Decode(target)
}

func absoluteURL(req *http.Request, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "data:") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	origin := requestOrigin(req)
	if origin == "" {
		return path
	}
	return strings.TrimRight(origin, "/") + path
}
