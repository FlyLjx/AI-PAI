package httpserver

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"aipi-go/internal/config"
	"aipi-go/internal/database"
	"aipi-go/internal/generation"
)

func TestGenerationAdmissionPersistsThousandConcurrentRequests(t *testing.T) {
	if os.Getenv("AI_PAI_RUN_ADMISSION_LOAD_TEST") != "1" {
		t.Skip("set AI_PAI_RUN_ADMISSION_LOAD_TEST=1 to run the PostgreSQL load test")
	}
	cfg := config.Load()
	cfg.Database.Host = "127.0.0.1"
	cfg.Database.Port = 5432
	cfg.Database.MaxOpenConns = 120
	cfg.Database.MaxIdleConns = 60
	raw, err := database.Open(cfg.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	db := database.Wrap(raw)
	router := &Router{db: db}
	userID := fmt.Sprintf("load-user-%d", time.Now().UnixNano())
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO users (id, email, password_hash, role, status, credits, generation_reserved_credits)
		VALUES (?, ?, 'load-test', 'user', 'active', 100, 0)
	`, userID, userID+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM generation_outbox WHERE task_id LIKE ?`, userID+"-%")
		_, _ = db.ExecContext(context.Background(), `DELETE FROM generation_tasks WHERE id LIKE ?`, userID+"-%")
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = ?`, userID)
	}()

	started := time.Now()
	errorsChannel := make(chan error, 1000)
	var wait sync.WaitGroup
	for index := 0; index < 1000; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			taskID := fmt.Sprintf("%s-%04d", userID, index)
			err := router.withUserGenerationLock(ctx, userID, func(tx *database.Tx) error {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO generation_tasks
						(id, user_id, model_id, provider_id, capability, prompt, size_tier, size, output_format, quantity, subscription_quota_units, user_ip, cost_credits, status)
					VALUES (?, ?, 'load-model', 'load-provider', 'image', 'load test', '1k', '1024x1024', 'jpeg', 1, 0, '127.0.0.1', 0.001, 'queued')
				`, taskID, userID); err != nil {
					return err
				}
				if err := generation.InsertOutboxWithTx(ctx, tx, generation.Job{TaskID: taskID, ConcurrencyScope: "api-key:load", ConcurrencyLimit: 1000}, 64); err != nil {
					return err
				}
				return router.reserveGenerationBalance(ctx, tx, userID, 0.001, "balance")
			})
			if err != nil {
				errorsChannel <- err
			}
		}(index)
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Errorf("admission failed: %v", err)
	}
	if t.Failed() {
		return
	}
	var tasksCount, outboxCount int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM generation_tasks WHERE id LIKE ?`, userID+"-%").Scan(&tasksCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM generation_outbox WHERE task_id LIKE ?`, userID+"-%").Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if tasksCount != 1000 || outboxCount != 1000 {
		t.Fatalf("durable admissions: tasks=%d outbox=%d, want 1000/1000", tasksCount, outboxCount)
	}
	t.Logf("persisted 1000 concurrent admissions in %s", time.Since(started))
}
