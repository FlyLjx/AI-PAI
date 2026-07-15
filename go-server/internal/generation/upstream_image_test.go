package generation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aipi-go/internal/models"
	"aipi-go/internal/providers"
)

func TestCallImageJSONNormalizesURLOnlyResponse(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected endpoint path: %s", req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		if err := json.NewDecoder(req.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "https://cdn.example.test/out.png"}},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	result, err := service.callImageJSON(context.Background(), testImageRequest(server.URL), 1)
	if err != nil {
		t.Fatalf("callImageJSON returned error: %v", err)
	}
	if received["response_format"] != "url" {
		t.Fatalf("response_format should force url, got %#v", received["response_format"])
	}
	if received["quality"] != "high" {
		t.Fatalf("quality should default to high, got %#v", received["quality"])
	}
	payload, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected normalized result: %#v", result)
	}
	images, ok := payload["data"].([]ExtractedImage)
	if !ok {
		t.Fatalf("unexpected normalized images: %#v", payload["data"])
	}
	if len(images) != 1 || images[0].Type != "url" || images[0].URL != "https://cdn.example.test/out.png" {
		t.Fatalf("unexpected images: %#v", images)
	}
}

func TestCallImageJSONRewritesLocalhostImageURLToProviderOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "http://127.0.0.1/images/2026/06/14/out.png"}},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	result, err := service.callImageJSON(context.Background(), testImageRequest(server.URL), 1)
	if err != nil {
		t.Fatalf("callImageJSON returned error: %v", err)
	}
	images := ExtractImages(result)
	if len(images) != 1 {
		t.Fatalf("expected one image, got %#v", images)
	}
	want := server.URL + "/images/2026/06/14/out.png"
	if images[0].URL != want {
		t.Fatalf("expected rewritten image URL %q, got %q", want, images[0].URL)
	}
}

func TestCallImageJSONSendsEditImageDataFields(t *testing.T) {
	var received map[string]any
	sourcePNG, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lYV0ZQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("decode source png: %v", err)
	}
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(sourcePNG)
	}))
	defer sourceServer.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/images/edits" {
			t.Fatalf("unexpected endpoint path: %s", req.URL.Path)
		}
		if err := json.NewDecoder(req.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "https://cdn.example.test/edited.png"}},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	input := testImageRequest(server.URL)
	input.Operation = "edit"
	input.ReferenceImageURLs = []string{sourceServer.URL + "/source.png"}
	if _, err := service.callImageJSON(context.Background(), input, 1); err != nil {
		t.Fatalf("callImageJSON returned error: %v", err)
	}
	wantBase64 := base64.StdEncoding.EncodeToString(sourcePNG)
	wantDataURL := "data:image/png;base64," + wantBase64
	if received["image_url"] != wantDataURL {
		t.Fatalf("expected image_url field, got %#v", received["image_url"])
	}
	if received["image"] != wantBase64 {
		t.Fatalf("expected image field, got %#v", received["image"])
	}
	urls, ok := received["image_urls"].([]any)
	if !ok || len(urls) != 1 || urls[0] != wantDataURL {
		t.Fatalf("expected image_urls array, got %#v", received["image_urls"])
	}
	references, ok := received["referenceImages"].([]any)
	if !ok || len(references) != 1 {
		t.Fatalf("expected referenceImages array, got %#v", received["referenceImages"])
	}
	reference, ok := references[0].(map[string]any)
	if !ok || reference["url"] != wantDataURL {
		t.Fatalf("expected reference image data URL, got %#v", references[0])
	}
}

func TestCallImageJSONSendsEditMaskImageData(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := json.NewDecoder(req.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "https://cdn.example.test/edited.png"}},
		})
	}))
	defer server.Close()

	sourceBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lYV0ZQAAAABJRU5ErkJggg=="
	maskBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	service := &Service{logger: slog.Default()}
	input := testImageRequest(server.URL)
	input.Operation = "edit"
	input.ReferenceImageURLs = []string{"data:image/png;base64," + sourceBase64}
	input.MaskImageURL = "data:image/png;base64," + maskBase64
	if _, err := service.callImageJSON(context.Background(), input, 1); err != nil {
		t.Fatalf("callImageJSON returned error: %v", err)
	}
	if received["mask"] != maskBase64 {
		t.Fatalf("expected mask field, got %#v", received["mask"])
	}
	maskImage, ok := received["maskImage"].(map[string]any)
	if !ok || maskImage["url"] != "data:image/png;base64,"+maskBase64 {
		t.Fatalf("expected maskImage data URL, got %#v", received["maskImage"])
	}
}

func TestCallImageJSONReturnsUpstreamPolicyErrorWithoutFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "非常抱歉，生成的图片可能违反了我们的内容政策。 / invalid_request_error / content_policy_violation",
			},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	_, err := service.callImageJSON(context.Background(), testImageRequest(server.URL), 1)
	if err == nil {
		t.Fatal("expected upstream error")
	}
	if strings.Contains(err.Error(), "invalid_request_error") || strings.Contains(err.Error(), "content_policy_violation") {
		t.Fatalf("policy suffix should be hidden from user-facing error: %v", err)
	}
	if !strings.Contains(err.Error(), "内容政策") {
		t.Fatalf("expected cleaned policy message, got: %v", err)
	}
}

func TestCallImageJSONRetriesTransientCurl56Error(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "Failed to perform, curl: (56) Connection closed abruptly.",
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "https://cdn.example.test/retry-ok.png"}},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	result, err := service.callImageJSON(context.Background(), testImageRequest(server.URL), 1)
	if err != nil {
		t.Fatalf("callImageJSON returned error after retry: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected one retry, got %d requests", requestCount)
	}
	images := ExtractImages(result)
	if len(images) != 1 || images[0].URL != "https://cdn.example.test/retry-ok.png" {
		t.Fatalf("unexpected images after retry: %#v", images)
	}
}

func TestCallImageJSONRetriesHTMLGatewayTimeout(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)
		if count == 1 {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusGatewayTimeout)
			_, _ = w.Write([]byte("<html><body><h1>504 Gateway Time-out</h1></body></html>"))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "https://cdn.example.test/html-timeout-retry.png"}},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	result, err := service.callImageJSON(context.Background(), testImageRequest(server.URL), 1)
	if err != nil {
		t.Fatalf("callImageJSON returned error after HTML 504 retry: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected one retry after HTML 504, got %d requests", requestCount)
	}
	images := ExtractImages(result)
	if len(images) != 1 || images[0].URL != "https://cdn.example.test/html-timeout-retry.png" {
		t.Fatalf("unexpected images after HTML 504 retry: %#v", images)
	}
}

func TestCallImageJSONRejectsHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><title>New API</title>"))
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	_, err := service.callImageJSON(context.Background(), testImageRequest(server.URL), 1)
	if err == nil || !strings.Contains(err.Error(), "网页 HTML") {
		t.Fatalf("expected HTML response error, got: %v", err)
	}
}

func TestCallImageJSONConcurrentURLOnlyResponses(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		index := atomic.AddInt64(&requestCount, 1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": "https://cdn.example.test/out-" + string(rune('a'+index-1)) + ".png"}},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	const workers = 12
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := service.callImageJSON(context.Background(), testImageRequest(server.URL), 1)
			if err != nil {
				errs <- err
				return
			}
			payload, ok := result.(map[string]any)
			if !ok {
				errs <- errUnexpectedResult{}
				return
			}
			images, ok := payload["data"].([]ExtractedImage)
			if !ok || len(images) != 1 || images[0].Type != "url" || images[0].URL == "" {
				errs <- errUnexpectedResult{}
				return
			}
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if requestCount != workers {
		t.Fatalf("expected %d upstream requests, got %d", workers, requestCount)
	}
}

func TestCallImageGenerationSendsQuantityAsSingleNRequest(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		var received map[string]any
		if err := json.NewDecoder(req.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if received["n"] != float64(4) {
			t.Errorf("upstream request should pass n=4, got %#v", received["n"])
		}
		data := make([]map[string]string, 0, 4)
		for index := 0; index < 4; index++ {
			data = append(data, map[string]string{"url": fmt.Sprintf("https://cdn.example.test/batch-%d.png", index+1)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	input := testImageRequest(server.URL)
	input.Quantity = 4
	result, err := service.callImageGeneration(context.Background(), input)
	if err != nil {
		t.Fatalf("callImageGeneration returned error: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected 1 upstream request, got %d", requestCount)
	}
	images := ExtractImages(result)
	if len(images) != input.Quantity {
		t.Fatalf("expected %d aggregated images, got %#v", input.Quantity, images)
	}
	for _, image := range images {
		if image.Type != "url" || image.URL == "" || image.B64 != "" {
			t.Fatalf("expected url-only image result, got %#v", image)
		}
	}
}

func TestCallImageGenerationBatchesQuantityByUpstreamLimit(t *testing.T) {
	receivedN := []int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var received map[string]any
		if err := json.NewDecoder(req.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		n := int(received["n"].(float64))
		receivedN = append(receivedN, n)
		data := make([]map[string]string, 0, n)
		for index := 0; index < n; index++ {
			data = append(data, map[string]string{"url": fmt.Sprintf("https://cdn.example.test/batch-%d-%d.png", len(receivedN), index+1)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	input := testImageRequest(server.URL)
	input.Quantity = 10
	result, err := service.callImageGeneration(context.Background(), input)
	if err != nil {
		t.Fatalf("callImageGeneration returned error: %v", err)
	}
	wantN := []int{4, 4, 2}
	if fmt.Sprint(receivedN) != fmt.Sprint(wantN) {
		t.Fatalf("expected upstream n batches %v, got %v", wantN, receivedN)
	}
	images := ExtractImages(result)
	if len(images) != input.Quantity {
		t.Fatalf("expected %d aggregated images, got %#v", input.Quantity, images)
	}
}

func TestCallImageGenerationRequestsMissingImagesWhenBatchIsPartial(t *testing.T) {
	receivedN := []int{}
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)
		var received map[string]any
		if err := json.NewDecoder(req.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		receivedN = append(receivedN, int(received["n"].(float64)))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": fmt.Sprintf("https://cdn.example.test/partial-%d.png", count)}},
		})
	}))
	defer server.Close()

	service := &Service{logger: slog.Default()}
	input := testImageRequest(server.URL)
	input.Quantity = 4
	result, err := service.callImageGeneration(context.Background(), input)
	if err != nil {
		t.Fatalf("callImageGeneration returned error: %v", err)
	}
	wantN := []int{4, 3, 2, 1}
	if fmt.Sprint(receivedN) != fmt.Sprint(wantN) {
		t.Fatalf("expected upstream n batches %v, got %v", wantN, receivedN)
	}
	images := ExtractImages(result)
	if len(images) != input.Quantity {
		t.Fatalf("expected %d aggregated images, got %#v", input.Quantity, images)
	}
	if requestCount != int64(input.Quantity) {
		t.Fatalf("expected %d upstream requests, got %d", input.Quantity, requestCount)
	}
}

type errUnexpectedResult struct{}

func (e errUnexpectedResult) Error() string {
	return "unexpected normalized result"
}

func testImageRequest(baseURL string) ImageRequest {
	return ImageRequest{
		TaskID:     "test-task",
		Capability: "chat_image",
		Provider: providers.Provider{
			ID:      "provider-test",
			Type:    "newapi",
			BaseURL: baseURL,
			APIKey:  "test-key",
			Status:  "active",
		},
		Model: models.Model{
			ModelName:          "gpt-image-test",
			DisplayName:        "Test Image",
			AppendSizeToPrompt: true,
		},
		Prompt:       "测试",
		SizeTier:     "2k",
		Size:         "2048x2048",
		Quantity:     1,
		OutputFormat: "jpeg",
	}
}
