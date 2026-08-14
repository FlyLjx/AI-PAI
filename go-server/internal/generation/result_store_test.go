package generation

import "testing"

func TestCompletedResultIsKeptInMemoryUntilForgotten(t *testing.T) {
	taskID := "base64-result-task"
	result := map[string]any{"data": []any{map[string]any{"b64_json": "IMAGE_BYTES"}}}
	RememberResult(taskID, result)
	t.Cleanup(func() { ForgetResult(taskID) })

	if got := ResultForTask(taskID, nil); got == nil {
		t.Fatal("inline result was not available to the waiting request")
	}
	ForgetResult(taskID)
	if got := ResultForTask(taskID, "fallback"); got != "fallback" {
		t.Fatalf("forgotten result = %#v, want fallback", got)
	}
}

func TestCompletedResultStoreTracksAndReleasesMemory(t *testing.T) {
	ForgetResult("bounded-result")
	RememberResult("bounded-result", map[string]any{"b64_json": "1234"})
	completedResults.Lock()
	bytesAfterStore := completedResults.bytes
	completedResults.Unlock()
	if bytesAfterStore <= 0 {
		t.Fatal("stored byte count was not tracked")
	}
	ForgetResult("bounded-result")
	completedResults.Lock()
	_, exists := completedResults.items["bounded-result"]
	completedResults.Unlock()
	if exists {
		t.Fatal("result was not released")
	}
}
