package database

import (
	"fmt"
	"strconv"
	"strings"
)

func Rebind(query string) string {
	if CurrentDialect() != DialectPostgres {
		return query
	}
	var builder strings.Builder
	index := 1
	for _, r := range query {
		if r == '?' {
			builder.WriteString("$")
			builder.WriteString(strconv.Itoa(index))
			index++
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func NowExpr() string {
	if CurrentDialect() == DialectPostgres {
		return "CURRENT_TIMESTAMP"
	}
	return "NOW()"
}

func CurrentDateExpr() string {
	if CurrentDialect() == DialectPostgres {
		return "CURRENT_DATE"
	}
	return "CURDATE()"
}

func DateAddDaysExpr(base string, days int) string {
	if CurrentDialect() == DialectPostgres {
		return fmt.Sprintf("(%s + INTERVAL '%d day')", base, days)
	}
	return fmt.Sprintf("DATE_ADD(%s, INTERVAL %d DAY)", base, days)
}

func DateSubHoursExpr(base string, hours int) string {
	if CurrentDialect() == DialectPostgres {
		return fmt.Sprintf("(%s - INTERVAL '%d hour')", base, hours)
	}
	return fmt.Sprintf("DATE_SUB(%s, INTERVAL %d HOUR)", base, hours)
}

func DateSubDaysExpr(base string, days int) string {
	if CurrentDialect() == DialectPostgres {
		return fmt.Sprintf("(%s - INTERVAL '%d day')", base, days)
	}
	return fmt.Sprintf("DATE_SUB(%s, INTERVAL %d DAY)", base, days)
}

func DateExpr(column string) string {
	if CurrentDialect() == DialectPostgres {
		return fmt.Sprintf("DATE(%s)", column)
	}
	return fmt.Sprintf("DATE(%s)", column)
}

func BoolCountExpr(condition string) string {
	if CurrentDialect() == DialectPostgres {
		return fmt.Sprintf("COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END),0)", condition)
	}
	return fmt.Sprintf("COALESCE(SUM(%s),0)", condition)
}

func GroupConcatExpr(expression string, orderBy string, separator string) string {
	if CurrentDialect() == DialectPostgres {
		return fmt.Sprintf("STRING_AGG(DISTINCT %s, %s ORDER BY %s)", expression, quoteLiteral(separator), orderBy)
	}
	return fmt.Sprintf("GROUP_CONCAT(DISTINCT %s ORDER BY %s SEPARATOR %s)", expression, orderBy, quoteLiteral(separator))
}

func JSONTextType() string {
	if CurrentDialect() == DialectPostgres {
		return "JSONB"
	}
	return "JSON"
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
