package imagecache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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
