package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"equipment/internal/service"

	"github.com/gin-gonic/gin"
)

// strp 解析可选字符串（空串→nil）。
func optionalStr(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

// Create POST /api/equipment 新增设备（默认在库）。
func (s *Server) CreateEquipment(c *gin.Context) {
	var in service.CreateEquipmentInput
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	eq, err := service.CreateEquipment(s.DB, in)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": eq.ID, "internal_code": eq.InternalCode})
}

// EditEquipment PUT /api/equipment/:id 基本信息编辑。
func (s *Server) EditEquipment(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		Name       string `json:"name"`
		Model      string `json:"model"`
		CategoryID *uint  `json:"category_id"`
		Remark     string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	eq, err := service.EditBasic(s.DB, id, service.EditBasicInput{
		Name: body.Name, Model: body.Model, CategoryID: body.CategoryID, Remark: body.Remark,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": eq.ID, "updated_at": eq.UpdatedAt})
}

// CorrectEquipment POST /api/equipment/:id/correct 受限更正（决策 13：编号更正须原因+audit）。
func (s *Server) CorrectEquipment(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		EquipmentNo *string `json:"equipment_no"` // 空串→清为无编号
		Name        *string `json:"name"`
		Model       *string `json:"model"`
		CategoryID  *uint   `json:"category_id"`
		Remark      *string `json:"remark"`
		Reason      string  `json:"reason"`
		Operator    string  `json:"operator"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	eq, err := service.Correct(s.DB, id, service.CorrectInput{
		EquipmentNo: optionalStrValue(body.EquipmentNo),
		Name:        body.Name, Model: body.Model,
		CategoryID: body.CategoryID, Remark: body.Remark,
		Reason: body.Reason, Operator: body.Operator,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": eq.ID, "equipment_no": eq.EquipmentNo})
}

func parseID(c *gin.Context) (uint, bool) {
	v, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || v == 0 {
		writeError(c, http.StatusBadRequest, "无效的设备 id")
		return 0, false
	}
	return uint(v), true
}

func optionalStrValue(p *string) *string {
	if p == nil {
		return nil
	}
	return optionalStr(*p)
}

// writeServiceError 业务错误 → HTTP。
func writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(c, http.StatusNotFound, "记录不存在")
	case errors.Is(err, service.ErrDuplicate):
		writeError(c, http.StatusConflict, "同名称同型号下该编号已存在，无法保存")
	case errors.Is(err, service.ErrEmptyName):
		writeError(c, http.StatusBadRequest, "设备名称不能为空")
	case errors.Is(err, service.ErrOperator):
		writeError(c, http.StatusBadRequest, "操作人不能为空")
	case errors.Is(err, service.ErrReason):
		writeError(c, http.StatusBadRequest, "受限更正必须填写原因")
	default:
		writeError(c, http.StatusBadRequest, err.Error())
	}
}
