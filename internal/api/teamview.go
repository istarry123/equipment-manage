package api

import (
	"net/http"
	"strings"

	"equipment/internal/service"

	"github.com/gin-gonic/gin"
)

// TeamEquipment GET /api/teams/equipment?team=&category=&status=&q=
// 班组设备视图：班组 → 设备类别 → 设备（三级，当前状态；未分配设备单独分组）。
func (s *Server) TeamEquipment(c *gin.Context) {
	opt := service.TeamViewOptions{
		Keyword: c.Query("q"),
		Status:  strings.ToUpper(c.Query("status")),
	}
	if v := parsePositiveInt(c.Query("category"), -1); v > 0 {
		id := uint(v)
		opt.CategoryID = &id
	}
	teamParam := c.Query("team")
	if teamParam == "unassigned" {
		opt.Unassigned = true
	} else if v := parsePositiveInt(teamParam, -1); v > 0 {
		id := uint(v)
		opt.TeamID = &id
	}
	view, err := service.TeamEquipmentView(s.DB, opt)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "查询班组设备失败")
		return
	}
	c.JSON(http.StatusOK, view)
}
