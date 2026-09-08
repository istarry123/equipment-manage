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
	DB      *gorm.DB
	Version string
}

// New 创建 gin 引擎（生产环境默认 Release 模式；DSH_DEBUG=1 时开启调试）。
func New(db *gorm.DB, version string) *gin.Engine {
	if strings.EqualFold(envOr("DSH_DEBUG", ""), "1") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &Server{DB: db, Version: version}

	r := gin.New()
	r.Use(gin.LoggerWithWriter(logger.Writer()), gin.Recovery())

	// ---- JSON API ----
	api := r.Group("/api")
	{
		h := &healthHandler{server: s}
		api.GET("/health", h.Get)
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
