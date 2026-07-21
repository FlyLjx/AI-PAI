package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aipi-go/internal/config"
)

func TestActionsRunNumber(t *testing.T) {
	if got := actionsRunNumber("build-42"); got != 42 {
		t.Fatalf("actionsRunNumber(build-42) = %d, want 42", got)
	}
	if got := actionsRunNumber("go-dev"); got != 0 {
		t.Fatalf("actionsRunNumber(go-dev) = %d, want 0", got)
	}
}

func TestLatestActionsVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/repos/FlyLjx/AI-PAI/actions/workflows/build.yml/runs" {
			t.Fatalf("unexpected GitHub API path: %s", req.URL.Path)
		}
		writeJSON(w, http.StatusOK, map[string]any{"workflow_runs": []map[string]any{{
			"id":             12345,
			"run_number":     17,
			"head_branch":    "main",
			"head_sha":       "abc123",
			"status":         "completed",
			"conclusion":     "success",
			"html_url":       "https://github.example/actions/12345",
			"updated_at":     "2026-07-16T08:00:00Z",
			"run_started_at": "2026-07-16T07:59:00Z",
		}}})
	}))
	defer server.Close()

	router := &Router{cfg: config.Config{
		GitHubAPIBaseURL: server.URL,
		GitHubRepository: "FlyLjx/AI-PAI",
		GitHubWorkflow:   "build.yml",
	}}
	version, err := router.latestActionsVersion(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if version.Version != "build-17" || version.RunID != 12345 || version.Commit != "abc123" {
		t.Fatalf("unexpected version: %+v", version)
	}
}

func TestQueueSystemUpdateWritesCompleteRequestAndState(t *testing.T) {
	directory := t.TempDir()
	request := systemUpdateRequest{
		RunID:          99,
		RunNumber:      12,
		Version:        "build-12",
		Commit:         "commit-12",
		RequestedBy:    "admin-1",
		RequestedAt:    time.Now().UTC().Format(time.RFC3339),
		CurrentVersion: "build-11",
		Force:          true,
	}
	state := systemUpdateState{Status: "waiting_idle", TargetVersion: request.Version, TargetRunID: request.RunID, PendingTaskCount: 2, Force: request.Force}
	if err := queueSystemUpdate(directory, request, state); err != nil {
		t.Fatal(err)
	}

	requestData, err := os.ReadFile(filepath.Join(directory, "request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var savedRequest systemUpdateRequest
	if err := json.Unmarshal(requestData, &savedRequest); err != nil {
		t.Fatal(err)
	}
	if savedRequest.Version != request.Version || savedRequest.RunID != request.RunID || !savedRequest.Force {
		t.Fatalf("unexpected request: %+v", savedRequest)
	}

	stateData, err := os.ReadFile(filepath.Join(directory, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var savedState systemUpdateState
	if err := json.Unmarshal(stateData, &savedState); err != nil {
		t.Fatal(err)
	}
	if savedState.Status != "waiting_idle" || savedState.TargetVersion != request.Version || savedState.PendingTaskCount != 2 || !savedState.Force {
		t.Fatalf("unexpected state: %+v", savedState)
	}

	if err := queueSystemUpdate(directory, request, state); !os.IsExist(err) {
		t.Fatalf("second queue error = %v, want os.ErrExist", err)
	}
}

func TestSignalSystemUpdateForceWritesMarkerAndState(t *testing.T) {
	directory := t.TempDir()
	state := systemUpdateState{
		Status:           "queued",
		TargetVersion:    "build-20",
		TargetRunID:      200,
		TargetCommit:     "commit-20",
		Message:          "强制更新已确认",
		PendingTaskCount: 3,
		Force:            true,
	}
	requestedAt := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	if err := signalSystemUpdateForce(directory, "admin-1", requestedAt, state); err != nil {
		t.Fatal(err)
	}

	forceData, err := os.ReadFile(filepath.Join(directory, "force.json"))
	if err != nil {
		t.Fatal(err)
	}
	var forcePayload map[string]any
	if err := json.Unmarshal(forceData, &forcePayload); err != nil {
		t.Fatal(err)
	}
	if forcePayload["force"] != true || forcePayload["requestedBy"] != "admin-1" {
		t.Fatalf("unexpected force signal: %+v", forcePayload)
	}

	stateData, err := os.ReadFile(filepath.Join(directory, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var savedState systemUpdateState
	if err := json.Unmarshal(stateData, &savedState); err != nil {
		t.Fatal(err)
	}
	if savedState.Status != "queued" || !savedState.Force || savedState.PendingTaskCount != 3 {
		t.Fatalf("unexpected forced state: %+v", savedState)
	}
}
