package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadMissingCreatesDefault 验证：文件缺失时自动创建默认配置并正确解析。
func TestLoadMissingCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if cfg.Port != 8080 || cfg.DBFile != "equipment.db" || cfg.LogDir != "logs" || cfg.BackupKeep != 30 {
		t.Fatalf("默认配置不符合预期: %+v", cfg)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("应自动创建配置文件: %v", err)
	}
	// 二次加载（读取已生成文件）应一致
	cfg2, err := Load(path)
	if err != nil || cfg2 != cfg {
		t.Fatalf("二次加载不一致: %+v err=%v", cfg2, err)
	}
}

// TestLoadParseYAML 验证合法 YAML 解析与缺省回落。
func TestLoadParseYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "port: 9000\ndb_file: test.db\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if cfg.Port != 9000 || cfg.DBFile != "test.db" {
		t.Fatalf("解析失败: %+v", cfg)
	}
	if cfg.LogDir != "logs" || cfg.BackupKeep != 30 {
		t.Fatalf("缺省字段应回落默认值: %+v", cfg)
	}
}

// TestLoadInvalidYAML 验证非法内容报错而非静默接受。
func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("port: [bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("非法 YAML 应返回错误")
	}
}
