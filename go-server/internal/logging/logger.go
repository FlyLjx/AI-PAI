package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func New(level string, logDir string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	if strings.TrimSpace(logDir) == "" {
		logDir = "logs"
	}
	handler := &textFileHandler{
		level:  slogLevel,
		logDir: logDir,
		out:    os.Stdout,
		state:  &fileState{},
	}
	logger := slog.New(handler)
	logger.Info("file logger enabled", "logDir", handler.absLogDir())
	return logger
}

type textFileHandler struct {
	level  slog.Level
	logDir string
	out    *os.File
	attrs  []slog.Attr
	group  string
	state  *fileState
}

type fileState struct {
	mu       sync.Mutex
	fileDate string
	file     *os.File
}

func (h *textFileHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *textFileHandler) Handle(_ context.Context, record slog.Record) error {
	if !h.Enabled(context.Background(), record.Level) {
		return nil
	}
	line := h.format(record)
	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	if h.out != nil {
		_, _ = h.out.WriteString(line)
	}
	file, err := h.openFile(record.Time)
	if err != nil {
		return err
	}
	_, err = file.WriteString(line)
	return err
}

func (h *textFileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := h.clone()
	next.attrs = append(next.attrs, attrs...)
	return next
}

func (h *textFileHandler) WithGroup(name string) slog.Handler {
	next := h.clone()
	if next.group != "" {
		next.group += "." + name
	} else {
		next.group = name
	}
	return next
}

func (h *textFileHandler) format(record slog.Record) string {
	when := record.Time
	if when.IsZero() {
		when = time.Now()
	}
	attrs := map[string]any{}
	for _, attr := range h.attrs {
		h.addAttr(attrs, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		h.addAttr(attrs, attr)
		return true
	})
	message := strings.TrimSpace(record.Message)
	if message == "" {
		message = "runtime"
	}
	var suffix string
	if len(attrs) > 0 {
		if bytes, err := json.Marshal(attrs); err == nil {
			suffix = " " + string(bytes)
		}
	}
	return fmt.Sprintf("%s [%s] %s%s\n", when.Format("2006-01-02 15:04:05.000"), strings.ToUpper(record.Level.String()), message, suffix)
}

func (h *textFileHandler) openFile(now time.Time) (*os.File, error) {
	if now.IsZero() {
		now = time.Now()
	}
	date := now.Format("2006-01-02")
	if h.state.file != nil && h.state.fileDate == date {
		return h.state.file, nil
	}
	if err := os.MkdirAll(h.logDir, 0755); err != nil {
		return nil, err
	}
	if h.state.file != nil {
		_ = h.state.file.Close()
	}
	path := filepath.Join(h.logDir, "app-"+date+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	h.state.file = file
	h.state.fileDate = date
	return file, nil
}

func (h *textFileHandler) absLogDir() string {
	path, err := filepath.Abs(h.logDir)
	if err != nil {
		return h.logDir
	}
	return path
}

func (h *textFileHandler) clone() *textFileHandler {
	return &textFileHandler{
		level:  h.level,
		logDir: h.logDir,
		out:    h.out,
		attrs:  append([]slog.Attr{}, h.attrs...),
		group:  h.group,
		state:  h.state,
	}
}

func (h *textFileHandler) addAttr(attrs map[string]any, attr slog.Attr) {
	key := attr.Key
	if h.group != "" {
		key = h.group + "." + key
	}
	attrs[key] = slogValue(attr.Value)
}

func slogValue(value slog.Value) any {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindBool:
		return value.Bool()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339)
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindGroup:
		group := map[string]any{}
		for _, attr := range value.Group() {
			group[attr.Key] = slogValue(attr.Value)
		}
		return group
	case slog.KindAny:
		raw := value.Any()
		if err, ok := raw.(error); ok {
			return err.Error()
		}
		return raw
	default:
		return value.String()
	}
}
