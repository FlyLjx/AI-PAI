package imagecache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxCachedImageBytes = 80 * 1024 * 1024

var ErrInvalidImage = errors.New("invalid image response")

type CachedImage struct {
	Path        string
	ContentType string
}

func StoreTaskImage(ctx context.Context, taskID string, index int, imageURL string) (CachedImage, error) {
	taskID = safePathSegment(taskID)
	imageURL = strings.TrimSpace(imageURL)
	if taskID == "" || index < 0 || imageURL == "" || strings.HasPrefix(strings.ToLower(imageURL), "data:image/") {
		return CachedImage{}, ErrInvalidImage
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return CachedImage{}, err
	}
	request.Header.Set("User-Agent", "AI-PAI image cache")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return CachedImage{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return CachedImage{}, ErrInvalidImage
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxCachedImageBytes+1))
	if err != nil {
		return CachedImage{}, err
	}
	if len(body) == 0 || len(body) > maxCachedImageBytes {
		return CachedImage{}, ErrInvalidImage
	}
	contentType := imageContentType(response.Header.Get("Content-Type"), body)
	if !strings.HasPrefix(contentType, "image/") {
		return CachedImage{}, ErrInvalidImage
	}
	extension := imageExtension(contentType, imageURL)
	if extension == "" {
		extension = ".img"
	}
	directory := filepath.Join(Directory(), taskID)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return CachedImage{}, err
	}
	path := filepath.Join(directory, strconv.Itoa(index)+extension)
	temp, err := os.CreateTemp(directory, "."+strconv.Itoa(index)+"-*.tmp")
	if err != nil {
		return CachedImage{}, err
	}
	tempPath := temp.Name()
	_, writeErr := io.Copy(temp, bytes.NewReader(body))
	closeErr := temp.Close()
	if writeErr != nil {
		_ = os.Remove(tempPath)
		return CachedImage{}, writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return CachedImage{}, closeErr
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return CachedImage{}, err
	}
	return CachedImage{Path: path, ContentType: contentType}, nil
}

func FindTaskImage(taskID string, index int) (CachedImage, bool) {
	taskID = safePathSegment(taskID)
	if taskID == "" || index < 0 {
		return CachedImage{}, false
	}
	pattern := filepath.Join(Directory(), taskID, strconv.Itoa(index)+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return CachedImage{}, false
	}
	for _, path := range matches {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() && info.Size() > 0 {
			return CachedImage{Path: path, ContentType: contentTypeFromExtension(path)}, true
		}
	}
	return CachedImage{}, false
}

func Directory() string {
	if value := strings.TrimSpace(os.Getenv("IMAGE_CACHE_DIR")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("LOG_DIR")); value != "" {
		return filepath.Join(value, "task-images")
	}
	return filepath.Join("logs", "task-images")
}

func CacheKey(taskID string, index int) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(taskID) + ":" + strconv.Itoa(index)))
	return hex.EncodeToString(sum[:8])
}

func safePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func imageContentType(header string, body []byte) string {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(header, ";")[0]))
	if strings.HasPrefix(contentType, "image/") {
		return contentType
	}
	return strings.ToLower(http.DetectContentType(body))
}

func imageExtension(contentType string, imageURL string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	parsed, err := url.Parse(strings.TrimSpace(imageURL))
	if err != nil {
		return ""
	}
	extension := strings.ToLower(filepath.Ext(parsed.Path))
	switch extension {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		if extension == ".jpeg" {
			return ".jpg"
		}
		return extension
	default:
		return ""
	}
}

func contentTypeFromExtension(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func StoreTaskImageWithTimeout(parent context.Context, taskID string, index int, imageURL string) (CachedImage, error) {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	var lastErr error
	delays := []time.Duration{0, 600 * time.Millisecond, 1500 * time.Millisecond}
	for _, delay := range delays {
		if delay > 0 {
			select {
			case <-ctx.Done():
				return CachedImage{}, ctx.Err()
			case <-time.After(delay):
			}
		}
		cached, err := StoreTaskImage(ctx, taskID, index, imageURL)
		if err == nil {
			return cached, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return CachedImage{}, ctx.Err()
		}
	}
	if lastErr == nil {
		lastErr = ErrInvalidImage
	}
	return CachedImage{}, lastErr
}
