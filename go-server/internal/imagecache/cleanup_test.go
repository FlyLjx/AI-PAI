package imagecache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aipi-go/internal/cleanupstatus"
)

func TestCleanupTaskImagesOlderThanInRemovesExpiredTaskDirectories(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.Local)
	old := now.Add(-25 * time.Hour)
	recent := now.Add(-23 * time.Hour)

	oldTask := filepath.Join(dir, "task-old")
	if err := os.Mkdir(oldTask, 0755); err != nil {
		t.Fatal(err)
	}
	writeCachedImage(t, oldTask, "0.png", old)
	if err := os.Chtimes(oldTask, old, old); err != nil {
		t.Fatal(err)
	}

	emptyTask := filepath.Join(dir, "task-empty")
	if err := os.Mkdir(emptyTask, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(emptyTask, old, old); err != nil {
		t.Fatal(err)
	}

	recentTask := filepath.Join(dir, "task-recent")
	if err := os.Mkdir(recentTask, 0755); err != nil {
		t.Fatal(err)
	}
	writeCachedImage(t, recentTask, "0.png", recent)

	result, err := CleanupTaskImagesOlderThanIn(dir, now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if len(result.Deleted) != 2 {
		t.Fatalf("deleted = %#v, want two task directories", result.Deleted)
	}
	for _, name := range []string{"task-old", "task-empty"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(recentTask, "0.png")); err != nil {
		t.Fatalf("recent cache was removed: %v", err)
	}
}

func TestCleanupTaskImagesOlderThanInSkipsSymlinksAndMissingDirectory(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.Local)
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-48 * time.Hour)
	writeCachedImage(t, target, "0.png", old)
	if err := os.Chtimes(target, old, old); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}

	result, err := CleanupTaskImagesOlderThanIn(dir, now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "target" {
		t.Fatalf("deleted = %#v, want target only", result.Deleted)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("symlink was removed or followed: %v", err)
	}

	result, err = CleanupTaskImagesOlderThanIn(filepath.Join(dir, "missing"), now, 1)
	if err != nil || len(result.Deleted) != 0 || len(result.Failures) != 0 {
		t.Fatalf("missing directory result = %#v, error = %v", result, err)
	}
}

func TestCleanupTaskImagesOlderThanInRejectsInvalidRetention(t *testing.T) {
	for _, retentionDays := range []int{0, maxTaskImageRetentionDays + 1} {
		if _, err := CleanupTaskImagesOlderThanIn(t.TempDir(), time.Now(), retentionDays); err == nil {
			t.Fatalf("retention %d was accepted", retentionDays)
		}
	}
}

func TestPurgeTaskImagesInRemovesTaskDirectoriesOnly(t *testing.T) {
	dir := t.TempDir()
	taskDir := filepath.Join(dir, "task-1")
	if err := os.Mkdir(taskDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeCachedImage(t, taskDir, "0.png", time.Now())
	marker := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := PurgeTaskImagesIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != "task-1" {
		t.Fatalf("deleted = %#v", result.Deleted)
	}
	if _, err := os.Stat(taskDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("task directory still exists: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("non-directory marker was removed: %v", err)
	}
}

func TestPurgeTaskImagesInContextCanBeCanceled(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"task-1", "task-2"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PurgeTaskImagesInContext(ctx, dir, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestPurgeTaskImagesInContextReportsReleasedBytes(t *testing.T) {
	dir := t.TempDir()
	taskDir := filepath.Join(dir, "task-1")
	if err := os.Mkdir(taskDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "image.png"), []byte("123456"), 0644); err != nil {
		t.Fatal(err)
	}
	tracker := cleanupstatus.New()
	tracker.Start(time.Now())
	ctx := cleanupstatus.WithTracker(context.Background(), tracker)
	if _, err := PurgeTaskImagesInContext(ctx, dir, 0); err != nil {
		t.Fatal(err)
	}
	status := tracker.Snapshot()
	if status.DeletedCacheDirectories != 1 || status.ReleasedCacheBytes != 6 {
		t.Fatalf("cache progress = %#v", status)
	}
}

func writeCachedImage(t *testing.T, dir, name string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("cached image"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}
