package imagecache

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

const maxTaskImageRetentionDays = 3650

type TaskImageCleanupFailure struct {
	Name  string
	Error string
}

type TaskImageCleanupResult struct {
	Deleted  []string
	Skipped  []string
	Failures []TaskImageCleanupFailure
}

// CleanupTaskImagesOlderThan removes task image directories whose contents
// have not changed within the retention window. The database rows and task
// history remain untouched; an older task can still fall back to its upstream
// URL or base64 result when requested.
func CleanupTaskImagesOlderThan(now time.Time, retentionDays int) (TaskImageCleanupResult, error) {
	return CleanupTaskImagesOlderThanIn(Directory(), now, retentionDays)
}

func CleanupTaskImagesOlderThanIn(dir string, now time.Time, retentionDays int) (TaskImageCleanupResult, error) {
	result := TaskImageCleanupResult{
		Deleted:  []string{},
		Skipped:  []string{},
		Failures: []TaskImageCleanupFailure{},
	}
	if retentionDays < 1 || retentionDays > maxTaskImageRetentionDays {
		return result, errors.New("图片缓存保留天数必须在 1 到 3650 天之间")
	}
	if now.IsZero() {
		now = time.Now()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return result, err
	}

	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := entry.Name()
		taskDir := filepath.Join(dir, name)
		info, err := entry.Info()
		if err != nil {
			result.Failures = append(result.Failures, TaskImageCleanupFailure{Name: name, Error: err.Error()})
			continue
		}
		if !info.Mode().IsDir() {
			result.Skipped = append(result.Skipped, name)
			continue
		}

		// Directory mtime changes when a cached file is created, but checking
		// files as well protects an active task whose directory mtime is not
		// updated consistently across filesystems.
		latest := info.ModTime()
		children, readErr := os.ReadDir(taskDir)
		if readErr != nil {
			result.Failures = append(result.Failures, TaskImageCleanupFailure{Name: name, Error: readErr.Error()})
			continue
		}
		for _, child := range children {
			if child.Type()&os.ModeSymlink != 0 {
				continue
			}
			childInfo, childErr := child.Info()
			if childErr == nil && childInfo.ModTime().After(latest) {
				latest = childInfo.ModTime()
			}
		}
		if !latest.Before(cutoff) {
			result.Skipped = append(result.Skipped, name)
			continue
		}
		if err := os.RemoveAll(taskDir); err != nil {
			result.Failures = append(result.Failures, TaskImageCleanupFailure{Name: name, Error: err.Error()})
			continue
		}
		result.Deleted = append(result.Deleted, name)
	}
	return result, nil
}
