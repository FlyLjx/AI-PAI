package apiaccess

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"aipi-go/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestToPublicLogIncludesRequestAndSuccessResponseParameters(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	finishedAt := createdAt.Add(45 * time.Second)
	taskID := "task-1"
	publicLog := ToPublicLog(UsageLog{
		ID:              "log-1",
		TaskID:          &taskID,
		Status:          "success",
		ImageCount:      2,
		DurationSeconds: 2.375,
		RequestParams: map[string]any{
			"model": "image-model",
			"n":     2,
		},
		CreatedAt:  createdAt,
		FinishedAt: &finishedAt,
	})

	if publicLog.DurationSeconds != 2.375 {
		t.Fatalf("expected durationSeconds 2.375, got %v", publicLog.DurationSeconds)
	}
	if publicLog.RequestParams["model"] != "image-model" || publicLog.RequestParams["n"] != 2 {
		t.Fatalf("unexpected request parameters: %#v", publicLog.RequestParams)
	}
	if publicLog.ResponseParams["created"] != finishedAt.Unix() {
		t.Fatalf("unexpected response created timestamp: %#v", publicLog.ResponseParams)
	}
	data, ok := publicLog.ResponseParams["data"].([]map[string]string)
	if !ok || len(data) != 2 {
		t.Fatalf("unexpected response data: %#v", publicLog.ResponseParams["data"])
	}
	if data[0]["url"] != "/api/tasks/task-1/images/0" || data[1]["url"] != "/api/tasks/task-1/images/1" {
		t.Fatalf("unexpected response URLs: %#v", data)
	}
}

func TestToPublicLogIncludesFailureResponseParameters(t *testing.T) {
	message := "上游接口返回错误"
	publicLog := ToPublicLog(UsageLog{
		Status:       "failed",
		ErrorMessage: &message,
		CreatedAt:    time.Now(),
	})

	errorPayload, ok := publicLog.ResponseParams["error"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected error response: %#v", publicLog.ResponseParams)
	}
	if errorPayload["message"] != message || errorPayload["type"] != "api_error" {
		t.Fatalf("unexpected error payload: %#v", errorPayload)
	}
	if errorPayload["param"] != nil || errorPayload["code"] != nil {
		t.Fatalf("expected nil param and code: %#v", errorPayload)
	}
}

func TestToPublicLogBuildsChatCompletionForImageChatCompatibility(t *testing.T) {
	createdAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	finishedAt := createdAt.Add(30 * time.Second)
	taskID := "task-chat"
	publicLog := ToPublicLog(UsageLog{
		ID:         "log-chat-1",
		TaskID:     &taskID,
		Endpoint:   "/v1/chat/completions",
		Model:      "gpt-image-2",
		Status:     "success",
		ImageCount: 1,
		CreatedAt:  createdAt,
		FinishedAt: &finishedAt,
	})

	if publicLog.ResponseParams["object"] != "chat.completion" || publicLog.ResponseParams["id"] != "chatcmpl-logchat1" {
		t.Fatalf("unexpected chat response metadata: %#v", publicLog.ResponseParams)
	}
	choices, ok := publicLog.ResponseParams["choices"].([]map[string]any)
	if !ok || len(choices) != 1 {
		t.Fatalf("unexpected chat choices: %#v", publicLog.ResponseParams["choices"])
	}
	message, ok := choices[0]["message"].(map[string]any)
	if !ok || message["content"] != "![image](/api/tasks/task-chat/images/0)" {
		t.Fatalf("unexpected chat message: %#v", choices[0]["message"])
	}
}

func TestToPublicLogOmitsResponseParametersBeforeCompletion(t *testing.T) {
	publicLog := ToPublicLog(UsageLog{Status: "processing", CreatedAt: time.Now()})
	if publicLog.ResponseParams != nil {
		t.Fatalf("expected no response parameters while processing, got %#v", publicLog.ResponseParams)
	}
}

func TestToAdminPublicLogIncludesChargeAndModelCost(t *testing.T) {
	adminLog := ToAdminPublicLog(UsageLog{
		Status:           "success",
		ChargedCredits:   0,
		ModelCostCredits: 0.125,
		CreatedAt:        time.Now(),
	})
	if adminLog.ChargedCredits != 0 {
		t.Fatalf("chargedCredits = %v, want 0", adminLog.ChargedCredits)
	}
	if adminLog.ModelCostCredits != 0.125 {
		t.Fatalf("modelCostCredits = %v, want 0.125", adminLog.ModelCostCredits)
	}
}

func TestToPublicLogExposesChargeButNotModelCost(t *testing.T) {
	payload, err := json.Marshal(ToPublicLog(UsageLog{
		ChargedCredits: 1, ModelCostCredits: 0.5, CreatedAt: time.Now(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	body := string(payload)
	if !strings.Contains(body, `"chargedCredits":1`) {
		t.Fatalf("public usage log omitted user charge: %s", body)
	}
	if strings.Contains(body, "modelCostCredits") {
		t.Fatalf("public usage log exposed internal billing data: %s", body)
	}
}

func TestFinishLogSnapshotsChargeAndModelCost(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()

	mock.ExpectExec(`UPDATE api_access_logs SET status = \?, image_count = \?, error_message = \?, charged_credits = CASE`).
		WithArgs("success", 1, nil, "success", "success", "log-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := NewRepository(database.Wrap(rawDB)).FinishLog(context.Background(), "log-1", "success", 1, ""); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
