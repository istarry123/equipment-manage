//go:build diag

// diag_on.go —— 诊断构建（go build -tags diag）专用。
// 与正式版共用同一份启动代码（cmd/equipment/main.go），差异仅在：
//  1) step 探针默认全开并实时写入 diag-step.log（崩溃窗口消失后仍可回传日志定位）；
//  2) 启动时打印环境/版本/配置/数据库 user_version 与各表计数；
//  3) 禁用自动打开浏览器（避免诊断机上弹窗干扰），服务就绪后保持运行供人工核对。
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"equipment/internal/config"
)

// diagLogFile 诊断日志句柄（nil = 未打开，写入静默失败不阻断启动）。
var diagLogFile *os.File

// diagMode 诊断构建恒为 true（正式构建恒为 false，见 diag_off.go）。
func diagMode() bool { return true }

// diagStartup 打印诊断横幅与环境信息，并打开 diag-step.log。
func diagStartup(version string) {
	// 诊断构建不要自动打开浏览器
	_ = os.Setenv("EQ_NO_BROWSER", "1")

	wd, _ := os.Getwd()
	exe, _ := os.Executable()
	buf := fmt.Sprintf("===== equipment-diag v%s (diag build) =====\n", version)
	buf += fmt.Sprintf("时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	buf += fmt.Sprintf("GOOS/GOARCH: %s/%s  Go: %s\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
	buf += fmt.Sprintf("工作目录: %s\n", wd)
	buf += fmt.Sprintf("可执行文件: %s\n", exe)
	buf += fmt.Sprintf("EQ_STEPLOG=%q EQ_NO_BROWSER=%q\n", os.Getenv("EQ_STEPLOG"), os.Getenv("EQ_NO_BROWSER"))
	// 关键文件存在性（帮助判断是否从错误目录启动）
	for _, f := range []string{"config.yaml", "equipment.db", "backup", "logs"} {
		abs := filepath.Join(wd, f)
		if st, err := os.Stat(abs); err == nil {
			buf += fmt.Sprintf("文件存在: %s (%d B)\n", abs, st.Size())
		} else {
			buf += fmt.Sprintf("文件缺失: %s\n", abs)
		}
	}
	// 打开诊断日志（尽量落到工作目录，失败则退到临时目录）
	name := filepath.Join(wd, "diag-step.log")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		name = filepath.Join(os.TempDir(), "equipment-diag-step.log")
		f, err = os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "[diag] 无法打开诊断日志:", err)
		return
	}
	diagLogFile = f
	fmt.Fprintln(os.Stderr, "[diag] 诊断日志: "+name)
	diagWrite(buf)
}

// diagWriteStep 诊断模式下把每一步实时写入文件（append 到文件末尾即当前状态）。
func diagWriteStep(msg string) {
	if diagMode() {
		diagWrite("[step] " + msg + "\n")
	}
}

// diagWrite 写入原始内容（内部：整行写入并即时 flush）。
func diagWrite(s string) {
	if diagLogFile == nil {
		return
	}
	if _, err := diagLogFile.WriteString(s); err == nil {
		_ = diagLogFile.Sync()
	}
}

// diagReportConfig 记录解析后的关键配置（帮助核对端口/DB 路径是否如预期）。
func diagReportConfig(cfg config.Config) {
	diagWrite(fmt.Sprintf("[config] port=%d db=%q log_dir=%q backup_keep=%d\n",
		cfg.Port, cfg.DBFile, cfg.LogDir, cfg.BackupKeep))
}

// diagReportDB 记录迁移后的 user_version 与各表计数（判断迁移/数据是否正常）。
func diagReportDB(sdb *sql.DB) {
	if sdb == nil {
		return
	}
	diagWrite("[db] 迁移完成，user_version 与表计数:\n")
	var ver int
	if err := sdb.QueryRow("PRAGMA user_version").Scan(&ver); err != nil {
		diagWrite(fmt.Sprintf("[db]   user_version 读取失败: %v\n", err))
	} else {
		diagWrite(fmt.Sprintf("[db]   user_version = %d\n", ver))
	}
	for _, t := range []string{"equipment", "flow_record", "borrow_record", "import_batch", "category", "team", "borrower", "audit_log", "settings"} {
		var n int
		if err := sdb.QueryRow("SELECT COUNT(*) FROM " + t).Scan(&n); err != nil {
			diagWrite(fmt.Sprintf("[db]   %s: 查询失败 %v\n", t, err))
		} else {
			diagWrite(fmt.Sprintf("[db]   %s = %d\n", t, n))
		}
	}
}

// diagFinalize 关闭诊断日志文件（正常/异常退出时调用）。
func diagFinalize() {
	if diagLogFile != nil {
		_ = diagLogFile.Close()
		diagLogFile = nil
	}
}
