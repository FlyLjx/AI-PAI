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
