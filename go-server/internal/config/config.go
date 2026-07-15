package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port             int
	CorsOrigins      []string
	OAuthClients     []OAuthClient
	RequestBodyLimit int64
	ServeStatic      bool
	PublicDir        string
	LogDir           string
	LogLevel         string
	Database         DatabaseConfig
}

type OAuthClient struct {
	ID          string
	Secret      string
	RedirectURI string
	Name        string
}

type DatabaseConfig struct {
	Driver       string
	Host         string
	Port         int
	RootPassword string
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

func Load() Config {
	LoadDotEnv()
	return Config{
		Port:             envInt("PORT", 3001),
		CorsOrigins:      envList("CORS_ORIGIN", "http://localhost:5173"),
		OAuthClients:     envOAuthClients("OAUTH_CLIENTS"),
		RequestBodyLimit: envBytes("REQUEST_BODY_LIMIT", 80*1024*1024),
		ServeStatic:      envBool("SERVE_STATIC", true),
		PublicDir:        envString("PUBLIC_DIR", "public"),
		LogDir:           envString("LOG_DIR", "logs"),
		LogLevel:         strings.ToLower(envString("LOG_LEVEL", "info")),
		Database: DatabaseConfig{
			Driver:       strings.ToLower(envString("DB_DRIVER", envString("DATABASE_DRIVER", "mysql"))),
			Host:         envString("DB_HOST", envString("MYSQL_HOST", "127.0.0.1")),
			Port:         envInt("DB_PORT", envInt("MYSQL_PORT", 3306)),
			RootPassword: envString("DB_ROOT_PASSWORD", envString("MYSQL_ROOT_PASSWORD", envString("MYSQL_PASSWORD", ""))),
			User:         envString("DB_USER", envString("MYSQL_USER", "root")),
			Password:     envString("DB_PASSWORD", envString("MYSQL_PASSWORD", "")),
			Name:         envString("DB_NAME", envString("MYSQL_DATABASE", "ai_pai")),
			SSLMode:      envString("DB_SSLMODE", "disable"),
			MaxOpenConns: envInt("DB_MAX_OPEN_CONNS", envInt("MYSQL_MAX_OPEN_CONNS", 80)),
			MaxIdleConns: envInt("DB_MAX_IDLE_CONNS", envInt("MYSQL_MAX_IDLE_CONNS", 40)),
		},
	}
}

func envOAuthClients(key string) []OAuthClient {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	records := strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ';' })
	clients := []OAuthClient{}
	for _, record := range records {
		parts := strings.Split(record, "|")
		if len(parts) < 3 {
			continue
		}
		client := OAuthClient{
			ID:          strings.TrimSpace(parts[0]),
			Secret:      strings.TrimSpace(parts[1]),
			RedirectURI: strings.TrimSpace(parts[2]),
			Name:        strings.TrimSpace(parts[0]),
		}
		if len(parts) >= 4 && strings.TrimSpace(parts[3]) != "" {
			client.Name = strings.TrimSpace(parts[3])
		}
		clients = append(clients, client)
	}
	return clients
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}

func envList(key string, fallback string) []string {
	raw := envString(key, fallback)
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			values = append(values, item)
		}
	}
	return values
}

func envBytes(key string, fallback int64) int64 {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	multiplier := int64(1)
	for _, suffix := range []struct {
		unit string
		size int64
	}{
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"b", 1},
	} {
		if strings.HasSuffix(value, suffix.unit) {
			multiplier = suffix.size
			value = strings.TrimSuffix(value, suffix.unit)
			break
		}
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return fallback
	}
	return parsed * multiplier
}
