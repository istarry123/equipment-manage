package service

import (
	"time"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// NameCount 计数条目（类别/班组等）。
type NameCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// StatusCount 状态计数。
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// FlowLine 最近流转条目。
type FlowLine struct {
	EquipmentID  uint   `json:"equipment_id"`
	EquipmentNo  string `json:"equipment_no"`
	DisplayNo    string `json:"display_no"`
	Name         string `json:"name"`
	Action       string `json:"action"`
	ActionText   string `json:"action_text"`
	ToTeamName   string `json:"to_team_name"`
	BorrowerName string `json:"borrower_name"`
	OccurredAt   string `json:"occurred_at"`
	Operator     string `json:"operator"`
}

// BorrowLine 当前外借条目。
type BorrowLine struct {
	BorrowRecordID uint   `json:"borrow_record_id"`
	EquipmentID    uint   `json:"equipment_id"`
	EquipmentNo    string `json:"equipment_no"`
	DisplayNo      string `json:"display_no"`
	Name           string `json:"name"`
	BorrowerName   string `json:"borrower_name"`
	BorrowDate     string `json:"borrow_date"`
	ExpectedReturn string `json:"expected_return_date"`
	OverdueDays    int    `json:"overdue_days"`
}

// Dashboard 首页聚合。
type Dashboard struct {
	Total          int64         `json:"total"`
	ByStatus       []StatusCount `json:"by_status"`
	ByCategory     []NameCount   `json:"by_category"`
	ByTeam         []NameCount   `json:"by_team"`
	RecentFlows    []FlowLine    `json:"recent_flows"`
	CurrentBorrows []BorrowLine  `json:"current_borrows"`
	OverdueCount   int64         `json:"overdue_count"`
	ByTeamCategory []TeamCatRow  `json:"by_team_category"` // 各班组×类别使用情况（班组设备视图同源）
}

// TeamCatRow 班组×类别统计行（status=IN_TEAM）。
type TeamCatRow struct {
	Team     string `json:"team"`
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// actionTextMap 动作 → 中文（与 api 层一致的小集合）。
var flowActionText = map[string]string{
	models.ActionImportInit: "导入建档", models.ActionOutToTeam: "出库给班组",
	models.ActionReturnFromTeam: "班组归还入库", models.ActionHandover: "班组转交",
	models.ActionBorrow: "外借", models.ActionReturnBorrow: "外借归还",
	models.ActionToMaintenance: "送修", models.ActionFromMaintenance: "维修完成",
	models.ActionScrap: "报废", models.ActionCorrect: "受限更正",
}

// DashboardStats 汇总首页所需统计（单机规模小，聚合成本可忽略）。
func DashboardStats(db *gorm.DB) (*Dashboard, error) {
	d := &Dashboard{
		// 空数据时输出 [] 而非 null（前端依赖数组语义，Go nil slice 会序列化为 null）
		ByCategory:     []NameCount{},
		ByTeam:         []NameCount{},
		RecentFlows:    []FlowLine{},
		CurrentBorrows: []BorrowLine{},
		ByTeamCategory: []TeamCatRow{},
	}
	if err := db.Model(&models.Equipment{}).Count(&d.Total).Error; err != nil {
		return nil, err
	}
	// 状态分布（补全零值）
	var stRows []StatusCount
	if err := db.Model(&models.Equipment{}).Select("status, COUNT(*) AS count").
		Group("status").Scan(&stRows).Error; err != nil {
		return nil, err
	}
	for _, code := range []string{
		models.StatusInStock, models.StatusInTeam, models.StatusBorrowed,
		models.StatusMaintenance, models.StatusScrapped, models.StatusOther,
	} {
		c := int64(0)
		for _, s := range stRows {
			if s.Status == code {
				c = s.Count
			}
		}
		d.ByStatus = append(d.ByStatus, StatusCount{Status: code, Count: c})
	}
	// 类别统计
	var catRows []NameCount
	if err := db.Table("equipment").Select("COALESCE(category.name,'未分类') AS name, COUNT(equipment.id) AS count").
		Joins("LEFT JOIN category ON category.id = equipment.category_id").
		Group("category.name").Order("count DESC").Scan(&catRows).Error; err != nil {
		return nil, err
	}
	if catRows == nil {
		catRows = []NameCount{}
	}
	d.ByCategory = catRows
	// 班组/内部单位统计（当前班组使用中设备）
	var teamRows []NameCount
	if err := db.Table("equipment").Select("COALESCE(team.name,'') AS name, COUNT(equipment.id) AS count").
		Joins("JOIN team ON team.id = equipment.current_team_id").
		Where("equipment.status = ?", models.StatusInTeam).
		Group("team.name").Order("count DESC").Scan(&teamRows).Error; err != nil {
		return nil, err
	}
	if teamRows == nil {
		teamRows = []NameCount{}
	}
	d.ByTeam = teamRows
	// 各班组×类别矩阵（班组设备视图同源：status=IN_TEAM）
	var tcRows []TeamCatRow
	if err := db.Table("equipment").
		Select("COALESCE(team.name,'') AS team, COALESCE(category.name,'(未分类)') AS category, COUNT(equipment.id) AS count").
		Joins("JOIN team ON team.id = equipment.current_team_id").
		Joins("LEFT JOIN category ON category.id = equipment.category_id").
		Where("equipment.status = ?", models.StatusInTeam).
		Group("team.name, category.name").
		Scan(&tcRows).Error; err != nil {
		return nil, err
	}
	if tcRows == nil {
		tcRows = []TeamCatRow{}
	}
	d.ByTeamCategory = tcRows
	// 最近流转（8 条）
	type txnRow struct {
		EquipmentID  uint
		EquipmentNo  *string
		EquipmentSeq int
		Name         string
		Model        string
		NoGrpCount   int64
		Action       string
		ToTeamName   string
		BorrowerName string
		OccurredAt   models.Time
		Operator     string
	}
	var flRows []txnRow
	if err := db.Table("flow_record").
		Select("flow_record.equipment_id, equipment.equipment_no, equipment.equipment_seq, equipment.name, equipment.model, " +
			EquipmentSelectNoGrp + ", flow_record.action, " +
			"flow_record.to_team_name, flow_record.borrower_name, flow_record.occurred_at, flow_record.operator").
		Joins("JOIN equipment ON equipment.id = flow_record.equipment_id").
		Order("flow_record.occurred_at DESC, flow_record.id DESC").
		Limit(8).Scan(&flRows).Error; err != nil {
		return nil, err
	}
	for _, r := range flRows {
		no := ""
		if r.EquipmentNo != nil {
			no = *r.EquipmentNo
		}
		d.RecentFlows = append(d.RecentFlows, FlowLine{
			EquipmentID: r.EquipmentID, EquipmentNo: no,
			DisplayNo: DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount),
			Name:      r.Name,
			Action:    r.Action, ActionText: flowActionText[r.Action],
			ToTeamName: r.ToTeamName, BorrowerName: r.BorrowerName,
			OccurredAt: r.OccurredAt.Format("01-02 15:04"), Operator: r.Operator,
		})
	}
	// 当前外借 + 逾期计数
	type brRow struct {
		ID             uint
		EquipmentID    uint
		EquipmentNo    *string
		EquipmentSeq   int
		EqName         string
		EqModel        string
		NoGrpCount     int64
		BorrowerName   string
		BorrowDate     models.Time
		ExpectedReturn models.NullTime
	}
	var brRows []brRow
	if err := db.Table("borrow_record").
		Select("borrow_record.id, borrow_record.equipment_id, equipment.equipment_no, equipment.equipment_seq, equipment.name AS eq_name, "+
			"equipment.model AS eq_model, "+EquipmentSelectNoGrp+", "+
			"borrower.name AS borrower_name, borrow_record.borrow_date, borrow_record.expected_return_date").
		Joins("JOIN equipment ON equipment.id = borrow_record.equipment_id").
		Joins("JOIN borrower ON borrower.id = borrow_record.borrower_id").
		Where("borrow_record.status = ?", models.BorrowOutstanding).
		Order("borrow_record.expected_return_date ASC").
		Limit(20).Scan(&brRows).Error; err != nil {
		return nil, err
	}
	for _, r := range brRows {
		no := ""
		if r.EquipmentNo != nil {
			no = *r.EquipmentNo
		}
		d.CurrentBorrows = append(d.CurrentBorrows, BorrowLine{
			BorrowRecordID: r.ID, EquipmentID: r.EquipmentID, EquipmentNo: no,
			DisplayNo:    DisplayNo(r.EquipmentNo, r.EqName, r.EqModel, r.EquipmentSeq, r.NoGrpCount),
			Name:         r.EqName,
			BorrowerName: r.BorrowerName, BorrowDate: r.BorrowDate.Format("01-02"),
			ExpectedReturn: timeFmtNull(r.ExpectedReturn),
		})
	}
	// 逾期计数（按设备口径：外借中且已过预计归还日）
	if err := db.Model(&models.BorrowRecord{}).
		Where("status = ? AND expected_return_date IS NOT NULL AND expected_return_date < ?",
			models.BorrowOutstanding, nowRFC3339()).
		Count(&d.OverdueCount).Error; err != nil {
		return nil, err
	}
	return d, nil
}

func timeFmtNull(n models.NullTime) string {
	if !n.Valid {
		return ""
	}
	return n.Time.Format("2006-01-02")
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339Nano)
}
