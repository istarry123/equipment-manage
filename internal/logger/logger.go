// Package logger 提供同时输出到控制台与日志文件的简易日志。
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	out io.Writer = os.Stdout
	l   *log.Logger
)

// Init 打开 logDir 下的 equipment.log（追加写），日志同时写控制台与文件。
func Init(logDir string) error {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(logDir, "equipment.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}
	out = io.MultiWriter(os.Stdout, f)
	l = log.New(out, "", log.LstdFlags)
	return nil
}

// Get 返回标准日志器（Init 之前调用则输出到控制台）。
func Get() *log.Logger {
	if l == nil {
		l = log.New(os.Stdout, "", log.LstdFlags)
	}
	return l
}

// Writer 返回当前日志输出目标（供 gin 等第三方库复用）。
func Writer() io.Writer {
	return out
}

// Info 记录一条普通日志。
func Info(format string, args ...any) {
	Get().Printf("[INFO] "+format, args...)
}

// Error 记录一条错误日志。
func Error(format string, args ...any) {
	Get().Printf("[ERROR] "+format, args...)
}
