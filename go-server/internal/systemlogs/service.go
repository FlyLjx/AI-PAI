package systemlogs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	UpdatedAt string `json:"updatedAt"`
	Category  string `json:"category"`
}

type Detail struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Content   string `json:"content"`
	Offset    int64  `json:"offset"`
	Truncated bool   `json:"truncated"`
}

type DeleteResult struct {
	Deleted   bool   `json:"deleted"`
	Name      string `json:"name"`
	Truncated bool   `json:"truncated"`
	Reason    string `json:"reason,omitempty"`
}

type Service struct {
	dir string
}

func New(dir string) Service {
	if dir == "" {
		dir = "logs"
	}
	return Service{dir: dir}
}

func (s Service) List() ([]FileInfo, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []FileInfo{}, nil
		}
		return nil, err
	}
	files := []FileInfo{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name:      entry.Name(),
			Size:      info.Size(),
			UpdatedAt: info.ModTime().Format(time.RFC3339),
			Category:  category(entry.Name()),
		})
	}
	sort.Slice(files, func(i int, j int) bool {
		return files[i].UpdatedAt > files[j].UpdatedAt
	})
	return files, nil
}

func (s Service) Read(name string, maxBytes int64) (Detail, error) {
	path, err := s.safePath(name)
	if err != nil {
		return Detail{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Detail{}, err
	}
	if maxBytes <= 0 {
		maxBytes = 300000
	}
	start := info.Size() - maxBytes
	if start < 0 {
		start = 0
	}
	file, err := os.Open(path)
	if err != nil {
		return Detail{}, err
	}
	defer file.Close()
	if _, err := file.Seek(start, 0); err != nil {
		return Detail{}, err
	}
	bytes := make([]byte, info.Size()-start)
	n, _ := file.Read(bytes)
	return Detail{
		Name:      filepath.Base(path),
		Size:      info.Size(),
		Content:   string(bytes[:n]),
		Offset:    start + int64(n),
		Truncated: start > 0,
	}, nil
}

func (s Service) ReadSince(name string, offset int64, maxBytes int64) (Detail, error) {
	path, err := s.safePath(name)
	if err != nil {
		return Detail{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Detail{}, err
	}
	if maxBytes <= 0 {
		maxBytes = 200000
	}
	if offset < 0 {
		offset = 0
	}
	if offset > info.Size() {
		offset = info.Size()
	}
	start := offset
	if info.Size()-start > maxBytes {
		start = info.Size() - maxBytes
	}
	file, err := os.Open(path)
	if err != nil {
		return Detail{}, err
	}
	defer file.Close()
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return Detail{}, err
	}
	bytes := make([]byte, info.Size()-start)
	n, _ := file.Read(bytes)
	return Detail{
		Name:      filepath.Base(path),
		Size:      info.Size(),
		Content:   string(bytes[:n]),
		Offset:    info.Size(),
		Truncated: start > offset,
	}, nil
}

func (s Service) Delete(name string) (DeleteResult, error) {
	path, err := s.safePath(name)
	if err != nil {
		return DeleteResult{}, err
	}
	result := DeleteResult{Name: filepath.Base(path), Deleted: true}
	if filepath.Base(path) == s.currentFileName() {
		return s.truncate(path, result, "active_file_truncated")
	}
	if err := os.Remove(path); err == nil {
		return result, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return DeleteResult{}, err
	}
	return s.truncate(path, result, "delete_failed_truncated")
}

func (s Service) safePath(name string) (string, error) {
	if name == "" {
		files, err := s.List()
		if err != nil {
			return "", err
		}
		if len(files) == 0 {
			return "", os.ErrNotExist
		}
		name = files[0].Name
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") || !strings.HasSuffix(strings.ToLower(name), ".log") {
		return "", errors.New("日志文件名不合法")
	}
	root, err := filepath.Abs(s.dir)
	if err != nil {
		return "", err
	}
	path, err := filepath.Abs(filepath.Join(s.dir, name))
	if err != nil {
		return "", err
	}
	if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return "", errors.New("日志文件路径不合法")
	}
	return path, nil
}

func (s Service) truncate(path string, result DeleteResult, reason string) (DeleteResult, error) {
	if err := os.Truncate(path, 0); err != nil {
		return DeleteResult{}, err
	}
	result.Truncated = true
	result.Reason = reason
	return result, nil
}

func (s Service) currentFileName() string {
	now := time.Now()
	return "app-" + now.Format("2006-01-02") + ".log"
}

func category(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "generation"):
		return "generation"
	case strings.Contains(lower, "error"):
		return "error"
	case strings.Contains(lower, "api"):
		return "api"
	default:
		return "system"
	}
}
