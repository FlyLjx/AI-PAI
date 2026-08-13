package generation

import (
	"strings"
	"sync"
	"time"
)

const completedResultTTL = 35 * time.Minute

// completedResults carries inline results only for the request that asked for
// base64. It avoids putting image bytes in the database while the worker and
// HTTP waiter share the same process.
var completedResults = struct {
	sync.Mutex
	items map[string]any
}{items: map[string]any{}}

func RememberResult(taskID string, result any) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || result == nil {
		return
	}
	completedResults.Lock()
	completedResults.items[taskID] = result
	completedResults.Unlock()
	time.AfterFunc(completedResultTTL, func() { ForgetResult(taskID) })
}

func ResultForTask(taskID string, fallback any) any {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fallback
	}
	completedResults.Lock()
	result, ok := completedResults.items[taskID]
	completedResults.Unlock()
	if ok {
		return result
	}
	return fallback
}

func ForgetResult(taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	completedResults.Lock()
	delete(completedResults.items, taskID)
	completedResults.Unlock()
}
