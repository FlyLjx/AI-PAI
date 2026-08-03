package systemlogs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"aipi-go/internal/imagecache"
	"aipi-go/internal/settings"
)

func TestCategoryRecognizesErrorLogFileNames(t *testing.T) {
	values := []string{
		"dev-go-watch.err.log",
		"next-user-pages-3003.err.log",
		"service.stderr.log",
		"worker-fatal.log",
		"panic-output.log",
	}
	for _, value := range values {
		if got := category(value); got != "error" {
			t.Fatalf("category(%q) = %q, want error", value, got)
		}
	}
}

func TestCleanupOlderThanDeletesOnlyExpiredLogFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.Local)
	oldTime := now.Add(-31 * 24 * time.Hour)
	recentTime := now.Add(-29 * 24 * time.Hour)

	writeTestFile(t, dir, "app-2026-06-27.log", oldTime)
	writeTestFile(t, dir, "worker.err.log", oldTime)
	writeTestFile(t, dir, "blocked..log", oldTime)
	writeTestFile(t, dir, "app-2026-07-01.log", recentTime)
	writeTestFile(t, dir, "app-2026-07-28.log", oldTime)
	writeTestFile(t, dir, "old.txt", oldTime)
	if err := os.Mkdir(filepath.Join(dir, "archived.log"), 0755); err != nil {
		t.Fatal(err)
	}

	result, err := New(dir).CleanupOlderThan(now, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Deleted, []string{"app-2026-06-27.log", "worker.err.log"}) {
		t.Fatalf("deleted = %#v", result.Deleted)
	}
	if len(result.Failures) != 1 || result.Failures[0].Name != "blocked..log" {
		t.Fatalf("failures = %#v", result.Failures)
	}
	for _, name := range []string{"app-2026-06-27.log", "worker.err.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists: %v", name, err)
		}
	}
	for _, name := range []string{"blocked..log", "app-2026-07-01.log", "app-2026-07-28.log", "old.txt", "archived.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was removed: %v", name, err)
		}
	}
	content, err := os.ReadFile(filepath.Join(dir, "blocked..log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep contents" {
		t.Fatalf("failed deletion truncated content: %q", content)
	}
}

func TestCleanupOlderThanKeepsBoundaryFileAndRejectsInvalidRetention(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.Local)
	writeTestFile(t, dir, "boundary.log", now.Add(-30*24*time.Hour))

	result, err := New(dir).CleanupOlderThan(now, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 0 || !slices.Contains(result.Skipped, "boundary.log") {
		t.Fatalf("result = %#v", result)
	}
	if _, err := New(dir).CleanupOlderThan(now, 0); err == nil {
		t.Fatal("expected invalid retention error")
	}
	if _, err := New(dir).CleanupOlderThan(now, maxLogRetentionDays+1); err == nil {
		t.Fatal("expected retention upper-bound error")
	}
}

func TestCleanupOlderThanMissingDirectoryIsSuccessful(t *testing.T) {
	result, err := New(filepath.Join(t.TempDir(), "missing")).CleanupOlderThan(time.Now(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 0 || len(result.Failures) != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestAutoCleanupWorkerRunsImmediatelyAndStops(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.Local)
	oldName := "app-2026-06-01.log"
	writeTestFile(t, dir, oldName, now.Add(-60*24*time.Hour))

	worker := autoCleanupWorker{
		service: New(dir),
		loadSettings: func(context.Context) (settings.Settings, error) {
			return settings.Settings{
				"systemLogAutoCleanupEnabled": true,
				"systemLogRetentionDays":      float64(30),
			}, nil
		},
		now:      func() time.Time { return now },
		interval: time.Hour,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := worker.start(ctx)

	deadline := time.Now().Add(time.Second)
	for {
		_, err := os.Stat(filepath.Join(dir, oldName))
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not run immediately")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestAutoCleanupWorkerRunsOnInterval(t *testing.T) {
	var calls atomic.Int32
	worker := autoCleanupWorker{
		service: New(t.TempDir()),
		loadSettings: func(context.Context) (settings.Settings, error) {
			calls.Add(1)
			return settings.Settings{"systemLogAutoCleanupEnabled": false}, nil
		},
		now:      time.Now,
		interval: 5 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := worker.start(ctx)
	deadline := time.Now().Add(time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if calls.Load() < 2 {
		t.Fatalf("settings calls = %d, want at least 2", calls.Load())
	}
}

func TestAutoCleanupWorkerCleansTaskImagesIndependently(t *testing.T) {
	taskImageDir := t.TempDir()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.Local)
	taskDir := filepath.Join(taskImageDir, "task-old")
	if err := os.Mkdir(taskDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, taskDir, "0.png", now.Add(-48*time.Hour))
	if err := os.Chtimes(taskDir, now.Add(-48*time.Hour), now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}

	worker := autoCleanupWorker{
		service:       New(filepath.Join(t.TempDir(), "logs")),
		taskImageDir:  taskImageDir,
		cleanupImages: imagecache.CleanupTaskImagesOlderThanIn,
		loadSettings: func(context.Context) (settings.Settings, error) {
			return settings.Settings{
				"systemLogAutoCleanupEnabled": true,
				"systemLogRetentionDays":      float64(30),
				"taskImageAutoCleanupEnabled": true,
				"taskImageRetentionDays":      float64(1),
			}, nil
		},
		now:      func() time.Time { return now },
		interval: time.Hour,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := worker.start(ctx)
	deadline := time.Now().Add(time.Second)
	for {
		_, err := os.Stat(taskDir)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not clean task images while running log cleanup")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestCleanupSettingPositiveIntBounds(t *testing.T) {
	for _, value := range []any{float64(0), float64(1.5), float64(maxLogRetentionDays + 1), maxLogRetentionDays + 1, int64(maxLogRetentionDays + 1)} {
		if got, ok := cleanupSettingPositiveInt(value); ok {
			t.Fatalf("cleanupSettingPositiveInt(%v) = %d, true", value, got)
		}
	}
	for _, value := range []any{float64(1), float64(maxLogRetentionDays), maxLogRetentionDays, int64(maxLogRetentionDays)} {
		if got, ok := cleanupSettingPositiveInt(value); !ok || got != int(anyNumber(value)) {
			t.Fatalf("cleanupSettingPositiveInt(%v) = %d, %t", value, got, ok)
		}
	}
}

func anyNumber(value any) int64 {
	switch item := value.(type) {
	case float64:
		return int64(item)
	case int:
		return int64(item)
	case int64:
		return item
	default:
		return 0
	}
}

func writeTestFile(t *testing.T, dir string, name string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("keep contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}
