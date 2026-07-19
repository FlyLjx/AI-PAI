package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
