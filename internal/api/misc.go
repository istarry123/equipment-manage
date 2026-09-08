package api

import (
	"net/http"
	"sync"
	"time"

	"equipment/internal/config"
	"equipment/internal/database"
	"equipment/internal/logger"
	"equipment/internal/models"
	"equipment/internal/service"

	"github.com/gin-gonic/gin"
)

// 获取当前运行库的文件路径与备份保留数（单机单目录场景）。
func runtimeDBInfo() (string, int) {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return "equipment.db", 30
	}
	return cfg.DBFile, cfg.BackupKeep
}

// Dashboard GET /api/dashboard
func (s *Server) Dashboard(c *gin.Context) {
	d, err := service.DashboardStats(s.DB)
	if err != nil {
		logger.Error("Dashboard 统计失败: %v", err)
		writeError(c, http.StatusInternalServerError, "统计失败")
		return
	}
	c.JSON(http.StatusOK, d)
}

// ExportEquipment GET /api/export/equipment?q=&category=&status=&team=
func (s *Server) ExportEquipment(c *gin.Context) {
	data, err := service.ExportEquipmentXLSX(s.DB, service.ExportFilter{
		Q: c.Query("q"), Category: c.Query("category"),
		Status: c.Query("status"), Team: c.Query("team"),
	})
	if err != nil {
		logger.Error("导出失败: %v", err)
		writeError(c, http.StatusInternalServerError, "导出失败")
		return
	}
	name := "equipment-" + time.Now().Format("20060102-150405") + ".xlsx"
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// ListBackups GET /api/backups
func (s *Server) ListBackups(c *gin.Context) {
	list, err := service.ListBackups()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "读取备份列表失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": list})
}

// ManualBackup POST /api/backup 手动备份（非破坏性）。
func (s *Server) ManualBackup(c *gin.Context) {
	dbFile, keep := runtimeDBInfo()
	name, err := service.BackupNow(s.DB, dbFile, keep)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.DB.Create(&models.AuditLog{ //nolint:errcheck
		Action: "BACKUP", Target: name, Operator: "系统导入", CreatedAt: models.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"name": name})
}

var restoreBusy sync.Mutex

// Restore POST /api/restore {filename, confirm:true}
// 危险操作（铁律 5）：需 confirm；先自动备份当前库 → 关闭连接 → 文件替换 → 重开并迁移。
func (s *Server) Restore(c *gin.Context) {
	if !restoreBusy.TryLock() {
		writeError(c, http.StatusConflict, "已有恢复操作进行中，请稍候")
		return
	}
	defer restoreBusy.Unlock()

	var body struct {
		Filename string `json:"filename"`
		Confirm  bool   `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Filename == "" {
		writeError(c, http.StatusBadRequest, "缺少 filename")
		return
	}
	if !body.Confirm {
		writeError(c, http.StatusBadRequest, "恢复是危险操作，请确认后再试（confirm: true）")
		return
	}
	path, err := service.BackupFilePath(body.Filename)
	if err != nil {
		writeError(c, http.StatusBadRequest, "备份文件不存在或非法")
		return
	}
	dbFile, keep := runtimeDBInfo()

	// 1) 恢复前自动备份当前库
	snapshot, err := service.BackupNow(s.DB, dbFile, keep)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "恢复前备份失败，已中止恢复: "+err.Error())
		return
	}

	// 2) 关闭当前连接
	sqlDB, err := database.SQLDB(s.DB)
	if err == nil {
		_ = sqlDB.Close()
	}

	// 3) 文件替换
	if err := service.FileReplace(path, dbFile); err != nil {
		writeError(c, http.StatusInternalServerError, "恢复文件替换失败: "+err.Error())
		return
	}

	// 4) 重开并迁移（schema 版本向后兼容自己产生的备份）
	newDB, reopenErr := database.Open(dbFile)
	if reopenErr == nil {
		if ns, err2 := database.SQLDB(newDB); err2 == nil {
			reopenErr = database.Migrate(ns)
			if reopenErr == nil {
				s.DB = newDB
			}
		} else {
			reopenErr = err2
		}
	}
	if reopenErr != nil {
		writeError(c, http.StatusInternalServerError, "恢复后重开数据库失败: "+reopenErr.Error())
		return
	}
	s.DB.Create(&models.AuditLog{ //nolint:errcheck
		Action: "RESTORE", Target: body.Filename, Operator: "系统管理员", CreatedAt: models.Now(),
	})
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "restored": body.Filename,
		"safety_backup": snapshot, "message": "恢复完成（原数据已安全备份为 " + snapshot + "）",
	})
}
