package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	"aipi-go/internal/appclock"
	"aipi-go/internal/config"
)

func Open(cfg config.DatabaseConfig) (*sql.DB, error) {
	appclock.ConfigureDefault()
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = "mysql"
	}
	SetDialect(driver)
	dsn, err := dsnForDriver(driver, cfg)
	if err != nil {
		return nil, err
	}
	sqlDriverName := driver
	if driver == "postgres" {
		sqlDriverName = "pgx"
	}
	db, err := sql.Open(sqlDriverName, dsn)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

func dsnForDriver(driver string, cfg config.DatabaseConfig) (string, error) {
	switch driver {
	case "mysql":
		mysqlConfig := mysql.NewConfig()
		mysqlConfig.User = cfg.User
		mysqlConfig.Passwd = cfg.Password
		mysqlConfig.Net = "tcp"
		mysqlConfig.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		mysqlConfig.DBName = cfg.Name
		mysqlConfig.ParseTime = true
		mysqlConfig.Loc = appclock.ConfigureDefault()
		mysqlConfig.Params = map[string]string{
			"charset":   "utf8mb4,utf8",
			"time_zone": "'" + appclock.DefaultDatabaseTimeZone + "'",
		}
		return mysqlConfig.FormatDSN(), nil
	case "postgres", "pgx":
		query := url.Values{}
		query.Set("sslmode", defaultString(cfg.SSLMode, "disable"))
		query.Set("application_name", "ai-pai")
		query.Set("timezone", appclock.DefaultLocationName)
		return fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?%s",
			url.QueryEscape(cfg.User),
			url.QueryEscape(cfg.Password),
			cfg.Host,
			cfg.Port,
			cfg.Name,
			query.Encode(),
		), nil
	default:
		return "", fmt.Errorf("unsupported database driver: %s", driver)
	}
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
