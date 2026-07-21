package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/build"
)

const systemUpdateCacheTTL = 2 * time.Minute

type systemUpdateVersion struct {
	Version     string `json:"version"`
	RunID       int64  `json:"runId,omitempty"`
	RunNumber   int    `json:"runNumber,omitempty"`
	Commit      string `json:"commit"`
	PublishedAt string `json:"publishedAt,omitempty"`
	URL         string `json:"url,omitempty"`
}

type systemUpdateState struct {
	Status           string `json:"status"`
	TargetVersion    string `json:"targetVersion,omitempty"`
	TargetRunID      int64  `json:"targetRunId,omitempty"`
	TargetCommit     string `json:"targetCommit,omitempty"`
	Message          string `json:"message,omitempty"`
	BackupDirectory  string `json:"backupDirectory,omitempty"`
	PendingTaskCount int    `json:"pendingTaskCount,omitempty"`
	Force            bool   `json:"force,omitempty"`
	StartedAt        string `json:"startedAt,omitempty"`
	FinishedAt       string `json:"finishedAt,omitempty"`
}

type systemUpdateView struct {
	Configured       bool                `json:"configured"`
	Current          systemUpdateVersion `json:"current"`
	Latest           systemUpdateVersion `json:"latest"`
	UpdateAvailable  bool                `json:"updateAvailable"`
	CanUpdate        bool                `json:"canUpdate"`
	CheckError       string              `json:"checkError,omitempty"`
	PendingTaskCount int                 `json:"pendingTaskCount"`
	State            systemUpdateState   `json:"state"`
	CheckedAt        string              `json:"checkedAt"`
}

type systemUpdateRequest struct {
	RunID          int64  `json:"runId"`
	RunNumber      int    `json:"runNumber"`
	Version        string `json:"version"`
	Commit         string `json:"commit"`
	URL            string `json:"url"`
	RequestedBy    string `json:"requestedBy"`
	RequestedAt    string `json:"requestedAt"`
	CurrentVersion string `json:"currentVersion"`
	Force          bool   `json:"force"`
}

type systemUpdatePostInput struct {
	Force bool `json:"force"`
}

type actionsRunsResponse struct {
	WorkflowRuns []struct {
		ID           int64  `json:"id"`
		RunNumber    int    `json:"run_number"`
		HeadBranch   string `json:"head_branch"`
		HeadSHA      string `json:"head_sha"`
		Status       string `json:"status"`
		Conclusion   string `json:"conclusion"`
		HTMLURL      string `json:"html_url"`
		UpdatedAt    string `json:"updated_at"`
		RunStartedAt string `json:"run_started_at"`
	} `json:"workflow_runs"`
}

func (r *Router) systemUpdate(w http.ResponseWriter, req *http.Request) {
	admin, err := r.requireAdmin(req)
	if err != nil {
		writeError(w, err)
		return
	}

	switch req.Method {
	case http.MethodGet:
		force := strings.EqualFold(req.URL.Query().Get("refresh"), "true") || req.URL.Query().Get("refresh") == "1"
		view := r.systemUpdateView(req.Context(), force)
		writeJSON(w, http.StatusOK, map[string]any{"data": view})
	case http.MethodPost:
		forceUpdate, err := decodeSystemUpdateForce(req)
		if err != nil {
			writeError(w, newAppError(http.StatusBadRequest, "更新参数格式不正确"))
			return
		}
		view := r.systemUpdateView(req.Context(), true)
		if !view.Configured {
			writeError(w, newAppError(http.StatusServiceUnavailable, "服务器尚未配置系统更新服务"))
			return
		}
		if forceUpdate && strings.EqualFold(view.State.Status, "waiting_idle") {
			forced := view.State
			forced.Status = "queued"
			forced.Message = "强制更新已确认，将跳过任务等待直接更新"
			forced.Force = true
			forced.PendingTaskCount = view.PendingTaskCount
			if err := signalSystemUpdateForce(r.cfg.SystemUpdateDir, admin.UserID, time.Now().UTC(), forced); err != nil {
				writeError(w, err)
				return
			}
			view.State = forced
			view.CanUpdate = false
			writeJSON(w, http.StatusAccepted, map[string]any{"data": view})
			return
		}
		if view.CheckError != "" || view.Latest.RunID == 0 {
			writeError(w, newAppError(http.StatusBadGateway, "暂时无法获取 GitHub Actions 最新版本"))
			return
		}
		if !view.UpdateAvailable {
			writeError(w, newAppError(http.StatusConflict, "当前已经是最新版本"))
			return
		}
		if isSystemUpdateActive(view.State.Status) {
			writeError(w, newAppError(http.StatusConflict, "已有系统更新正在执行"))
			return
		}

		request := systemUpdateRequest{
			RunID:          view.Latest.RunID,
			RunNumber:      view.Latest.RunNumber,
			Version:        view.Latest.Version,
			Commit:         view.Latest.Commit,
			URL:            view.Latest.URL,
			RequestedBy:    admin.UserID,
			RequestedAt:    time.Now().UTC().Format(time.RFC3339),
			CurrentVersion: view.Current.Version,
			Force:          forceUpdate,
		}
		status := "queued"
		message := "更新请求已提交，等待服务器开始处理"
		if forceUpdate {
			message = "强制更新请求已提交，将直接替换应用容器"
		} else if view.PendingTaskCount > 0 {
			status = "waiting_idle"
			message = fmt.Sprintf("更新请求已提交，等待 %d 个运行中任务完成后自动更新", view.PendingTaskCount)
		}
		queued := systemUpdateState{
			Status:           status,
			TargetVersion:    request.Version,
			TargetRunID:      request.RunID,
			TargetCommit:     request.Commit,
			Message:          message,
			PendingTaskCount: view.PendingTaskCount,
			Force:            forceUpdate,
			StartedAt:        request.RequestedAt,
		}
		if err := queueSystemUpdate(r.cfg.SystemUpdateDir, request, queued); err != nil {
			if errors.Is(err, os.ErrExist) {
				writeError(w, newAppError(http.StatusConflict, "已有系统更新等待执行"))
				return
			}
			writeError(w, err)
			return
		}
		view.State = queued
		view.CanUpdate = false
		writeJSON(w, http.StatusAccepted, map[string]any{"data": view})
	default:
		writeMethodNotAllowed(w)
	}
}

func (r *Router) systemUpdateView(ctx context.Context, force bool) systemUpdateView {
	current := currentSystemVersion()
	state := readSystemUpdateState(r.cfg.SystemUpdateDir)
	latest, err := r.latestActionsVersion(ctx, force)
	pendingTaskCount, taskCountErr := r.systemUpdatePendingTaskCount(ctx)
	view := systemUpdateView{
		Configured:       strings.TrimSpace(r.cfg.SystemUpdateDir) != "",
		Current:          current,
		Latest:           latest,
		PendingTaskCount: pendingTaskCount,
		State:            state,
		CheckedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	if err != nil {
		view.CheckError = err.Error()
	} else if taskCountErr != nil {
		view.CheckError = taskCountErr.Error()
	}
	view.UpdateAvailable = latest.RunID != 0 && (latest.Version != current.Version || latest.Commit != current.Commit)
	view.CanUpdate = view.Configured && view.CheckError == "" && view.UpdateAvailable && !isSystemUpdateActive(state.Status)
	return view
}

func decodeSystemUpdateForce(req *http.Request) (bool, error) {
	force := strings.EqualFold(req.URL.Query().Get("force"), "true") || req.URL.Query().Get("force") == "1"
	if req.Body == nil {
		return force, nil
	}
	defer req.Body.Close()
	var input systemUpdatePostInput
	if err := json.NewDecoder(io.LimitReader(req.Body, 4096)).Decode(&input); err != nil {
		if errors.Is(err, io.EOF) {
			return force, nil
		}
		return false, err
	}
	return force || input.Force, nil
}

func (r *Router) systemUpdatePendingTaskCount(ctx context.Context) (int, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var count int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM generation_tasks
		WHERE status IN ('queued', 'pending', 'processing')
	`).Scan(&count); err != nil {
		return 0, fmt.Errorf("运行任务统计失败: %w", err)
	}
	return count, nil
}

func currentSystemVersion() systemUpdateVersion {
	return systemUpdateVersion{
		Version:     defaultString(strings.TrimSpace(build.Version), "go-dev"),
		RunNumber:   actionsRunNumber(build.Version),
		Commit:      defaultString(strings.TrimSpace(build.Commit), "local"),
		PublishedAt: strings.TrimSpace(build.Time),
	}
}

func actionsRunNumber(version string) int {
	value := strings.TrimPrefix(strings.TrimSpace(version), "build-")
	if value == version {
		return 0
	}
	number, _ := strconv.Atoi(value)
	return number
}

func isSystemUpdateActive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "waiting_idle", "checking", "pulling", "backing_up", "updating", "rolling_back":
		return true
	default:
		return false
	}
}

func (r *Router) latestActionsVersion(ctx context.Context, force bool) (systemUpdateVersion, error) {
	r.updateMu.Lock()
	defer r.updateMu.Unlock()
	if !force && r.updateCache.RunID != 0 && time.Since(r.updateCacheAt) < systemUpdateCacheTTL {
		return r.updateCache, nil
	}

	endpoint := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/runs?branch=main&status=success&per_page=1",
		strings.TrimRight(r.cfg.GitHubAPIBaseURL, "/"),
		strings.Trim(strings.TrimSpace(r.cfg.GitHubRepository), "/"),
		url.PathEscape(strings.TrimSpace(r.cfg.GitHubWorkflow)),
	)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return systemUpdateVersion{}, err
	}
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("User-Agent", "AI-PAI-system-update")
	client := &http.Client{Timeout: 12 * time.Second}
	response, err := client.Do(httpReq)
	if err != nil {
		return systemUpdateVersion{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return systemUpdateVersion{}, fmt.Errorf("GitHub Actions 返回状态 %d", response.StatusCode)
	}
	var payload actionsRunsResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&payload); err != nil {
		return systemUpdateVersion{}, err
	}
	if len(payload.WorkflowRuns) == 0 {
		return systemUpdateVersion{}, errors.New("没有找到成功的 main 分支构建")
	}
	run := payload.WorkflowRuns[0]
	if run.Status != "completed" || run.Conclusion != "success" || run.HeadBranch != "main" {
		return systemUpdateVersion{}, errors.New("最新构建尚未成功完成")
	}
	publishedAt := run.UpdatedAt
	if publishedAt == "" {
		publishedAt = run.RunStartedAt
	}
	version := systemUpdateVersion{
		Version:     "build-" + strconv.Itoa(run.RunNumber),
		RunID:       run.ID,
		RunNumber:   run.RunNumber,
		Commit:      run.HeadSHA,
		PublishedAt: publishedAt,
		URL:         run.HTMLURL,
	}
	r.updateCache = version
	r.updateCacheAt = time.Now()
	return version, nil
}

func readSystemUpdateState(directory string) systemUpdateState {
	if strings.TrimSpace(directory) == "" {
		return systemUpdateState{Status: "unconfigured", Message: "服务器尚未配置系统更新服务"}
	}
	data, err := os.ReadFile(filepath.Join(directory, "status.json"))
	if errors.Is(err, os.ErrNotExist) {
		return systemUpdateState{Status: "idle"}
	}
	if err != nil {
		return systemUpdateState{Status: "failed", Message: "更新状态读取失败"}
	}
	var state systemUpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		return systemUpdateState{Status: "failed", Message: "更新状态文件损坏"}
	}
	if strings.TrimSpace(state.Status) == "" {
		state.Status = "idle"
	}
	return state
}

func queueSystemUpdate(directory string, request systemUpdateRequest, state systemUpdateState) error {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return errors.New("system update directory is not configured")
	}
	if err := os.MkdirAll(directory, 0750); err != nil {
		return err
	}
	requestPath := filepath.Join(directory, "request.json")
	if _, err := os.Stat(requestPath); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := writeJSONFileAtomic(filepath.Join(directory, "status.json"), state); err != nil {
		return err
	}
	return writeJSONFileAtomic(requestPath, request)
}

func signalSystemUpdateForce(directory string, requestedBy string, requestedAt time.Time, state systemUpdateState) error {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return errors.New("system update directory is not configured")
	}
	if err := os.MkdirAll(directory, 0750); err != nil {
		return err
	}
	forcePath := filepath.Join(directory, "force.json")
	payload := map[string]any{
		"force":       true,
		"requestedBy": strings.TrimSpace(requestedBy),
		"requestedAt": requestedAt.Format(time.RFC3339),
	}
	if err := writeJSONFileAtomic(forcePath, payload); err != nil {
		return err
	}
	return writeJSONFileAtomic(filepath.Join(directory, "status.json"), state)
}

func writeJSONFileAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".system-update-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
