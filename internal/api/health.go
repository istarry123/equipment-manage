package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type healthHandler struct {
	server *Server
}

type healthResponse struct {
	Status     string `json:"status"`
	DB         string `json:"db"`
	Version    string `json:"version"`
	ServerTime string `json:"server_time"`
}

// Get 健康检查：验证路由、配置与数据库连通性，供前端骨架页调用。
func (h *healthHandler) Get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	var one int
	err := h.server.DB.WithContext(ctx).Raw("SELECT 1").Scan(&one).Error
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "degraded",
			"db":     "down",
			"error":  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, healthResponse{
		Status:     "ok",
		DB:         "up",
		Version:    h.server.Version,
		ServerTime: time.Now().Format(time.RFC3339),
	})
}
