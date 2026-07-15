package database

import (
	"strings"
)

type Dialect string

const (
	DialectMySQL    Dialect = "mysql"
	DialectPostgres Dialect = "postgres"
)

var currentDialect = DialectMySQL

func SetDialect(driver string) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case string(DialectPostgres), "pgx":
		currentDialect = DialectPostgres
	default:
		currentDialect = DialectMySQL
	}
}

func CurrentDialect() Dialect {
	return currentDialect
}
