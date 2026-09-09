//go:build !diag

// diag_off.go —— 正式构建（无 -tags diag）的空实现。
// 保持正式 exe 行为与旧版完全一致：step 探针仅受 EQ_STEPLOG=1 控制，
// 不写 diag-step.log、不打印诊断横幅、不自动禁用浏览器。
package main

import (
	"database/sql"

	"equipment/internal/config"
)

// diagMode 正式构建恒为 false。
func diagMode() bool { return false }

// diagStartup 正式构建无操作。
func diagStartup(version string) {}

// diagWriteStep 正式构建无操作（step 的 EQ_STEPLOG 控制见 main.go）。
func diagWriteStep(msg string) {}

// diagReportConfig 正式构建无操作。
func diagReportConfig(cfg config.Config) {}

// diagReportDB 正式构建无操作。
func diagReportDB(sdb *sql.DB) {}

// diagFinalize 正式构建无操作。
func diagFinalize() {}
