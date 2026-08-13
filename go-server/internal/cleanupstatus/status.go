package cleanupstatus

import (
	"context"
	"sync"
	"time"
)

type Status struct {
	State                   string     `json:"state"`
	Phase                   string     `json:"phase"`
	TotalRows               int64      `json:"totalRows"`
	ProcessedRows           int64      `json:"processedRows"`
	ReleasedDatabaseBytes   int64      `json:"releasedDatabaseBytes"`
	DeletedCacheDirectories int64      `json:"deletedCacheDirectories"`
	ReleasedCacheBytes      int64      `json:"releasedCacheBytes"`
	StartedAt               *time.Time `json:"startedAt,omitempty"`
	UpdatedAt               *time.Time `json:"updatedAt,omitempty"`
	CompletedAt             *time.Time `json:"completedAt,omitempty"`
	Error                   string     `json:"error,omitempty"`
}

type Tracker struct {
	mu     sync.RWMutex
	status Status
}

func New() *Tracker { return &Tracker{status: Status{State: "idle"}} }
func (t *Tracker) Start(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = Status{State: "running", Phase: "database", StartedAt: &now, UpdatedAt: &now}
}
func (t *Tracker) SetPhase(phase string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Phase = phase
	now := time.Now()
	t.status.UpdatedAt = &now
}
func (t *Tracker) SetTotalRows(total int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.TotalRows = total
	now := time.Now()
	t.status.UpdatedAt = &now
}
func (t *Tracker) AddDatabase(processed, released int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.ProcessedRows += processed
	t.status.ReleasedDatabaseBytes += released
	now := time.Now()
	t.status.UpdatedAt = &now
}
func (t *Tracker) AddCache(directories, released int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.DeletedCacheDirectories += directories
	t.status.ReleasedCacheBytes += released
	now := time.Now()
	t.status.UpdatedAt = &now
}
func (t *Tracker) Complete(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.State = "completed"
	t.status.Phase = "done"
	t.status.CompletedAt = &now
	t.status.UpdatedAt = &now
}
func (t *Tracker) Fail(err error, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.State = "failed"
	t.status.Error = err.Error()
	t.status.CompletedAt = &now
	t.status.UpdatedAt = &now
}
func (t *Tracker) Cancel(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.State = "canceled"
	t.status.Phase = "stopped"
	t.status.CompletedAt = &now
	t.status.UpdatedAt = &now
}
func (t *Tracker) Snapshot() Status { t.mu.RLock(); defer t.mu.RUnlock(); return t.status }

type contextKey struct{}

func WithTracker(ctx context.Context, tracker *Tracker) context.Context {
	return context.WithValue(ctx, contextKey{}, tracker)
}
func FromContext(ctx context.Context) *Tracker {
	tracker, _ := ctx.Value(contextKey{}).(*Tracker)
	return tracker
}
