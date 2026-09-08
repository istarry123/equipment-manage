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
)

// version 通过 -ldflags 覆盖；默认跟随发布版本。
var version = "0.2.0-phase1"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return err
	}
	if err := logger.Init(cfg.LogDir); err != nil {
		return err
	}
	logger.Info("设备资产与流转管理系统 v%s 启动", version)
	logger.Info("配置: 端口=%d 数据库=%s 备份保留=%d 份", cfg.Port, cfg.DBFile, cfg.BackupKeep)

	db, err := database.Open(cfg.DBFile)
	if err != nil {
		return err
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := database.Migrate(sqlDB); err != nil {
		return err
	}
	logger.Info("数据库迁移完成: %s", cfg.DBFile)

	router := api.New(db, version)

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败（请检查端口占用或 config.yaml）: %w", addr, err)
	}

	url := fmt.Sprintf("http://localhost:%d", cfg.Port)
	logger.Info("服务已就绪: %s", url)

	// 服务启动后自动打开浏览器（测试时可设 EQ_NO_BROWSER=1 禁用）
	if os.Getenv("EQ_NO_BROWSER") != "1" {
		go func() {
			time.Sleep(600 * time.Millisecond)
			if err := browser.Open(url); err != nil {
				logger.Error("%v（请手动访问 %s）", err, url)
			}
		}()
	} else {
		logger.Info("EQ_NO_BROWSER=1：跳过自动打开浏览器")
	}

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
