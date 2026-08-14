package generation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"aipi-go/internal/tasks"
	"github.com/redis/go-redis/v9"
)

const (
	generationConsumerGroup     = "ai-pai-workers"
	generationCompletionChannel = "generation:completed"
	outboxBatchSize             = 200
)

type streamJob struct {
	Job
	Stream    string
	MessageID string
}

type outboxRow struct {
	TaskID         string
	Shard          int
	Scope          sql.NullString
	Limit          int
	ResponseFormat sql.NullString
	Quality        sql.NullString
	Status         string
	MessageID      sql.NullString
}

type publishedOutboxRow struct {
	TaskID    string
	MessageID string
}

func (q *Queue) startRedisQueue() {
	q.redis = redis.NewClient(&redis.Options{
		Addr:         q.redisConfig.Addr,
		Password:     q.redisConfig.Password,
		DB:           q.redisConfig.DB,
		PoolSize:     maxInt(q.workers/4, 50),
		MinIdleConns: 10,
	})
	host, _ := os.Hostname()
	q.consumer = fmt.Sprintf("%s-%d-%d", host, os.Getpid(), time.Now().UnixNano())
	q.ensureConsumerGroups()
	go q.dispatchOutboxLoop()
	go q.consumeStreamLoop()
	go q.recoverPendingLoop()
	go q.subscribeCompletionLoop()
}

func (q *Queue) ensureConsumerGroups() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, stream := range q.streamNames() {
		err := q.redis.XGroupCreateMkStream(ctx, stream, generationConsumerGroup, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") && q.logger != nil {
			q.logger.Warn("generation stream group initialization failed", "stream", stream, "error", err)
		}
	}
}

func (q *Queue) streamNames() []string {
	streams := make([]string, q.queueConfig.StreamShards)
	for index := range streams {
		streams[index] = fmt.Sprintf("generation:tasks:%d", index)
	}
	return streams
}

func (q *Queue) dispatchOutboxLoop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		q.dispatchOutboxBatch()
		select {
		case <-q.shutdown:
			return
		case <-q.dispatchWake:
		case <-ticker.C:
		}
	}
}

func (q *Queue) dispatchOutboxBatch() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := q.service.db.QueryContext(ctx, `
		SELECT task_id, shard, concurrency_scope, concurrency_limit, response_format, quality, status, stream_message_id
		FROM generation_outbox
		WHERE (status = 'pending' AND next_attempt_at <= ?)
			OR (status = 'sent' AND updated_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?
	`, time.Now(), time.Now().Add(-2*time.Minute), outboxBatchSize)
	if err != nil {
		q.logRedisWarning("generation outbox lookup failed", err)
		return
	}
	defer rows.Close()
	batch := make([]outboxRow, 0, outboxBatchSize)
	for rows.Next() {
		var row outboxRow
		if err := rows.Scan(&row.TaskID, &row.Shard, &row.Scope, &row.Limit, &row.ResponseFormat, &row.Quality, &row.Status, &row.MessageID); err != nil {
			q.logRedisWarning("generation outbox scan failed", err)
			return
		}
		batch = append(batch, row)
	}
	publish := make([]outboxRow, 0, len(batch))
	for _, row := range batch {
		stream := fmt.Sprintf("generation:tasks:%d", row.Shard%q.queueConfig.StreamShards)
		if row.Status == "sent" && row.MessageID.Valid {
			messages, err := q.redis.XRangeN(ctx, stream, row.MessageID.String, row.MessageID.String, 1).Result()
			if err == nil && len(messages) == 1 {
				_, _ = q.service.db.ExecContext(context.Background(), `UPDATE generation_outbox SET updated_at = CURRENT_TIMESTAMP WHERE task_id = ? AND status = 'sent'`, row.TaskID)
				continue
			}
		}
		publish = append(publish, row)
	}
	q.publishOutboxRows(ctx, publish)
}

func (q *Queue) publishOutboxRows(ctx context.Context, rows []outboxRow) {
	if len(rows) == 0 {
		return
	}
	pipe := q.redis.Pipeline()
	commands := make([]*redis.StringCmd, len(rows))
	for index, row := range rows {
		commands[index] = pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: fmt.Sprintf("generation:tasks:%d", row.Shard%q.queueConfig.StreamShards),
			Values: map[string]any{
				"task_id": row.TaskID, "scope": row.Scope.String, "limit": row.Limit,
				"response_format": row.ResponseFormat.String, "quality": row.Quality.String,
			},
		})
	}
	_, _ = pipe.Exec(ctx)
	published := make([]publishedOutboxRow, 0, len(rows))
	failedIDs := make([]string, 0)
	var publishErr error
	for index, command := range commands {
		messageID, err := command.Result()
		if err != nil {
			failedIDs = append(failedIDs, rows[index].TaskID)
			publishErr = err
			continue
		}
		published = append(published, publishedOutboxRow{TaskID: rows[index].TaskID, MessageID: messageID})
	}
	if err := q.markOutboxPublished(ctx, published); err != nil {
		q.logRedisWarning("generation outbox publish state update failed", err)
	}
	if len(failedIDs) > 0 {
		q.markOutboxPublishFailed(ctx, failedIDs, publishErr)
		q.logRedisWarning("generation outbox publish failed", publishErr)
	}
}

func (q *Queue) markOutboxPublished(ctx context.Context, rows []publishedOutboxRow) error {
	if len(rows) == 0 {
		return nil
	}
	caseParts := make([]string, 0, len(rows))
	whereParts := make([]string, 0, len(rows))
	args := make([]any, 0, len(rows)*3)
	for _, row := range rows {
		caseParts = append(caseParts, "WHEN ? THEN ?")
		args = append(args, row.TaskID, row.MessageID)
		whereParts = append(whereParts, "?")
	}
	for _, row := range rows {
		args = append(args, row.TaskID)
	}
	_, err := q.service.db.ExecContext(ctx, `
		UPDATE generation_outbox
		SET status = 'sent', attempts = attempts + 1,
			stream_message_id = CASE task_id `+strings.Join(caseParts, " ")+` ELSE stream_message_id END,
			last_error = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE task_id IN (`+strings.Join(whereParts, ",")+`) AND status <> 'completed'
	`, args...)
	return err
}

func (q *Queue) markOutboxPublishFailed(ctx context.Context, taskIDs []string, publishErr error) {
	if len(taskIDs) == 0 {
		return
	}
	placeholders := make([]string, len(taskIDs))
	args := make([]any, 0, len(taskIDs)+3)
	message := "redis publish failed"
	if publishErr != nil {
		message = publishErr.Error()
	}
	args = append(args, message, time.Now().Add(time.Second))
	for index, taskID := range taskIDs {
		placeholders[index] = "?"
		args = append(args, taskID)
	}
	_, _ = q.service.db.ExecContext(ctx, `
		UPDATE generation_outbox
		SET attempts = attempts + 1, last_error = ?, next_attempt_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE task_id IN (`+strings.Join(placeholders, ",")+`) AND status <> 'completed'
	`, args...)
}

func (q *Queue) consumeStreamLoop() {
	streams := q.streamNames()
	readArgs := append(append([]string{}, streams...), make([]string, len(streams))...)
	for index := len(streams); index < len(readArgs); index++ {
		readArgs[index] = ">"
	}
	for {
		select {
		case <-q.shutdown:
			return
		default:
		}
		result, err := q.redis.XReadGroup(context.Background(), &redis.XReadGroupArgs{
			Group:    generationConsumerGroup,
			Consumer: q.consumer,
			Streams:  readArgs,
			Count:    200,
			Block:    2 * time.Second,
		}).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			q.logRedisWarning("generation stream read failed", err)
			if strings.Contains(err.Error(), "NOGROUP") {
				q.ensureConsumerGroups()
			}
			time.Sleep(time.Second)
			continue
		}
		q.queueStreamMessages(result)
	}
}

func (q *Queue) queueStreamMessages(streams []redis.XStream) {
	for _, stream := range streams {
		for _, message := range stream.Messages {
			job, ok := decodeStreamJob(stream.Stream, message)
			if !ok {
				_, _ = q.redis.XAck(context.Background(), stream.Stream, generationConsumerGroup, message.ID).Result()
				continue
			}
			select {
			case <-q.shutdown:
				return
			case q.incoming <- job:
			}
		}
	}
}

func decodeStreamJob(stream string, message redis.XMessage) (streamJob, bool) {
	taskID := strings.TrimSpace(fmt.Sprint(message.Values["task_id"]))
	if taskID == "" {
		return streamJob{}, false
	}
	limit, _ := strconv.Atoi(fmt.Sprint(message.Values["limit"]))
	return streamJob{
		Job: Job{
			TaskID:              taskID,
			ConcurrencyScope:    strings.TrimSpace(fmt.Sprint(message.Values["scope"])),
			ConcurrencyLimit:    maxInt(limit, 1),
			ImageResponseFormat: strings.TrimSpace(fmt.Sprint(message.Values["response_format"])),
			ImageQuality:        strings.TrimSpace(fmt.Sprint(message.Values["quality"])),
		},
		Stream:    stream,
		MessageID: message.ID,
	}, true
}

func (q *Queue) recoverPendingLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-q.shutdown:
			return
		case <-ticker.C:
			for _, stream := range q.streamNames() {
				messages, _, err := q.redis.XAutoClaim(context.Background(), &redis.XAutoClaimArgs{
					Stream: stream, Group: generationConsumerGroup, Consumer: q.consumer,
					MinIdle: 30 * time.Second, Start: "0-0", Count: 100,
				}).Result()
				if err == nil && len(messages) > 0 {
					q.queueStreamMessages([]redis.XStream{{Stream: stream, Messages: messages}})
				}
			}
		}
	}
}

func (q *Queue) finishStreamJob(queued streamJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, err := q.service.tasks.FindByID(ctx, queued.TaskID)
	if err != nil || !isTerminalTask(task) {
		return
	}
	_, _ = q.redis.XAck(ctx, queued.Stream, generationConsumerGroup, queued.MessageID).Result()
	_, _ = q.redis.XDel(ctx, queued.Stream, queued.MessageID).Result()
	_, _ = q.service.db.ExecContext(ctx, `DELETE FROM generation_outbox WHERE task_id = ?`, queued.TaskID)
	q.publishCompletion(queued.TaskID)
}

func isTerminalTask(task *tasks.Task) bool {
	return task != nil && (task.Status == tasks.StatusSuccess || task.Status == tasks.StatusFailed || task.Status == tasks.StatusCanceled)
}

func (q *Queue) publishCompletion(taskID string) {
	q.notifyWaiters(taskID)
	if q.redis != nil {
		_ = q.redis.Publish(context.Background(), generationCompletionChannel, taskID).Err()
	}
}

func (q *Queue) subscribeCompletionLoop() {
	pubsub := q.redis.Subscribe(context.Background(), generationCompletionChannel)
	defer pubsub.Close()
	for {
		select {
		case <-q.shutdown:
			return
		case message, ok := <-pubsub.Channel():
			if !ok {
				return
			}
			q.notifyWaiters(message.Payload)
		}
	}
}

func (q *Queue) Subscribe(taskID string) (<-chan struct{}, func()) {
	channel := make(chan struct{}, 1)
	taskID = strings.TrimSpace(taskID)
	q.mu.Lock()
	if q.waiters[taskID] == nil {
		q.waiters[taskID] = map[chan struct{}]struct{}{}
	}
	q.waiters[taskID][channel] = struct{}{}
	q.mu.Unlock()
	return channel, func() {
		q.mu.Lock()
		delete(q.waiters[taskID], channel)
		if len(q.waiters[taskID]) == 0 {
			delete(q.waiters, taskID)
		}
		q.mu.Unlock()
	}
}

func (q *Queue) notifyWaiters(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for channel := range q.waiters[strings.TrimSpace(taskID)] {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}

func (q *Queue) logRedisWarning(message string, err error) {
	if q.logger != nil {
		q.logger.Warn(message, "error", err)
	}
}

func (q *Queue) RecordAcceptedRequest(apiKeyID, taskID string, at time.Time) {
	if q == nil || q.redis == nil || strings.TrimSpace(apiKeyID) == "" || strings.TrimSpace(taskID) == "" {
		return
	}
	key := "generation:requests:" + strings.TrimSpace(apiKeyID)
	pipe := q.redis.Pipeline()
	pipe.ZAdd(context.Background(), key, redis.Z{Score: float64(at.UnixMilli()), Member: taskID})
	pipe.ZRemRangeByScore(context.Background(), key, "-inf", strconv.FormatInt(at.Add(-24*time.Hour).UnixMilli(), 10))
	pipe.Expire(context.Background(), key, 25*time.Hour)
	_, _ = pipe.Exec(context.Background())
}

func (q *Queue) RequestCountSince(ctx context.Context, apiKeyID string, since time.Time) (int, error) {
	if q == nil || q.redis == nil {
		return 0, errors.New("redis is not initialized")
	}
	count, err := q.redis.ZCount(ctx, "generation:requests:"+strings.TrimSpace(apiKeyID), strconv.FormatInt(since.UnixMilli(), 10), "+inf").Result()
	return int(count), err
}

func (q *Queue) Ping(ctx context.Context) error {
	if q == nil || q.redis == nil {
		return errors.New("redis is not initialized")
	}
	return q.redis.Ping(ctx).Err()
}

func (q *Queue) ShouldRecordIPEvidence(ctx context.Context, userID, apiKeyID, ip string) bool {
	if q == nil || q.redis == nil {
		return true
	}
	key := fmt.Sprintf("generation:ip-evidence:%s:%s:%s", strings.TrimSpace(userID), strings.TrimSpace(apiKeyID), strings.TrimSpace(ip))
	created, err := q.redis.SetNX(ctx, key, "1", time.Hour).Result()
	return err != nil || created
}
