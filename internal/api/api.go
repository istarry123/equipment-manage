// Package api 组装 HTTP 路由：JSON API + 内嵌前端静态资源。
package api

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"equipment/internal/logger"
	"equipment/internal/webui"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server 持有 API 依赖。
type Server struct {
	DB       *gorm.DB
	Version  string
	sessions *parseStore
	details  *detailStore
}

// New 创建 gin 引擎（生产环境默认 Release 模式；DSH_DEBUG=1 时开启调试）。
func New(db *gorm.DB, version string) *gin.Engine {
	if strings.EqualFold(os.Getenv("DSH_DEBUG"), "1") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &Server{DB: db, Version: version, sessions: newParseStore(), details: newDetailStore()}

	r := gin.New()
	r.Use(gin.LoggerWithWriter(logger.Writer()), gin.Recovery())

	// ---- JSON API ----
	api := r.Group("/api")
	{
		h := &healthHandler{server: s}
		api.GET("/health", h.Get)

		ih := &importHandler{server: s, store: s.sessions}
		api.POST("/import/parse", ih.Parse)
		api.POST("/import/run", ih.Run)
		api.POST("/import/reset", ih.Reset)
		api.POST("/import/reconcile", ih.Reconcile)

		// 外借明细通道（v1.3 Phase 3：解析 + 匹配预览，只读）
		dh := &importDetailHandler{server: s, store: s.details}
		api.POST("/import/borrow-detail/parse", dh.ParseDetail)

		api.GET("/equipment", s.ListEquipment)
		api.POST("/equipment", s.CreateEquipment)
		api.GET("/equipment/:id", s.GetEquipment)
		api.PUT("/equipment/:id", s.EditEquipment)
		api.POST("/equipment/:id/correct", s.CorrectEquipment)
		api.POST("/equipment/:id/flow", s.DoFlow)
		api.GET("/equipment/:id/transactions", s.ListTransactions)

		api.GET("/categories", s.ListCategories)
		api.POST("/categories", s.CreateCategory)
		api.DELETE("/categories/:id", s.DeleteCategory)
		api.GET("/teams", s.ListTeams)
		api.POST("/teams", s.CreateTeam)
		api.PUT("/teams/:id", s.UpdateTeam)
		api.DELETE("/teams/:id", s.DeleteTeam)
		api.GET("/teams/equipment", s.TeamEquipment)

		api.GET("/borrows", s.ListBorrows)
		api.POST("/borrows/:id/return", s.ReturnBorrow)
		api.POST("/borrows/:id/extend", s.ExtendBorrow)
		api.GET("/borrowers", s.ListBorrowers)
		api.POST("/borrowers", s.CreateBorrower)
		api.PUT("/borrowers/:id", s.UpdateBorrower)

		api.GET("/dashboard", s.Dashboard)
		api.GET("/export/equipment", s.ExportEquipment)
		api.GET("/export/flow", s.ExportFlow)
		api.GET("/backups", s.ListBackups)
		api.POST("/backup", s.ManualBackup)
		api.POST("/restore", s.Restore)
	}

	// ---- 前端静态资源（SPA） ----
	distFS, err := fs.Sub(webui.Dist, "dist")
	if err != nil {
		panic("webui/dist 缺失：请先构建前端（npm run build）再编译后端")
	}
	fileServer := http.FileServer(http.FS(distFS))
	r.NoRoute(func(c *gin.Context) {
		p := strings.TrimPrefix(c.Request.URL.Path, "/")
		if p == "api" || strings.HasPrefix(p, "api/") {
			writeError(c, http.StatusNotFound, "接口不存在")
			return
		}
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(distFS, p); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		// SPA 回退：未知路径一律返回 index.html
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
	return r
}

// writeError 输出统一错误体：{"error":{"code":..,"message":..}}
func writeError(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, gin.H{
		"error": gin.H{"code": code, "message": message},
	})
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
