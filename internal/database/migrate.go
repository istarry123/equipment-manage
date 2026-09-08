package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var versionRe = regexp.MustCompile(`^V(\d+)_`)

// Migrate 执行版本化迁移：按文件名前缀（V001_xxx.sql 中的数字）升序执行，
// 以 SQLite PRAGMA user_version 记录当前版本，保证重复启动幂等。
func Migrate(sqlDB *sql.DB) error {
	current, err := userVersion(sqlDB)
	if err != nil {
		return err
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		m := versionRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil || v <= current {
			continue
		}
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		stmts := splitStatements(string(content))
		if err := runStatements(sqlDB, v, stmts); err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
		current = v
	}
	return nil
}

// userVersion 读取当前 schema 版本。
func userVersion(sqlDB *sql.DB) (int, error) {
	var v int
	if err := sqlDB.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("读取 user_version 失败: %w", err)
	}
	return v, nil
}

// runStatements 在一个事务内依次执行语句并更新 user_version。
func runStatements(sqlDB *sql.DB, version int, stmts []string) error {
	ctx := context.Background()
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // 提交成功后 Rollback 为空操作

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("执行SQL失败 [%s]: %w", truncate(stmt, 160), err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return err
	}
	return tx.Commit()
}

// splitStatements 按分号切分 SQL 文件；剔除整行注释与空段。
// 约束：迁移脚本中的字符串字面量不得包含分号（本项目 DDL 满足该约定）。
func splitStatements(content string) []string {
	var out []string
	// 去掉注释行后整体按分号切分
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	for _, seg := range strings.Split(b.String(), ";") {
		if s := strings.TrimSpace(seg); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
