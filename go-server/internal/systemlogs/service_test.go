package systemlogs

import "testing"

func TestCategoryRecognizesErrorLogFileNames(t *testing.T) {
	values := []string{
		"dev-go-watch.err.log",
		"next-user-pages-3003.err.log",
		"service.stderr.log",
		"worker-fatal.log",
		"panic-output.log",
	}
	for _, value := range values {
		if got := category(value); got != "error" {
			t.Fatalf("category(%q) = %q, want error", value, got)
		}
	}
}
