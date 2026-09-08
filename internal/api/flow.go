package api

import (
	"net/http"
	"strings"
	"time"

	"equipment/internal/models"
	"equipment/internal/service"

	"github.com/gin-gonic/gin"
)

// actionText 动作 → 中文。
var actionTextMap = map[string]string{
	models.ActionImportInit:      "导入建档",
	models.ActionOutToTeam:       "出库给班组",
	models.ActionReturnFromTeam:  "班组归还入库",
	models.ActionHandover:        "班组转交",
	models.ActionBorrow:          "外借",
	models.ActionReturnBorrow:    "外借归还",
	models.ActionToMaintenance:   "送修",
	models.ActionFromMaintenance: "维修完成",
	models.ActionScrap:           "报废",
	models.ActionCorrect:         "受限更正",
}

// txnItem 时间线条目。
type txnItem struct {
	ID           uint   `json:"id"`
	Action       string `json:"action"`
	ActionText   string `json:"action_text"`
	FromStatus   string `json:"from_status"`
	ToStatus     string `json:"to_status"`
	FromTeamName string `json:"from_team_name"`
	ToTeamName   string `json:"to_team_name"`
	BorrowerName string `json:"borrower_name"`
	OccurredAt   string `json:"occurred_at"`
	Operator     string `json:"operator"`
	Remark       string `json:"remark"`
}

// flowRequest 流转请求。
type flowRequest struct {
	Action             string  `json:"action"`
	Operator           string  `json:"operator"`
	ToTeamID           *uint   `json:"to_team_id"`
	BorrowerID         *uint   `json:"borrower_id"`
	ExpectedReturnDate *string `json:"expected_return_date"`
	Remark             string  `json:"remark"`
}

// DoFlow POST /api/equipment/:id/flow
func (s *Server) DoFlow(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body flowRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if body.Action == "" {
		writeError(c, http.StatusBadRequest, "缺少流转动作 action")
		return
	}
	var exp *time.Time
	if body.ExpectedReturnDate != nil && strings.TrimSpace(*body.ExpectedReturnDate) != "" {
		t, err := parseTime(*body.ExpectedReturnDate)
		if err != nil {
			writeError(c, http.StatusBadRequest, "预计归还日期格式错误（示例 2026-10-01）")
			return
		}
		exp = &t
	}
	eq, err := service.Transition(s.DB, id, service.FlowRequest{
		Action: body.Action, Operator: body.Operator,
		ToTeamID: body.ToTeamID, BorrowerID: body.BorrowerID,
		ExpectedReturnDate: exp, Remark: body.Remark,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": eq.ID, "status": eq.Status,
		"status_text":   statusTextMap[eq.Status],
		"current_since": timeFmt(eq.CurrentSince),
	})
}

// parseTime 解析常见日期格式。
func parseTime(v string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, strings.TrimSpace(v), time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, &time.ParseError{}
}

func timeFmt(n models.NullTime) string {
	if !n.Valid {
		return ""
	}
	return n.Time.Format("2006-01-02 15:04:05")
}

// ListTransactions GET /api/equipment/:id/transactions
func (s *Server) ListTransactions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	offset := parsePositiveInt(c.Query("offset"), 0)
	limit := parsePositiveInt(c.Query("limit"), 100)
	if limit > 500 {
		limit = 500
	}
	rows, total, err := service.ListTransactions(s.DB, id, limit, offset)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "查询流转历史失败")
		return
	}
	items := make([]txnItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, txnItem{
			ID: r.ID, Action: r.Action, ActionText: actionTextMap[r.Action],
			FromStatus: r.FromStatus, ToStatus: r.ToStatus,
			FromTeamName: r.FromTeamName, ToTeamName: r.ToTeamName,
			BorrowerName: r.BorrowerName,
			OccurredAt:   r.OccurredAt.Format("2006-01-02 15:04:05"),
			Operator:     r.Operator, Remark: r.Remark,
		})
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "items": items})
}
