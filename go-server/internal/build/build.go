package build

import "time"

var (
	Version = "go-dev"
	Commit  = "local"
	Time    = ""
)

func Info() map[string]string {
	buildTime := Time
	if buildTime == "" {
		buildTime = time.Now().Format(time.RFC3339)
	}
	return map[string]string{
		"runtime": "go",
		"version": Version,
		"commit":  Commit,
		"time":    buildTime,
	}
}
