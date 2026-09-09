package api

import (
	"net/http"
	"strconv"
	"strings"

	"equipment/internal/logger"
	"equipment/internal/models"
	"equipment/internal/service"

	"github.com/gin-gonic/gin"
)

// equipmentItem 台账列表/详情条目。
type equipmentItem struct {
	ID                uint    `json:"id"`
	EquipmentNo       *string `json:"equipment_no"`
	EquipmentSeq      int     `json:"equipment_seq"`
	DisplayNo         string  `json:"display_no"`
	InternalCode      string  `json:"internal_code"`
	Name              string  `json:"name"`
	Model             string  `json:"model"`
	Category          string  `json:"category"`
	CategoryID        *uint   `json:"category_id"`
	Status            string  `json:"status"`
	StatusText        string  `json:"status_text"`
	CurrentTeam       string  `json:"current_team"`
	CurrentTeamID     *uint   `json:"current_team_id"`
	CurrentBorrower   string  `json:"current_borrower"`
	CurrentBorrowerID *uint   `json:"current_borrower_id"`
	CurrentSince      *string `json:"current_since"`
	Remark            string  `json:"remark"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type equipmentListResponse struct {
	Total int64           `json:"total"`
	Items []equipmentItem `json:"items"`
}

// statusText 状态码→中文。
var statusTextMap = map[string]string{
	models.StatusInStock: "在库", models.StatusInTeam: "班组使用", models.StatusBorrowed: "外借",
	models.StatusMaintenance: "维修", models.StatusScrapped: "报废", models.StatusOther: "其他",
}

// rowEquipment equipment 行（含联查名称与同组计数）。
type rowEquipment struct {
	models.Equipment
	CategoryName string `gorm:"column:category_name"`
	TeamName     string `gorm:"column:team_name"`
	BorrowerName string `gorm:"column:borrower_name"`
	NoGrpCount   int64  `gorm:"column:no_grp_count"` // 同 (no,name,model) 计数（无编号恒 0）
}

func toItem(r rowEquipment) equipmentItem {
	var since *string
	if r.CurrentSince.Valid {
		v := r.CurrentSince.Time.Format("2006-01-02 15:04:05")
		since = &v
	}
	return equipmentItem{
		ID:                r.ID,
		EquipmentNo:       r.EquipmentNo,
		EquipmentSeq:      r.EquipmentSeq,
		DisplayNo:         service.DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount),
		InternalCode:      r.InternalCode,
		Name:              r.Name,
		Model:             r.Model,
		Category:          r.CategoryName,
		CategoryID:        r.CategoryID,
		Status:            r.Status,
		StatusText:        statusTextMap[r.Status],
		CurrentTeam:       r.TeamName,
		CurrentTeamID:     r.CurrentTeamID,
		CurrentBorrower:   r.BorrowerName,
		CurrentBorrowerID: r.CurrentBorrowerID,
		CurrentSince:      since,
		Remark:            r.Remark,
		CreatedAt:         r.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:         r.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// equipmentSelectNoGrp 供列表/详情复用的同组计数表达式。
const equipmentSelectNoGrp = `CASE WHEN equipment.equipment_no IS NOT NULL
  THEN (SELECT COUNT(*) FROM equipment e2
        WHERE e2.equipment_no = equipment.equipment_no
          AND e2.name = equipment.name AND e2.model = equipment.model)
  ELSE 0 END AS no_grp_count`

// List GET /api/equipment?q=&category=&status=&team=&offset=&limit=
func (s *Server) ListEquipment(c *gin.Context) {
	offset := parsePositiveInt(c.Query("offset"), 0)
	limit := parsePositiveInt(c.Query("limit"), 20)
	if limit > 200 {
		limit = 200
	}
	conds, args := equipmentConds(c.Query("q"), c.Query("category"), c.Query("status"), c.Query("team"))

	var total int64
	base := s.DB.Model(&models.Equipment{})
	for i, cond := range conds {
		base = base.Where(cond, args[i]...)
	}
	if err := base.Count(&total).Error; err != nil {
		logger.Error("设备列表 count 失败: %v", err)
		writeError(c, http.StatusInternalServerError, "查询设备数量失败")
		return
	}

	q := s.DB.Model(&models.Equipment{}).
		Select("equipment.*, COALESCE(category.name,'') AS category_name, COALESCE(team.name,'') AS team_name, COALESCE(borrower.name,'') AS borrower_name, " + equipmentSelectNoGrp).
		Joins("LEFT JOIN category ON category.id = equipment.category_id").
		Joins("LEFT JOIN team ON team.id = equipment.current_team_id").
		Joins("LEFT JOIN borrower ON borrower.id = equipment.current_borrower_id")
	for i, cond := range conds {
		q = q.Where(cond, args[i]...)
	}

	var rows []rowEquipment
	if err := q.Order("equipment.id ASC").Offset(offset).Limit(limit).Scan(&rows).Error; err != nil {
		logger.Error("设备列表查询失败: %v", err)
		writeError(c, http.StatusInternalServerError, "查询设备列表失败")
		return
	}
	items := make([]equipmentItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toItem(r))
	}
	c.JSON(http.StatusOK, equipmentListResponse{Total: total, Items: items})
}

// GetEquipment GET /api/equipment/:id
func (s *Server) GetEquipment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(c, http.StatusBadRequest, "无效的设备 id")
		return
	}
	var r rowEquipment
	err = s.DB.Model(&models.Equipment{}).
		Select("equipment.*, COALESCE(category.name,'') AS category_name, COALESCE(team.name,'') AS team_name, COALESCE(borrower.name,'') AS borrower_name, "+equipmentSelectNoGrp).
		Joins("LEFT JOIN category ON category.id = equipment.category_id").
		Joins("LEFT JOIN team ON team.id = equipment.current_team_id").
		Joins("LEFT JOIN borrower ON borrower.id = equipment.current_borrower_id").
		Where("equipment.id = ?", id).
		Scan(&r).Error
	if err != nil || r.ID == 0 {
		writeError(c, http.StatusNotFound, "设备不存在")
		return
	}
	c.JSON(http.StatusOK, toItem(r))
}

// equipmentConds 构建共用筛选条件。
func equipmentConds(q, category, status, team string) ([]string, [][]any) {
	var conds []string
	var args [][]any
	if q != "" {
		like := "%" + q + "%"
		conds = append(conds, "equipment.equipment_no LIKE ? OR equipment.name LIKE ? OR equipment.model LIKE ? OR equipment.internal_code LIKE ?")
		args = append(args, []any{like, like, like, like})
	}
	if v := parsePositiveInt(category, -1); v > 0 {
		conds = append(conds, "equipment.category_id = ?")
		args = append(args, []any{v})
	}
	if v := parsePositiveInt(team, -1); v > 0 {
		conds = append(conds, "equipment.current_team_id = ?")
		args = append(args, []any{v})
	}
	if status != "" {
		conds = append(conds, "equipment.status = ?")
		args = append(args, []any{strings.ToUpper(status)})
	}
	return conds, args
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
