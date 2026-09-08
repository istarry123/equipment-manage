package api

import (
	"net/http"
	"strconv"

	"equipment/internal/logger"
	"equipment/internal/models"

	"github.com/gin-gonic/gin"
)

// equipmentItem 台账列表条目（Phase 2 提供基础列表用于验证导入结果；
// 搜索/筛选/编辑等完整能力在 Phase 3）。
type equipmentItem struct {
	ID           uint    `json:"id"`
	EquipmentNo  *string `json:"equipment_no"`
	InternalCode string  `json:"internal_code"`
	Name         string  `json:"name"`
	Model        string  `json:"model"`
	Category     string  `json:"category"`
	Status       string  `json:"status"`
	CurrentSince *string `json:"current_since"`
	Remark       string  `json:"remark"`
}

type equipmentListResponse struct {
	Total int64           `json:"total"`
	Items []equipmentItem `json:"items"`
}

// List GET /api/equipment?offset=&limit=&q= 最小化台账列表（导入结果验证用）。
func (s *Server) ListEquipment(c *gin.Context) {
	offset := parsePositiveInt(c.Query("offset"), 0)
	limit := parsePositiveInt(c.Query("limit"), 20)
	if limit > 200 {
		limit = 200
	}
	q := c.Query("q")

	base := s.DB.Model(&models.Equipment{})
	if q != "" {
		like := "%" + q + "%"
		base = base.Where("equipment.equipment_no LIKE ? OR equipment.name LIKE ? OR equipment.model LIKE ? OR equipment.internal_code LIKE ?",
			like, like, like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		logger.Error("设备列表 count 失败: %v", err)
		writeError(c, http.StatusInternalServerError, "查询设备数量失败")
		return
	}

	var rows []struct {
		models.Equipment
		CategoryName string `gorm:"column:category_name"`
	}
	err := base.
		Select("equipment.*, COALESCE(category.name,'') AS category_name").
		Joins("LEFT JOIN category ON category.id = equipment.category_id").
		Order("equipment.id ASC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error
	if err != nil {
		logger.Error("设备列表查询失败: %v", err)
		writeError(c, http.StatusInternalServerError, "查询设备列表失败")
		return
	}

	items := make([]equipmentItem, 0, len(rows))
	for _, r := range rows {
		var since *string
		if r.CurrentSince.Valid {
			v := r.CurrentSince.Time.Format("2006-01-02 15:04:05")
			since = &v
		}
		items = append(items, equipmentItem{
			ID:           r.ID,
			EquipmentNo:  r.EquipmentNo,
			InternalCode: r.InternalCode,
			Name:         r.Name,
			Model:        r.Model,
			Category:     r.CategoryName,
			Status:       r.Status,
			CurrentSince: since,
			Remark:       r.Remark,
		})
	}
	c.JSON(http.StatusOK, equipmentListResponse{Total: total, Items: items})
}

func parsePositiveInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return v
}
