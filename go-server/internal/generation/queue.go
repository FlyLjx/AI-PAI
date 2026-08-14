package generation

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"aipi-go/internal/config"
	"aipi-go/internal/database"
	"aipi-go/internal/settings"
	"aipi-go/internal/tasks"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTaskProcessingTimeout = settings.DefaultTaskTimeoutMinutes * time.Minute
	taskTimeoutSettingsCacheTTL  = 15 * time.Second
	taskTimeoutSweepInterval     = 30 * time.Second
	taskTimeoutSweepBatchSize    = 500
	taskTimeoutSweepMaxBatches   = 10
)

type Queue struct {
	jobs         chan streamJob
	incoming     chan streamJob
	releases     chan string
	workers      int
	service      *Service
	logger       *slog.Logger
	settings     *settings.Repository
	started      bool
	mu           sync.Mutex
	shutdown     chan struct{}
	active       map[string]context.CancelFunc
	paused       bool
	pausedAt     time.Time
	timeoutMu    sync.RWMutex
	timeout      time.Duration
	timeoutAt    time.Time
	redis        *redis.Client
	redisConfig  config.RedisConfig
	queueConfig  config.GenerationConfig
	dispatchWake chan struct{}
	consumer     string
	waiters      map[string]map[chan struct{}]struct{}
}

type Job struct {
	TaskID              string
	ConcurrencyScope    string
	ConcurrencyLimit    int
	ImageResponseFormat string
	ImageQuality        string
}

func NewQueue(db *database.DB, logger *slog.Logger, redisConfig config.RedisConfig, queueConfig config.GenerationConfig) *Queue {
	workers := queueConfig.Workers
	if workers < 1 {
		workers = 1
	}
	if queueConfig.StreamShards < 1 {
		queueConfig.StreamShards = 1
	}
	bufferSize := workers * 2
	return &Queue{
		jobs:         make(chan streamJob, bufferSize),
		incoming:     make(chan streamJob, bufferSize),
		releases:     make(chan string, bufferSize),
		workers:      workers,
		service:      NewService(db, logger),
		logger:       logger,
		settings:     settings.NewRepository(db),
		shutdown:     make(chan struct{}),
		redisConfig:  redisConfig,
		queueConfig:  queueConfig,
		dispatchWake: make(chan struct{}, 1),
		waiters:      map[string]map[chan struct{}]struct{}{},
	}
}

func (q *Queue) Start() {
	q.mu.Lock()
	if q.started {
		q.mu.Unlock()
		return
	}
	q.started = true
	q.mu.Unlock()
	q.startRedisQueue()
	go q.watchTimedOutTasks()
	go q.scheduleJobs()
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
	select {
	case q.dispatchWake <- struct{}{}:
	default:
	}
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
		case queued := <-q.jobs:
			q.runStreamJob(queued, workerID)
			select {
			case q.releases <- strings.TrimSpace(queued.ConcurrencyScope):
			case <-q.shutdown:
				return
			}
		}
	}
}

func (q *Queue) runStreamJob(queued streamJob, workerID int) {
	defer func() {
		if recovered := recover(); recovered != nil && q.logger != nil {
			q.logger.Error("generation worker panic recovered", "worker", workerID, "taskId", queued.TaskID, "panic", recovered)
		}
	}()
	q.process(queued.Job, workerID)
	q.finishStreamJob(queued)
}

func (q *Queue) process(job Job, workerID any) {
	if err := q.waitWhilePaused(); err != nil {
		return
	}
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
			q.publishCompletion(id)
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
