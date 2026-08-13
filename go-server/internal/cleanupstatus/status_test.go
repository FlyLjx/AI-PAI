package cleanupstatus

import (
	"sync"
	"testing"
	"time"
)

func TestTrackerCollectsCleanupProgressConcurrently(t *testing.T) {
	tracker := New()
	started := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	tracker.Start(started)
	tracker.SetTotalRows(100)

	var workers sync.WaitGroup
	for range 10 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			tracker.AddDatabase(10, 200)
			tracker.AddCache(1, 300)
		}()
	}
	workers.Wait()
	tracker.SetPhase("cache")
	tracker.Complete(started.Add(time.Minute))

	status := tracker.Snapshot()
	if status.State != "completed" || status.Phase != "done" {
		t.Fatalf("state = %q, phase = %q", status.State, status.Phase)
	}
	if status.TotalRows != 100 || status.ProcessedRows != 100 {
		t.Fatalf("row progress = %d/%d", status.ProcessedRows, status.TotalRows)
	}
	if status.ReleasedDatabaseBytes != 2000 || status.DeletedCacheDirectories != 10 || status.ReleasedCacheBytes != 3000 {
		t.Fatalf("unexpected cleanup totals: %#v", status)
	}
}
