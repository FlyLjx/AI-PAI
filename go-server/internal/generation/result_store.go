package generation

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

const (
	completedResultTTL      = 2 * time.Minute
	completedResultMaxItems = 512
	completedResultMaxBytes = 512 * 1024 * 1024
)

type completedResultItem struct {
	value     any
	size      int
	createdAt time.Time
}

// completedResults carries inline results only for the request that asked for
// base64. It avoids putting image bytes in the database while the worker and
// HTTP waiter share the same process.
var completedResults = struct {
	sync.Mutex
	items map[string]completedResultItem
	bytes int
}{items: map[string]completedResultItem{}}

func RememberResult(taskID string, result any) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || result == nil {
		return
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > completedResultMaxBytes {
		return
	}
	completedResults.Lock()
	if previous, ok := completedResults.items[taskID]; ok {
		completedResults.bytes -= previous.size
	}
	for len(completedResults.items) >= completedResultMaxItems || completedResults.bytes+len(encoded) > completedResultMaxBytes {
		evictOldestCompletedResult()
	}
	completedResults.items[taskID] = completedResultItem{value: result, size: len(encoded), createdAt: time.Now()}
	completedResults.bytes += len(encoded)
	completedResults.Unlock()
	time.AfterFunc(completedResultTTL, func() { ForgetResult(taskID) })
}

func ResultForTask(taskID string, fallback any) any {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fallback
	}
	completedResults.Lock()
	item, ok := completedResults.items[taskID]
	completedResults.Unlock()
	if ok {
		return item.value
	}
	return fallback
}

func ForgetResult(taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	completedResults.Lock()
	if item, ok := completedResults.items[taskID]; ok {
		completedResults.bytes -= item.size
	}
	delete(completedResults.items, taskID)
	completedResults.Unlock()
}

func evictOldestCompletedResult() {
	var oldestID string
	var oldestAt time.Time
	for taskID, item := range completedResults.items {
		if oldestID == "" || item.createdAt.Before(oldestAt) {
			oldestID, oldestAt = taskID, item.createdAt
		}
	}
	if oldestID == "" {
		return
	}
	completedResults.bytes -= completedResults.items[oldestID].size
	delete(completedResults.items, oldestID)
}
