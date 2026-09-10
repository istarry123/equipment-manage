package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"equipment/internal/logger"
	"equipment/internal/models"
	"equipment/internal/service"

	"github.com/gin-gonic/gin"
)

// ExportFlow GET /api/export/flow
// ?team_id=&status=&borrower_id=&include_current=&include_history=&include_summary=
// 文件下载 API：返回 .xlsx（Content-Type + Content-Disposition），并写 EXPORT_EQUIPMENT_FLOW 审计。
func (s *Server) ExportFlow(c *gin.Context) {
	f := service.FlowExportFilter{
		TeamID:         parseUintQuery(c.Query("team_id")),
		Status:         c.Query("status"),
		BorrowerID:     parseUintQuery(c.Query("borrower_id")),
		IncludeCurrent: parseBoolQuery(c.Query("include_current"), true),
		IncludeHistory: parseBoolQuery(c.Query("include_history"), true),
		IncludeSummary: parseBoolQuery(c.Query("include_summary"), true),
	}

	data, err := service.BuildFlowExportData(s.DB, f)
	if err != nil {
		logger.Error("流转导出查询失败: %v", err)
		writeError(c, http.StatusInternalServerError, "导出失败")
		return
	}
	xlsx, err := service.RenderFlowExportXLSX(data)
	if err != nil {
		logger.Error("流转导出生成失败: %v", err)
		writeError(c, http.StatusInternalServerError, "导出失败")
		return
	}

	// 审计（不记录 Excel 二进制，只记时间/筛选/导出数量）
	_ = s.DB.Create(&models.AuditLog{
		Action:    "EXPORT_EQUIPMENT_FLOW",
		Target:    "设备流转情况",
		Detail:    flowAuditDetail(f, data),
		Reason:    "",
		Operator:  "系统导出",
		CreatedAt: models.Now(),
	}).Error

	name := "设备流转情况_" + time.Now().Format("2006-01-02") + ".xlsx"
	c.Header("Content-Disposition", contentDisposition(name))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Data(http.StatusOK, "application/octet-stream", xlsx)
}

// flowAuditDetail 拼审计详情（筛选条件 + 导出数量）。
func flowAuditDetail(f service.FlowExportFilter, d *service.FlowExportData) string {
	parts := []string{}
	if f.TeamID != nil {
		parts = append(parts, fmt.Sprintf("team_id=%d", *f.TeamID))
	}
	if f.Status != "" {
		parts = append(parts, "status="+f.Status)
	}
	if f.BorrowerID != nil {
		parts = append(parts, fmt.Sprintf("borrower_id=%d", *f.BorrowerID))
	}
	if f.IncludeCurrent {
		parts = append(parts, "含当前")
	}
	if f.IncludeHistory {
		parts = append(parts, "含历史")
	}
	if f.IncludeSummary {
		parts = append(parts, "含统计")
	}
	cond := "全部"
	if len(parts) > 0 {
		cond = strings.Join(parts, ",")
	}
	return fmt.Sprintf("筛选[%s]；设备 %d 台、历史 %d 条", cond, len(d.Current), len(d.History))
}

// contentDisposition 构造带中文文件名的下载头（RFC 5987 filename* 供 Chrome/Edge 正确解码）。
func contentDisposition(filename string) string {
	// 提供 ASCII 回退名 + UTF-8 编码名
	return fmt.Sprintf(`attachment; filename="equipment-flow-export.xlsx"; filename*=UTF-8''%s`, url.PathEscape(filename))
}

// parseUintQuery 解析可选的无符号整数 query（非法/空 → nil）。
func parseUintQuery(s string) *uint {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil
	}
	u := uint(v)
	return &u
}

// parseBoolQuery 解析可选布尔 query（缺省返回 def；接受 true/false/1/0）。
func parseBoolQuery(s string, def bool) bool {
	if s == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
