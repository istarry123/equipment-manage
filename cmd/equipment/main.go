// 设备资产与流转管理系统 - 入口
// 启动顺序：配置 → 日志 → 数据库(迁移) → HTTP 服务 → 自动打开浏览器
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"equipment/internal/api"
	"equipment/internal/browser"
	"equipment/internal/config"
	"equipment/internal/database"
	"equipment/internal/logger"
	"equipment/internal/service"
)

// version 通过 -ldflags 覆盖；默认跟随发布版本。
var version = "1.0.0"

// step 现场诊断探针：设 EQ_STEPLOG=1 时每步向控制台打印标记，用于定位启动早期崩溃。
func step(msg string) {
	if os.Getenv("EQ_STEPLOG") == "1" {
		fmt.Fprintln(os.Stderr, "[step] "+msg)
	}
}

func main() {
	step("main: 开始")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}

func run() error {
	step("加载配置")
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return err
	}
	step("初始化日志")
	if err := logger.Init(cfg.LogDir); err != nil {
		return err
	}
	logger.Info("设备资产与流转管理系统 v%s 启动", version)
	logger.Info("配置: 端口=%d 数据库=%s 备份保留=%d 份", cfg.Port, cfg.DBFile, cfg.BackupKeep)

	step("打开数据库")
	db, err := database.Open(cfg.DBFile)
	if err != nil {
		return err
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	step("执行数据库迁移")
	if err := database.Migrate(sqlDB); err != nil {
		return err
	}
	logger.Info("数据库迁移完成: %s", cfg.DBFile)

	// 启动自动备份（决策：程序启动自动备份，保留最近 cfg.BackupKeep 份）
	if name, err := service.BackupNow(db, cfg.DBFile, cfg.BackupKeep); err != nil {
		logger.Error("启动自动备份失败: %v", err)
	} else {
		logger.Info("启动自动备份完成: %s", name)
	}

	step("启动自动备份")
	if name, err := service.BackupNow(db, cfg.DBFile, cfg.BackupKeep); err != nil {
		logger.Error("启动自动备份失败: %v", err)
	} else {
		logger.Info("启动自动备份完成: %s", name)
	}

	step("组装路由")
	router := api.New(db, version)

	step("监听端口")
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败（请检查端口占用或 config.yaml）: %w", addr, err)
	}

	url := fmt.Sprintf("http://localhost:%d", cfg.Port)
	logger.Info("服务已就绪: %s", url)

	// 服务启动后自动打开浏览器（测试时可设 EQ_NO_BROWSER=1 禁用）
	if os.Getenv("EQ_NO_BROWSER") != "1" {
		step("尝试打开浏览器")
		go func() {
			time.Sleep(600 * time.Millisecond)
			if err := browser.Open(url); err != nil {
				logger.Error("%v（请手动访问 %s）", err, url)
			}
		}()
	} else {
		logger.Info("EQ_NO_BROWSER=1：跳过自动打开浏览器")
	}

	step("开始服务")

	srv := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP 服务异常退出: %w", err)
		}
	case <-sigCh:
		logger.Info("收到退出信号，正在安全关闭...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("关闭服务异常: %v", err)
	}
	logger.Info("已安全退出")
	return nil
}
