package generation

import (
	"context"
	"hash/fnv"
	"strings"

	"aipi-go/internal/database"
)

// InsertOutboxWithTx makes queue admission part of the same commit as the task.
func InsertOutboxWithTx(ctx context.Context, tx *database.Tx, job Job, shards int) error {
	if shards < 1 {
		shards = 1
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO generation_outbox
			(id, task_id, shard, concurrency_scope, concurrency_limit, response_format, quality, status, attempts, next_attempt_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, job.TaskID, job.TaskID, taskShard(job.TaskID, shards), nullableString(job.ConcurrencyScope), maxInt(job.ConcurrencyLimit, 1), nullableString(job.ImageResponseFormat), nullableString(job.ImageQuality))
	return err
}

func taskShard(taskID string, shards int) int {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.TrimSpace(taskID)))
	return int(hasher.Sum32() % uint32(shards))
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func maxInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
