package generation

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"aipi-go/internal/database"
	"aipi-go/internal/settings"
	"aipi-go/internal/tasks"
)

const (
	defaultTaskProcessingTimeout = settings.DefaultTaskTimeoutMinutes * time.Minute
	taskTimeoutSettingsCacheTTL  = 15 * time.Second
	taskTimeoutSweepInterval     = 30 * time.Second
	taskTimeoutSweepBatchSize    = 500
	taskTimeoutSweepMaxBatches   = 10
)

type Queue struct {
	jobs      chan Job
	workers   int
	unlimited bool
	service   *Service
	logger    *slog.Logger
	settings  *settings.Repository
	started   bool
	mu        sync.Mutex
	shutdown  chan struct{}
	scopes    map[string]*scopeLimiter
	active    map[string]context.CancelFunc
	paused    bool
	pausedAt  time.Time
	timeoutMu sync.RWMutex
	timeout   time.Duration
	timeoutAt time.Time
}

type Job struct {
	TaskID              string
	ConcurrencyScope    string
	ConcurrencyLimit    int
	ImageResponseFormat string
	ImageQuality        string
}

type scopeLimiter struct {
	active int
	limit  int
	cond   *sync.Cond
}

func NewQueue(db *database.DB, logger *slog.Logger, workers int) *Queue {
	unlimited := workers <= 0
	bufferSize := 1024
	if !unlimited {
		bufferSize = workers * 4
	}
	return &Queue{
		jobs:      make(chan Job, bufferSize),
		workers:   workers,
		unlimited: unlimited,
		service:   NewService(db, logger),
		logger:    logger,
		settings:  settings.NewRepository(db),
		shutdown:  make(chan struct{}),
	}
}

func (q *Queue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started {
		return
	}
	q.started = true
	go q.watchTimedOutTasks()
	if q.unlimited {
		return
	}
	for index := 0; index < q.workers; index++ {
		go q.worker(index + 1)
	}
}

func (q *Queue) Enqueue(taskID string) {
	q.enqueue(Job{TaskID: taskID})
}

func (q *Queue) EnqueueScoped(taskID string, scope string, limit int) {
	q.enqueue(Job{
		TaskID:           taskID,
		ConcurrencyScope: strings.TrimSpace(scope),
		ConcurrencyLimit: limit,
	})
}

func (q *Queue) EnqueueScopedWithOptions(taskID string, scope string, limit int, options ProcessOptions) {
	q.enqueue(Job{
		TaskID:              taskID,
		ConcurrencyScope:    strings.TrimSpace(scope),
		ConcurrencyLimit:    limit,
		ImageResponseFormat: strings.TrimSpace(options.ImageResponseFormat),
		ImageQuality:        strings.TrimSpace(options.ImageQuality),
	})
}

func (q *Queue) enqueue(job Job) {
	q.Start()
	if q.unlimited {
		go q.process(job, "unlimited")
		return
	}
	q.jobs <- job
}

func APIKeyConcurrencyScope(apiKeyID string) string {
	apiKeyID = strings.TrimSpace(apiKeyID)
	if apiKeyID == "" {
		return ""
	}
	return "api-key:" + apiKeyID
}

func (q *Queue) worker(workerID int) {
	for {
		select {
		case <-q.shutdown:
			return
		case job := <-q.jobs:
			q.process(job, workerID)
		}
	}
}

func (q *Queue) process(job Job, workerID any) {
	if err := q.waitWhilePaused(); err != nil {
		return
	}
	release := q.acquireScope(job.ConcurrencyScope, job.ConcurrencyLimit)
	defer release()

	processingTimeout := q.taskProcessingTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), processingTimeout)
	q.registerActiveTask(job.TaskID, cancel)
	defer q.unregisterActiveTask(job.TaskID)
	err := q.service.ProcessWithOptions(ctx, job.TaskID, ProcessOptions{
		ImageResponseFormat: job.ImageResponseFormat,
		ImageQuality:        job.ImageQuality,
		ProcessingTimeout:   processingTimeout,
	})
	cancel()
	if err != nil {
		q.logger.Error("generation worker failed", "worker", workerID, "taskId", job.TaskID, "error", err)
	}
}

func (q *Queue) SetPaused(paused bool) {
	q.mu.Lock()
	if q.paused == paused {
		q.mu.Unlock()
		return
	}
	q.paused = paused
	if paused {
		q.pausedAt = time.Now()
	} else {
		q.pausedAt = time.Time{}
	}
	q.mu.Unlock()
}

func (q *Queue) Paused() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused
}

func (q *Queue) PauseSnapshot() (bool, time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused, q.pausedAt
}

func (q *Queue) waitWhilePaused() error {
	for {
		q.mu.Lock()
		paused := q.paused
		q.mu.Unlock()
		if !paused {
			return nil
		}
		select {
		case <-q.shutdown:
			return context.Canceled
		case <-time.After(time.Second):
		}
	}
}

func (q *Queue) watchTimedOutTasks() {
	q.sweepTimedOutTasks()
	ticker := time.NewTicker(taskTimeoutSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-q.shutdown:
			return
		case <-ticker.C:
			q.sweepTimedOutTasks()
		}
	}
}

func (q *Queue) sweepTimedOutTasks() {
	processingTimeout := q.taskProcessingTimeout()
	for batch := 0; batch < taskTimeoutSweepMaxBatches; batch++ {
		now := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		var ids []string
		var err error
		if q.Paused() {
			ids, err = q.service.FailTimedOutProcessing(ctx, now.Add(-processingTimeout), now, timeoutMessage(processingTimeout), taskTimeoutSweepBatchSize)
		} else {
			ids, err = q.service.FailTimedOut(ctx, now.Add(-processingTimeout), now, timeoutMessage(processingTimeout), taskTimeoutSweepBatchSize)
		}
		cancel()
		for _, id := range ids {
			q.Cancel(id)
		}
		if err != nil {
			q.logger.Error("generation timeout sweep failed", "error", err)
			return
		}
		if len(ids) < taskTimeoutSweepBatchSize {
			return
		}
	}
}

func (q *Queue) taskProcessingTimeout() time.Duration {
	q.timeoutMu.RLock()
	timeout, loadedAt := q.timeout, q.timeoutAt
	q.timeoutMu.RUnlock()
	if timeout > 0 && time.Since(loadedAt) < taskTimeoutSettingsCacheTTL {
		return timeout
	}
	if q.settings != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		values, err := q.settings.Get(ctx)
		cancel()
		if err == nil {
			timeout = settings.TaskTimeout(values)
			q.SetTaskProcessingTimeout(timeout)
			return timeout
		}
		if q.logger != nil {
			q.logger.Warn("task timeout settings lookup failed", "error", err)
		}
	}
	if timeout <= 0 {
		timeout = defaultTaskProcessingTimeout
	}
	return timeout
}

// SetTaskProcessingTimeout refreshes the queue deadline immediately after an
// administrator changes the setting. The periodic lookup remains as a safety
// net for changes made outside the admin API.
func (q *Queue) SetTaskProcessingTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = defaultTaskProcessingTimeout
	}
	q.timeoutMu.Lock()
	q.timeout = timeout
	q.timeoutAt = time.Now()
	q.timeoutMu.Unlock()
}

func timeoutMessage(timeout time.Duration) string {
	minutes := int(timeout / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	return "任务处理超时（超过 " + strconv.Itoa(minutes) + " 分钟）"
}

func (q *Queue) TouchWaitingTasks(ctx context.Context) error {
	if q == nil || q.service == nil {
		return nil
	}
	return q.service.TouchWaitingTasks(ctx)
}

func (q *Queue) Cancel(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false
	}
	q.mu.Lock()
	cancel := q.active[taskID]
	q.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (q *Queue) registerActiveTask(taskID string, cancel context.CancelFunc) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || cancel == nil {
		return
	}
	q.mu.Lock()
	if q.active == nil {
		q.active = map[string]context.CancelFunc{}
	}
	q.active[taskID] = cancel
	q.mu.Unlock()
}

func (q *Queue) unregisterActiveTask(taskID string) {
	q.mu.Lock()
	delete(q.active, strings.TrimSpace(taskID))
	q.mu.Unlock()
}

func (q *Queue) acquireScope(scope string, limit int) func() {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return func() {}
	}
	if limit < 1 {
		limit = 1
	}
	limiter := q.scopeLimiter(scope, limit)
	q.mu.Lock()
	if limit > limiter.limit {
		limiter.cond.Broadcast()
	}
	limiter.limit = limit
	for limiter.active >= limiter.limit {
		limiter.cond.Wait()
	}
	limiter.active++
	q.mu.Unlock()
	return func() {
		q.mu.Lock()
		if limiter.active > 0 {
			limiter.active--
		}
		limiter.cond.Broadcast()
		q.mu.Unlock()
	}
}

func (q *Queue) scopeLimiter(scope string, limit int) *scopeLimiter {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.scopes == nil {
		q.scopes = map[string]*scopeLimiter{}
	}
	if limiter, ok := q.scopes[scope]; ok {
		return limiter
	}
	limiter := &scopeLimiter{limit: limit}
	limiter.cond = sync.NewCond(&q.mu)
	q.scopes[scope] = limiter
	return limiter
}

type Service struct {
	db     *database.DB
	logger *slog.Logger
	tasks  *tasks.Repository
}

func NewService(db *database.DB, logger *slog.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
		tasks:  tasks.NewRepository(db),
	}
}
