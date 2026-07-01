package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr            string
	StorageDir      string
	DBDriver        string
	DSN             string
	WebRoot         string
	MaxUploadMB     int64
	AllowOrigin     string
	JWTSecret       string
	DisableRegister bool
}

func Load() *Config {
	storage := getEnv("WEBDOC_STORAGE", filepath.Join("..", "..", "storage", "docs"))
	abs, _ := filepath.Abs(storage)
	driver, dsn := buildDatabaseConfig(abs)

	return &Config{
		Addr:            getEnv("WEBDOC_ADDR", ":8787"),
		StorageDir:      abs,
		DBDriver:        driver,
		DSN:             dsn,
		WebRoot:         getEnv("WEBDOC_WEB_ROOT", ""),
		MaxUploadMB:     50,
		AllowOrigin:     getEnv("WEBDOC_ORIGIN", "*"),
		JWTSecret:       getEnv("WEBDOC_JWT_SECRET", "webdoc-default-secret-please-change"),
		DisableRegister: getEnv("WEBDOC_DISABLE_REGISTER", "") == "1",
	}
}

// buildDatabaseConfig 默认使用 SQLite；显式指定 Postgres 时兼容旧的 PG* 配置。
func buildDatabaseConfig(storageDir string) (string, string) {
	if dsn := os.Getenv("WEBDOC_DSN"); dsn != "" {
		driver := getEnv("WEBDOC_DB_DRIVER", inferDriverFromDSN(dsn))
		return normalizeDriver(driver), dsn
	}

	driver := normalizeDriver(getEnv("WEBDOC_DB_DRIVER", "sqlite"))
	if driver == "postgres" {
		return driver, buildPostgresDSN()
	}

	path := getEnv("WEBDOC_DB_PATH", filepath.Join(filepath.Dir(storageDir), "webdoc.db"))
	abs, _ := filepath.Abs(path)
	return "sqlite", abs
}

func buildPostgresDSN() string {
	host := getEnv("WEBDOC_PG_HOST", "127.0.0.1")
	port := getEnv("WEBDOC_PG_PORT", "5432")
	user := getEnv("WEBDOC_PG_USER", "webdoc")
	pass := getEnv("WEBDOC_PG_PASSWORD", "webdoc")
	name := getEnv("WEBDOC_PG_DB", "webdoc")
	ssl := getEnv("WEBDOC_PG_SSLMODE", "disable")
	tz := getEnv("WEBDOC_PG_TZ", "UTC")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		host, port, user, pass, name, ssl, tz)
}

func inferDriverFromDSN(dsn string) string {
	lower := strings.ToLower(strings.TrimSpace(dsn))
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") || strings.Contains(lower, "host=") {
		return "postgres"
	}
	return "sqlite"
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgresql":
		return "postgres"
	case "postgres", "sqlite":
		return strings.ToLower(strings.TrimSpace(driver))
	default:
		return "sqlite"
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
