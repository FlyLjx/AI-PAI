package tasks

import "testing"

func TestToPublicUsesStoredResultURLsWithoutResultJSON(t *testing.T) {
	task := &Task{
		ID:               "task-stored-url",
		Status:           StatusSuccess,
		Quantity:         1,
		StoredResultURLs: []string{"https://example.test/result.png"},
	}
	public := ToPublic(task)
	if len(public.DirectResultURLs) != 1 || public.DirectResultURLs[0] != "https://example.test/result.png" {
		t.Fatalf("unexpected direct URLs: %+v", public.DirectResultURLs)
	}
	if len(public.ResultURLs) != 1 || public.ResultURLs[0] != "https://example.test/result.png" {
		t.Fatalf("unexpected result URLs: %+v", public.ResultURLs)
	}
}
