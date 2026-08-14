package generation

import "testing"

func TestNextEligibleJobSkipsSaturatedScope(t *testing.T) {
	pending := []streamJob{
		{Job: Job{TaskID: "a-2", ConcurrencyScope: "a", ConcurrencyLimit: 1}},
		{Job: Job{TaskID: "b-1", ConcurrencyScope: "b", ConcurrencyLimit: 1}},
	}
	if index := nextEligibleJob(pending, map[string]int{"a": 1}); index != 1 {
		t.Fatalf("eligible index = %d, want 1", index)
	}
}

func TestNextEligibleJobHonorsThousandConcurrencyLimit(t *testing.T) {
	pending := []streamJob{{Job: Job{TaskID: "a-1000", ConcurrencyScope: "a", ConcurrencyLimit: 1000}}}
	if index := nextEligibleJob(pending, map[string]int{"a": 999}); index != 0 {
		t.Fatalf("eligible index = %d, want 0", index)
	}
	if index := nextEligibleJob(pending, map[string]int{"a": 1000}); index != -1 {
		t.Fatalf("eligible index = %d, want -1", index)
	}
}
