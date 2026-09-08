// Package database 负责 SQLite 连接（纯 Go 驱动 modernc，CGO 关闭）与版本化迁移。
package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Open 打开 SQLite 数据库并返回 GORM 句柄。
// path 为数据库文件路径；传 ":memory:" 表示内存库（测试用）。
// 连接级 PRAGMA（外键、busy_timeout）通过 DSN 参数对每个连接生效。
func Open(path string) (*gorm.DB, error) {
	dsn := DSN(path)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败(%s): %w", path, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetConnMaxIdleTime(0)
	return db, nil
}

// DSN 构造 modernc/sqlite 连接串，并注入关键 PRAGMA。
// path 支持：文件路径、":memory:"、或 "mem://<名字>"（命名内存库，便于测试隔离）。
func DSN(path string) string {
	if strings.HasPrefix(path, "mem://") {
		name := strings.TrimPrefix(path, "mem://")
		return "file:mem_" + name + "?mode=memory&cache=shared&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	}
	if strings.Contains(path, "mode=memory") || path == ":memory:" {
		return "file::memory:?cache=shared&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	}
	p := filepath.ToSlash(path)
	return "file:" + p + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
}

// SQLDB 返回底层 *sql.DB（供迁移等原生 SQL 使用）。
func SQLDB(db *gorm.DB) (*sql.DB, error) {
	return db.DB()
}
