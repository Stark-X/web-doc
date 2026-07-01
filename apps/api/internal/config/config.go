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
	DSN             string
	DBDriver        string
	SQLitePath      string
	WebRoot         string
	MaxUploadMB     int64
	AllowOrigin     string
	JWTSecret       string
	DisableRegister bool
}

func Load() *Config {
	storage := getEnv("WEBDOC_STORAGE", filepath.Join("..", "..", "storage", "docs"))
	abs, _ := filepath.Abs(storage)

	return &Config{
		Addr:            getEnv("WEBDOC_ADDR", ":8787"),
		StorageDir:      abs,
		DSN:             buildDSN(),
		DBDriver:        dbDriver(),
		SQLitePath:      getEnv("WEBDOC_SQLITE_PATH", "/data/webdoc.db"),
		WebRoot:         getEnv("WEBDOC_WEB_ROOT", ""),
		MaxUploadMB:     50,
		AllowOrigin:     getEnv("WEBDOC_ORIGIN", "*"),
		JWTSecret:       getEnv("WEBDOC_JWT_SECRET", "webdoc-default-secret-please-change"),
		DisableRegister: getEnv("WEBDOC_DISABLE_REGISTER", "") == "1",
	}
}

// buildDSN 优先使用 WEBDOC_DSN；否则根据 WEBDOC_DB_DRIVER 选择 SQLite 或 PostgreSQL。
func buildDSN() string {
	if dsn := os.Getenv("WEBDOC_DSN"); dsn != "" {
		return dsn
	}
	if dbDriver() != "postgres" {
		return getEnv("WEBDOC_SQLITE_PATH", "/data/webdoc.db")
	}
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

func dbDriver() string {
	if driver := os.Getenv("WEBDOC_DB_DRIVER"); driver != "" {
		return strings.ToLower(driver)
	}
	return strings.ToLower(getEnv("WEBDOC_DB_TYPE", "sqlite"))
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
