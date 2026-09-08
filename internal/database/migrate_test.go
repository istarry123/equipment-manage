package database

import (
	"strings"
	"testing"
)

// TestMigrateMemory 内存库：首次迁移 + 幂等重复迁移 + user_version。
func TestMigrateMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	sqlDB, err := SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(sqlDB); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if err := Migrate(sqlDB); err != nil {
		t.Fatalf("重复迁移失败(应幂等): %v", err)
	}
	v, err := userVersion(sqlDB)
	if err != nil || v != 2 {
		t.Fatalf("user_version 应为 2, got %d err=%v", v, err)
	}
}

// TestSchemaTablesExist 文件库迁移后校验核心表齐全。
func TestSchemaTablesExist(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/schema.db"
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	sqlDB, err := SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := Migrate(sqlDB); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	want := []string{"category", "team", "borrower", "settings", "audit_log", "equipment", "borrow_record", "flow_record"}
	rows, err := sqlDB.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		got[n] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("迁移后缺少表: %s (现有: %v)", w, got)
		}
	}
}

// TestSplitStatements 校验 SQL 语句切分（剔除注释与空段）。
func TestSplitStatements(t *testing.T) {
	sql := "-- 注释行\nCREATE TABLE a(id INTEGER);\n\nCREATE INDEX idx ON a(id);\n"
	stmts := splitStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("期望 2 条语句, got %d: %v", len(stmts), stmts)
	}
	if !strings.HasPrefix(stmts[0], "CREATE TABLE a") {
		t.Fatalf("首语句异常: %s", stmts[0])
	}
}
