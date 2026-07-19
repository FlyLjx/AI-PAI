package requestmonitor

import (
	"strings"
	"testing"

	"aipi-go/internal/database"
)

func TestTrendBucketExpressionUsesDatabaseLocalTimeForPostgres(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("postgres")
	defer database.SetDialect(string(previousDialect))

	expression := trendBucketExpression(300)
	if !strings.Contains(expression, "created_at AT TIME ZONE 'Asia/Shanghai'") {
		t.Fatalf("PostgreSQL bucket expression does not interpret local database time: %s", expression)
	}
	if !strings.Contains(expression, "/ 300") || !strings.HasSuffix(expression, "* 300") {
		t.Fatalf("PostgreSQL bucket expression uses the wrong interval: %s", expression)
	}
}

func TestTrendBucketExpressionKeepsMySQLSessionTimezone(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	expression := trendBucketExpression(3600)
	if expression != "FLOOR(UNIX_TIMESTAMP(created_at) / 3600) * 3600" {
		t.Fatalf("unexpected MySQL bucket expression: %s", expression)
	}
}
