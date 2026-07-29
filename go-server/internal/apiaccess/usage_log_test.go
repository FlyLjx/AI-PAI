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
		TaskUsage: map[string]any{
			"input_tokens":  float64(55531),
			"output_tokens": float64(24576),
			"total_tokens":  float64(80107),
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
	usage, ok := publicLog.ResponseParams["usage"].(map[string]int)
	if !ok || usage["input_tokens"] != 55531 || usage["output_tokens"] != 24576 || usage["total_tokens"] != 80107 {
		t.Fatalf("unexpected response usage: %#v", publicLog.ResponseParams["usage"])
	}
}

func TestToPublicLogSummarizesBase64ImageResponse(t *testing.T) {
	taskID := "task-base64"
	publicLog := ToPublicLog(UsageLog{
		TaskID:         &taskID,
		Endpoint:       "/v1/images/generations",
		Status:         "success",
		ImageCount:     1,
		ResponseFormat: "b64_json",
		CreatedAt:      time.Now(),
	})

	data, ok := publicLog.ResponseParams["data"].([]map[string]string)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected response data: %#v", publicLog.ResponseParams["data"])
	}
	if data[0]["b64_json"] != "[base64 image data omitted from logs]" {
		t.Fatalf("unexpected base64 summary: %#v", data[0])
	}
	if _, exists := data[0]["url"]; exists {
		t.Fatalf("base64 response summary should not contain a URL: %#v", data[0])
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
		TaskUsage: map[string]any{
			"input_tokens":  9690,
			"output_tokens": 6563,
			"total_tokens":  16253,
		},
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
	usage, ok := publicLog.ResponseParams["usage"].(map[string]int)
	if !ok || usage["prompt_tokens"] != 9690 || usage["completion_tokens"] != 6563 || usage["total_tokens"] != 16253 {
		t.Fatalf("unexpected chat response usage: %#v", publicLog.ResponseParams["usage"])
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

func TestBuildLogWhereSupportsAdminLogFilters(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	where, args := buildLogWhere(ListLogsInput{
		UserID:   " user-1 ",
		APIKeyID: " key-1 ",
		Status:   " SUCCESS ",
		Keyword:  " Example@MAIL.COM ",
	})

	for _, fragment := range []string{
		"api_access_logs.user_id = ?",
		"api_access_logs.api_key_id = ?",
		"api_access_logs.status IN ('success', 'succeeded')",
		"api_access_logs.id",
		"api_access_logs.task_id",
		"api_access_logs.error_message",
		"api_access_logs.request_params AS CHAR",
		"users.email",
		"api_access_keys.name",
		"api_access_keys.key_prefix",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where clause omitted %q: %s", fragment, where)
		}
	}
	if len(args) != 17 {
		t.Fatalf("args length = %d, want 17: %#v", len(args), args)
	}
	if args[0] != "user-1" || args[1] != "key-1" {
		t.Fatalf("exact filter args = %#v, want trimmed user and key IDs", args[:2])
	}
	for index, arg := range args[2:] {
		if arg != "%example@mail.com%" {
			t.Fatalf("keyword arg %d = %#v, want lowercase search pattern", index, arg)
		}
	}
}

func TestUsageLogSelectReadsOnlyTaskUsage(t *testing.T) {
	previousDialect := database.CurrentDialect()
	defer database.SetDialect(string(previousDialect))

	database.SetDialect("postgres")
	postgresQuery := usageLogSelect()
	if !strings.Contains(postgresQuery, "generation_tasks.result_json -> 'usage'") {
		t.Fatalf("postgres usage query missing JSONB extraction: %s", postgresQuery)
	}
	if strings.Contains(postgresQuery, "generation_tasks.result_json,") {
		t.Fatalf("postgres usage query should not load the complete task result: %s", postgresQuery)
	}

	database.SetDialect("mysql")
	mysqlQuery := usageLogSelect()
	if !strings.Contains(mysqlQuery, "JSON_EXTRACT(generation_tasks.result_json, '$.usage')") {
		t.Fatalf("mysql usage query missing JSON extraction: %s", mysqlQuery)
	}
}

func TestBuildLogWhereUsesPostgresJSONTextSearch(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("postgres")
	defer database.SetDialect(string(previousDialect))

	where, args := buildLogWhere(ListLogsInput{Keyword: "response_format"})
	if !strings.Contains(where, "CAST(api_access_logs.request_params AS TEXT)") {
		t.Fatalf("postgres keyword filter omitted JSON text cast: %s", where)
	}
	if len(args) != 15 {
		t.Fatalf("args length = %d, want 15", len(args))
	}
}

func TestBuildLogWhereNormalizesStatusAliases(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		wantCondition string
		wantArgs      int
	}{
		{name: "succeeded", status: "succeeded", wantCondition: "status IN ('success', 'succeeded')"},
		{name: "cancelled", status: "cancelled", wantCondition: "status IN ('canceled', 'cancelled')"},
		{name: "failed", status: "FAILED", wantCondition: "status = ?", wantArgs: 1},
		{name: "all", status: "ALL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			where, args := buildLogWhere(ListLogsInput{Status: test.status})
			if test.wantCondition != "" && !strings.Contains(where, test.wantCondition) {
				t.Fatalf("where = %q, want condition %q", where, test.wantCondition)
			}
			if len(args) != test.wantArgs {
				t.Fatalf("args = %#v, want %d values", args, test.wantArgs)
			}
			if test.name == "failed" && args[0] != "failed" {
				t.Fatalf("status arg = %#v, want failed", args[0])
			}
			if test.name == "all" && where != "" {
				t.Fatalf("all status should not add a filter: %q", where)
			}
		})
	}
}
