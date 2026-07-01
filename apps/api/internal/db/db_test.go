package db

import (
	"path/filepath"
	"testing"

	"github.com/xiaofengguo/web-doc/api/internal/model"
)

func TestOpenSQLiteAutoMigratesAndSeeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webdoc.db")

	d, err := Open("sqlite", path)
	if err != nil {
		t.Fatalf("Open sqlite: %v", err)
	}

	var count int64
	if err := d.Model(&model.PromptTemplate{}).Count(&count).Error; err != nil {
		t.Fatalf("count prompt templates: %v", err)
	}
	if count == 0 {
		t.Fatal("expected built-in prompt templates to be seeded")
	}
}

func TestOpenRejectsUnsupportedDriver(t *testing.T) {
	if _, err := Open("mysql", "webdoc"); err == nil {
		t.Fatal("expected unsupported driver error")
	}
}
