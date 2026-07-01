package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/xiaofengguo/web-doc/api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开数据库连接，并自动迁移所需表结构。
// driver 支持 sqlite（默认）或 postgres。
func Open(driver, dsn string) (*gorm.DB, error) {
	driver = normalizeDriver(driver)
	dialector, err := dialector(driver, dsn)
	if err != nil {
		return nil, err
	}

	d, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	if driver == "sqlite" {
		if err := configureSQLite(d); err != nil {
			return nil, err
		}
	}
	if err := d.AutoMigrate(
		&model.Node{},
		&model.Share{},
		&model.AISettings{},
		&model.PromptTemplate{},
		&model.MCPToken{},
		&model.User{},
	); err != nil {
		return nil, err
	}
	// 兜底：内置 Prompt 模板（仅首次插入）
	model.SeedBuiltinPrompts(d)
	return d, nil
}

func dialector(driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case "sqlite":
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
		return sqlite.Open(dsn), nil
	case "postgres":
		return postgres.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite":
		return "sqlite"
	case "postgres", "postgresql":
		return "postgres"
	default:
		return strings.ToLower(strings.TrimSpace(driver))
	}
}

func ensureSQLiteDir(dsn string) error {
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return nil
	}
	return os.MkdirAll(filepath.Dir(dsn), 0o755)
}

func configureSQLite(d *gorm.DB) error {
	for _, stmt := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if err := d.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
