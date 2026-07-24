package httpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aipi-go/internal/tasks"
)

func TestCompatChatPromptUsesLatestUserText(t *testing.T) {
	prompt := compatChatPrompt([]any{
		map[string]any{"role": "user", "content": "first prompt"},
		map[string]any{"role": "assistant", "content": "old response"},
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": "a cute cat"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.test/ref.png"}},
		}},
	})
	if prompt != "a cute cat" {
		t.Fatalf("prompt = %q, want latest user text", prompt)
	}
}

func TestRouterRegistersChatImageCompatibilityEndpoint(t *testing.T) {
	router := &Router{mux: http.NewServeMux()}
	router.routes()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://example.test/v1/chat/completions", nil)

	router.mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d; route may not be registered", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func TestWriteCompatImageChatCompletionReturnsNewAPICompatibleJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCompatImageChatCompletion(recorder, compatImageInput{Model: "gpt-image-2"}, []map[string]string{{
		"url": "https://aipi.example.test/api/tasks/task-1/images/0",
	}}, 1784430000, "chatcmpl-log1")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage map[string]int `json:"usage"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Object != "chat.completion" || response.Created != 1784430000 || response.Model != "gpt-image-2" {
		t.Fatalf("unexpected response metadata: %+v", response)
	}
	if len(response.Choices) != 1 || response.Choices[0].Message.Role != "assistant" || response.Choices[0].FinishReason != "stop" {
		t.Fatalf("unexpected choices: %+v", response.Choices)
	}
	if response.Choices[0].Message.Content != "![image](https://aipi.example.test/api/tasks/task-1/images/0)" {
		t.Fatalf("unexpected content: %q", response.Choices[0].Message.Content)
	}
	if response.Usage == nil {
		t.Fatal("usage is required by new-api channel tests")
	}
}

func TestWriteCompatImageSuccessKeepsOpenAIImagesResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://aipi.example.test/v1/images/generations", nil)
	writeCompatImageSuccess(recorder, request, compatImageInput{Model: "gpt-image-2"}, []string{
		"/api/tasks/task-1/images/0",
	}, compatImageResponseOpenAI, "log-1")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Created int64 `json:"created"`
		Data    []struct {
			URL string `json:"url"`
		} `json:"data"`
		Choices json.RawMessage `json:"choices"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Created <= 0 || len(response.Data) != 1 || response.Data[0].URL != "https://aipi.example.test/api/tasks/task-1/images/0" {
		t.Fatalf("unexpected image response: %+v", response)
	}
	if len(response.Choices) != 0 {
		t.Fatalf("standard image response must not include chat choices: %s", response.Choices)
	}
}

func TestCompatResultURLsForAPIPrefersUpstreamDirectURL(t *testing.T) {
	providerBaseURL := "https://upstream.example.test/v1"
	finalTask := &tasks.Task{
		ID:              "task-1",
		Status:          tasks.StatusSuccess,
		ProviderBaseURL: &providerBaseURL,
		ResultJSON: map[string]any{
			"data": []any{map[string]any{"url": "http://127.0.0.1:6001/generated/out.png"}},
		},
		CreatedAt: time.Unix(1784430000, 0),
		UpdatedAt: time.Unix(1784430000, 0),
	}

	urls := compatResultURLsForAPI(finalTask, false, false)
	if len(urls) != 1 || urls[0] != "https://upstream.example.test/generated/out.png" {
		t.Fatalf("compat URLs = %#v, want upstream direct URL", urls)
	}
}

func TestCompatResultURLsForAPISupportsFirstPartyProxyURL(t *testing.T) {
	providerBaseURL := "https://upstream.example.test/v1"
	finalTask := &tasks.Task{
		ID:              "task-1",
		Status:          tasks.StatusSuccess,
		ProviderBaseURL: &providerBaseURL,
		ResultJSON: map[string]any{
			"data": []any{map[string]any{"url": "https://cdn.example.test/generated/out.png"}},
		},
		CreatedAt: time.Unix(1784430000, 0),
		UpdatedAt: time.Unix(1784430000, 0),
	}

	urls := compatResultURLsForAPI(finalTask, true, false)
	if len(urls) != 1 || urls[0] != "/api/tasks/task-1/images/0" {
		t.Fatalf("compat proxy URLs = %#v, want first-party task image URL", urls)
	}
}

func TestCompatResultURLsForAPISupportsBase64Response(t *testing.T) {
	finalTask := &tasks.Task{
		ID:     "task-1",
		Status: tasks.StatusSuccess,
		ResultJSON: map[string]any{
			"data": []any{map[string]any{"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lYV0ZQAAAABJRU5ErkJggg=="}},
		},
		CreatedAt: time.Unix(1784430000, 0),
		UpdatedAt: time.Unix(1784430000, 0),
	}

	urls := compatResultURLsForAPI(finalTask, false, true)
	if len(urls) != 1 || !strings.HasPrefix(urls[0], "iVBOR") {
		t.Fatalf("compat b64 values = %#v, want raw b64 image", urls)
	}
}

func TestCompatResultValuesForAPIConvertsURLToBase64WhenRequested(t *testing.T) {
	imageBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lYV0ZQAAAABJRU5ErkJggg=="
	imageBytes, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer upstream.Close()

	finalTask := &tasks.Task{
		ID:     "task-1",
		Status: tasks.StatusSuccess,
		ResultJSON: map[string]any{
			"data": []any{map[string]any{"url": upstream.URL + "/generated/out.png"}},
		},
		CreatedAt: time.Unix(1784430000, 0),
		UpdatedAt: time.Unix(1784430000, 0),
	}

	values, err := compatResultValuesForAPI(context.Background(), finalTask, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != imageBase64 {
		t.Fatalf("compat b64 values = %#v, want converted base64 image", values)
	}
}

func TestWriteCompatImageSuccessReturnsBase64JSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://aipi.example.test/v1/images/generations", nil)
	writeCompatImageSuccess(recorder, request, compatImageInput{Model: "gpt-image-2", ResponseFormat: "b64_json"}, []string{
		"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lYV0ZQAAAABJRU5ErkJggg==",
	}, compatImageResponseOpenAI, "log-1")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Data []struct {
			URL string `json:"url"`
			B64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 1 || response.Data[0].URL != "" || !strings.HasPrefix(response.Data[0].B64, "iVBOR") {
		t.Fatalf("unexpected image response: %+v", response)
	}
}

func TestWriteCompatImageChatCompletionSupportsStreaming(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCompatImageChatCompletion(recorder, compatImageInput{Model: "gpt-image-2", Stream: true}, []map[string]string{{
		"url": "https://aipi.example.test/api/tasks/task-1/images/0",
	}}, 1784430000, "chatcmpl-log1")

	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("content type = %q", contentType)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"chat.completion.chunk", "![image](https://aipi.example.test/api/tasks/task-1/images/0)", `"finish_reason":"stop"`, "data: [DONE]"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("stream response is missing %q: %s", expected, body)
		}
	}
}
