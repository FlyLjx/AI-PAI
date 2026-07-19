package requestmonitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"aipi-go/internal/database"
)

type Recorder struct {
	repository *Repository
	logger     *slog.Logger
	queue      chan Record
}

func NewRecorder(db *database.DB, logger *slog.Logger) *Recorder {
	recorder := &Recorder{
		repository: NewRepository(db),
		logger:     logger,
		queue:      make(chan Record, 1024),
	}
	go recorder.run()
	return recorder
}

func (recorder *Recorder) Submit(record Record) {
	if recorder == nil {
		return
	}
	if record.ID == "" {
		record.ID = newRecordID()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	select {
	case recorder.queue <- record:
	default:
		if recorder.logger != nil {
			recorder.logger.Warn("request monitor queue full; record dropped", "path", record.Path)
		}
	}
}

func (recorder *Recorder) run() {
	for record := range recorder.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := recorder.repository.Insert(ctx, record)
		cancel()
		if err != nil && recorder.logger != nil {
			recorder.logger.Error("request monitor insert failed", "path", record.Path, "error", err)
		}
	}
}

func newRecordID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("request-%d", time.Now().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	value := hex.EncodeToString(bytes[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", value[0:8], value[8:12], value[12:16], value[16:20], value[20:32])
}
