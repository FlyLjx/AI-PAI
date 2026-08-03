package generation

import (
	"context"
	"errors"
	"testing"
	"time"

	"aipi-go/internal/database"
	"aipi-go/internal/settings"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestQueueAcquireScopeSerializesSameScope(t *testing.T) {
	queue := &Queue{}
	releaseFirst := queue.acquireScope("api-key:test", 1)

	acquiredSecond := make(chan func(), 1)
	go func() {
		acquiredSecond <- queue.acquireScope("api-key:test", 1)
	}()

	select {
	case releaseSecond := <-acquiredSecond:
		releaseSecond()
		releaseFirst()
		t.Fatal("second job acquired the same API key scope before the first job released it")
	case <-time.After(50 * time.Millisecond):
	}

	releaseFirst()
	select {
	case releaseSecond := <-acquiredSecond:
		releaseSecond()
	case <-time.After(time.Second):
		t.Fatal("second job did not acquire the API key scope after release")
	}
}

func TestGenerationTaskTimeoutDefaultsToFiveMinutes(t *testing.T) {
	queue := &Queue{}
	if got := queue.taskProcessingTimeout(); got != defaultTaskProcessingTimeout {
		t.Fatalf("task processing timeout = %s, want 5m", got)
	}
	if normalizeTaskProcessingError(context.DeadlineExceeded) != ErrTaskTimedOut {
		t.Fatal("deadline exceeded should use the task timeout error")
	}
}

func TestQueueTaskTimeoutCanBeRefreshed(t *testing.T) {
	queue := &Queue{}
	queue.SetTaskProcessingTimeout(10 * time.Minute)
	if got := queue.taskProcessingTimeout(); got != 10*time.Minute {
		t.Fatalf("task processing timeout = %s, want 10m", got)
	}
	if got := timeoutMessage(10 * time.Minute); got != "任务处理超时（超过 10 分钟）" {
		t.Fatalf("timeout message = %q", got)
	}
	if err := newTaskTimeoutError(10 * time.Minute); !errors.Is(err, ErrTaskTimedOut) || err.Error() != "任务处理超时（超过 10 分钟）" {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestQueueLoadsTaskTimeoutFromSettings(t *testing.T) {
	rawDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer rawDB.Close()
	mock.ExpectQuery("SELECT setting_key, setting_value FROM system_settings").
		WillReturnRows(sqlmock.NewRows([]string{"setting_key", "setting_value"}).
			AddRow("taskTimeoutMinutes", "10"))

	queue := &Queue{settings: settings.NewRepository(database.Wrap(rawDB))}
	if got := queue.taskProcessingTimeout(); got != 10*time.Minute {
		t.Fatalf("task processing timeout = %s, want 10m", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueueCancelStopsActiveTask(t *testing.T) {
	queue := &Queue{}
	ctx, cancel := context.WithCancel(context.Background())
	queue.registerActiveTask("task-1", cancel)

	if !queue.Cancel("task-1") {
		t.Fatal("active task was not canceled")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("active task context was not canceled")
	}

	queue.unregisterActiveTask("task-1")
	if queue.Cancel("task-1") {
		t.Fatal("inactive task should not report an active cancellation")
	}
}

func TestQueueAcquireScopeAllowsDifferentScopes(t *testing.T) {
	queue := &Queue{}
	releaseFirst := queue.acquireScope("api-key:first", 1)
	defer releaseFirst()

	acquiredSecond := make(chan func(), 1)
	go func() {
		acquiredSecond <- queue.acquireScope("api-key:second", 1)
	}()

	select {
	case releaseSecond := <-acquiredSecond:
		releaseSecond()
	case <-time.After(time.Second):
		t.Fatal("different API key scopes should not block each other")
	}
}

func TestQueueAcquireScopeHonorsScopeLimit(t *testing.T) {
	queue := &Queue{}
	releaseFirst := queue.acquireScope("api-key:limited", 2)
	defer releaseFirst()
	releaseSecond := queue.acquireScope("api-key:limited", 2)

	acquiredThird := make(chan func(), 1)
	go func() {
		acquiredThird <- queue.acquireScope("api-key:limited", 2)
	}()

	select {
	case releaseThird := <-acquiredThird:
		releaseThird()
		releaseSecond()
		t.Fatal("third job acquired the API key scope before a slot was released")
	case <-time.After(50 * time.Millisecond):
	}

	releaseSecond()
	select {
	case releaseThird := <-acquiredThird:
		releaseThird()
	case <-time.After(time.Second):
		t.Fatal("third job did not acquire the API key scope after a slot was released")
	}
}
