package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xiaofengguo/web-doc/api/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 打开数据库连接，并自动迁移所需表结构。
//
// 默认使用 SQLite，dsn 可为 SQLite 文件路径或 file: 风格 DSN，例如：
//
//	/data/webdoc.db
//	file:/data/webdoc.db?cache=shared
//
// 也兼容 PostgreSQL DSN，例如：
//
//	host=localhost user=webdoc password=webdoc dbname=webdoc port=5432 sslmode=disable TimeZone=UTC
//	postgres://webdoc:webdoc@localhost:5432/webdoc?sslmode=disable
func Open(dsn string) (*gorm.DB, error) {
	dialector, err := dialectorForDSN(dsn)
	if err != nil {
		return nil, err
	}
	d, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
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

func dialectorForDSN(dsn string) (gorm.Dialector, error) {
	if isPostgresDSN(dsn) {
		return postgres.Open(dsn), nil
	}
	if err := ensureSQLiteDir(dsn); err != nil {
		return nil, err
	}
	return sqlite.Open(dsn), nil
}

func isPostgresDSN(dsn string) bool {
	lower := strings.ToLower(strings.TrimSpace(dsn))
	return strings.HasPrefix(lower, "postgres://") ||
		strings.HasPrefix(lower, "postgresql://") ||
		strings.Contains(lower, "host=") ||
		strings.Contains(lower, "dbname=")
}

func ensureSQLiteDir(dsn string) error {
	path := sqlitePath(dsn)
	if path == "" || path == ":memory:" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite database directory %q: %w", dir, err)
	}
	return nil
}

func sqlitePath(dsn string) string {
	path := strings.TrimSpace(dsn)
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
		if i := strings.IndexAny(path, "?"); i >= 0 {
			path = path[:i]
		}
	}
	return path
}
