package generation

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aipi-go/internal/database"
	"aipi-go/internal/tasks"
	"aipi-go/internal/users"
)

type Queue struct {
	jobs      chan Job
	workers   int
	unlimited bool
	service   *Service
	logger    *slog.Logger
	started   bool
	mu        sync.Mutex
	shutdown  chan struct{}
	scopes    map[string]*scopeLimiter
}

type Job struct {
	TaskID           string
	ConcurrencyScope string
	ConcurrencyLimit int
}

type scopeLimiter struct {
	active int
	limit  int
	cond   *sync.Cond
}

func NewQueue(db *database.DB, logger *slog.Logger, workers int, hub *tasks.Hub, userHub *users.Hub) *Queue {
	unlimited := workers <= 0
	bufferSize := 1024
	if !unlimited {
		bufferSize = workers * 4
	}
	return &Queue{
		jobs:      make(chan Job, bufferSize),
		workers:   workers,
		unlimited: unlimited,
		service:   NewService(db, logger, hub, userHub),
		logger:    logger,
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
	release := q.acquireScope(job.ConcurrencyScope, job.ConcurrencyLimit)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	err := q.service.Process(ctx, job.TaskID)
	cancel()
	if err != nil {
		q.logger.Error("generation worker failed", "worker", workerID, "taskId", job.TaskID, "error", err)
	}
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
	db      *database.DB
	logger  *slog.Logger
	tasks   *tasks.Repository
	hub     *tasks.Hub
	userHub *users.Hub
}

func NewService(db *database.DB, logger *slog.Logger, hub *tasks.Hub, userHub *users.Hub) *Service {
	return &Service{
		db:      db,
		logger:  logger,
		tasks:   tasks.NewRepository(db),
		hub:     hub,
		userHub: userHub,
	}
}
