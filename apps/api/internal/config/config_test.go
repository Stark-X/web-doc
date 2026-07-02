package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsToSQLite(t *testing.T) {
	clearDBEnv(t)
	root := t.TempDir()
	t.Setenv("WEBDOC_STORAGE", filepath.Join(root, "docs"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DBDriver != "sqlite" {
		t.Fatalf("DBDriver = %q, want sqlite", cfg.DBDriver)
	}
	wantDSN := filepath.Join(root, "webdoc.db")
	if cfg.DSN != wantDSN {
		t.Fatalf("DSN = %q, want %q", cfg.DSN, wantDSN)
	}
}

func TestLoadUsesSQLitePathOverride(t *testing.T) {
	clearDBEnv(t)
	path := filepath.Join(t.TempDir(), "custom.db")
	t.Setenv("WEBDOC_DB_PATH", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DBDriver != "sqlite" {
		t.Fatalf("DBDriver = %q, want sqlite", cfg.DBDriver)
	}
	if cfg.DSN != path {
		t.Fatalf("DSN = %q, want %q", cfg.DSN, path)
	}
}

func TestLoadInfersPostgresDSN(t *testing.T) {
	clearDBEnv(t)
	dsn := "postgres://webdoc:webdoc@localhost:5432/webdoc?sslmode=disable"
	t.Setenv("WEBDOC_DSN", dsn)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DBDriver != "postgres" {
		t.Fatalf("DBDriver = %q, want postgres", cfg.DBDriver)
	}
	if cfg.DSN != dsn {
		t.Fatalf("DSN = %q, want %q", cfg.DSN, dsn)
	}
}

func TestLoadBuildsPostgresDSNWhenRequested(t *testing.T) {
	clearDBEnv(t)
	t.Setenv("WEBDOC_DB_DRIVER", "postgres")
	t.Setenv("WEBDOC_PG_HOST", "db")
	t.Setenv("WEBDOC_PG_DB", "webdoc")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DBDriver != "postgres" {
		t.Fatalf("DBDriver = %q, want postgres", cfg.DBDriver)
	}
	if !strings.Contains(cfg.DSN, "host=db") || !strings.Contains(cfg.DSN, "dbname=webdoc") {
		t.Fatalf("DSN = %q, want Postgres host/dbname", cfg.DSN)
	}
}

func TestLoadNormalizesShareBaseURL(t *testing.T) {
	clearDBEnv(t)
	t.Setenv("WEBDOC_SHARE_BASE_URL", " https://share.example.com/p/ ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ShareBaseURL != "https://share.example.com/p" {
		t.Fatalf("ShareBaseURL = %q, want %q", cfg.ShareBaseURL, "https://share.example.com/p")
	}
}

func TestNormalizeShareBaseURLAllowsPathPrefix(t *testing.T) {
	got, err := normalizeShareBaseURL("https://share.example.com/view/static/")
	if err != nil {
		t.Fatalf("normalizeShareBaseURL: %v", err)
	}
	if got != "https://share.example.com/view/static" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeShareBaseURLRejectsInvalidValues(t *testing.T) {
	cases := []string{
		"share.example.com/p",
		"ftp://share.example.com/p",
		"https://share.example.com/p?x=1",
		"https://share.example.com/p#section",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			if _, err := normalizeShareBaseURL(tc); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func clearDBEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"WEBDOC_DSN",
		"WEBDOC_DB_DRIVER",
		"WEBDOC_DB_PATH",
		"WEBDOC_PG_HOST",
		"WEBDOC_PG_PORT",
		"WEBDOC_PG_USER",
		"WEBDOC_PG_PASSWORD",
		"WEBDOC_PG_DB",
		"WEBDOC_PG_SSLMODE",
		"WEBDOC_PG_TZ",
		"WEBDOC_STORAGE",
		"WEBDOC_SHARE_BASE_URL",
	} {
		t.Setenv(key, "")
	}
}
