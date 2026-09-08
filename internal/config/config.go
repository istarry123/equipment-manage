// Package config 负责读取与生成 config.yaml。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 为程序配置。字段对应 config.yaml。
type Config struct {
	Port       int    `yaml:"port"`        // 本地 Web 服务端口
	DBFile     string `yaml:"db_file"`     // SQLite 文件名
	LogDir     string `yaml:"log_dir"`     // 日志目录
	BackupKeep int    `yaml:"backup_keep"` // 备份保留份数
}

// Default 返回默认配置（与 config.yaml 示例一致）。
func Default() Config {
	return Config{
		Port:       8080,
		DBFile:     "equipment.db",
		LogDir:     "logs",
		BackupKeep: 30,
	}
}

// Load 读取 path 指定的 YAML 配置：
//   - 文件不存在时写入默认配置文件并返回默认值（方便“双击即用”）；
//   - 文件存在则解析，缺省字段回落到默认值。
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if werr := writeDefaultFile(path, cfg); werr != nil {
			return cfg, fmt.Errorf("创建默认配置文件失败: %w", werr)
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var parsed Config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return cfg, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}
	// 缺省字段回落默认值
	if parsed.Port == 0 {
		parsed.Port = cfg.Port
	}
	if parsed.DBFile == "" {
		parsed.DBFile = cfg.DBFile
	}
	if parsed.LogDir == "" {
		parsed.LogDir = cfg.LogDir
	}
	if parsed.BackupKeep == 0 {
		parsed.BackupKeep = cfg.BackupKeep
	}
	return parsed, nil
}

func writeDefaultFile(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
