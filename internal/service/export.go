package service

import (
	"strconv"

	"equipment/internal/models"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// ExportFilter 台账导出筛选（与列表一致）。
type ExportFilter struct {
	Q        string
	Category string
	Status   string
	Team     string
}

// 状态 → 中文（导出用）。
var exportStatusText = map[string]string{
	models.StatusInStock: "在库", models.StatusInTeam: "班组使用", models.StatusBorrowed: "外借",
	models.StatusMaintenance: "维修", models.StatusScrapped: "报废", models.StatusOther: "其他",
}

type exportRow struct {
	EquipmentNo  *string
	EquipmentSeq int
	Name         string
	Model        string
	CategoryName string
	Status       string
	TeamName     string
	BorrowerName string
	CurrentSince models.NullTime
	InternalCode string
	Remark       string
	UpdatedAt    models.Time
	NoGrpCount   int64
}

// ExportEquipmentXLSX 导出当前台账（关键字/类别/状态/班组筛选与列表一致）。
func ExportEquipmentXLSX(db *gorm.DB, f ExportFilter) ([]byte, error) {
	q := db.Table("equipment").
		Select(`equipment.equipment_no, equipment.equipment_seq, equipment.name, equipment.model,
			COALESCE(category.name,'') AS category_name, equipment.status,
			COALESCE(team.name,'') AS team_name, COALESCE(borrower.name,'') AS borrower_name,
			equipment.current_since, equipment.internal_code, equipment.remark, equipment.updated_at, ` + EquipmentSelectNoGrp).
		Joins("LEFT JOIN category ON category.id = equipment.category_id").
		Joins("LEFT JOIN team ON team.id = equipment.current_team_id").
		Joins("LEFT JOIN borrower ON borrower.id = equipment.current_borrower_id")

	if f.Q != "" {
		like := "%" + f.Q + "%"
		q = q.Where("(equipment.equipment_no LIKE ? OR equipment.name LIKE ? OR equipment.model LIKE ? OR equipment.internal_code LIKE ?)",
			like, like, like, like)
	}
	if cat := atoiUint(f.Category); cat > 0 {
		q = q.Where("equipment.category_id = ?", cat)
	}
	if tm := atoiUint(f.Team); tm > 0 {
		q = q.Where("equipment.current_team_id = ?", tm)
	}
	if f.Status != "" {
		q = q.Where("equipment.status = ?", f.Status)
	}
	q = q.Order("equipment.id ASC")

	var rows []exportRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	x := excelize.NewFile()
	sheet := "设备台账"
	x.NewSheet(sheet)
	_ = x.SetSheetRow(sheet, "A1", &[]any{
		"显示编号", "设备编号", "名称", "型号", "类别", "状态", "当前位置", "到达时间", "内部码", "备注", "更新时间",
	})
	for i, r := range rows {
		displayNo := DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount)
		no := ""
		if r.EquipmentNo != nil {
			no = *r.EquipmentNo
		}
		if no == "" {
			no = "无编号"
		}
		loc := ""
		switch r.Status {
		case models.StatusInStock:
			loc = "仓库（在库）"
		case models.StatusInTeam:
			loc = r.TeamName
		case models.StatusBorrowed:
			loc = r.BorrowerName
		case models.StatusMaintenance:
			loc = "维修中"
		case models.StatusScrapped:
			loc = "已报废"
		default:
			loc = r.Status
		}
		since := ""
		if r.CurrentSince.Valid {
			since = r.CurrentSince.Time.Format("2006-01-02 15:04:05")
		}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = x.SetSheetRow(sheet, cell, &[]any{
			displayNo, no, r.Name, r.Model, r.CategoryName, exportStatusText[r.Status], loc, since,
			r.InternalCode, r.Remark, r.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	if idx, err := x.GetSheetIndex(sheet); err == nil {
		x.SetActiveSheet(idx)
	}
	buf, err := x.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func atoiUint(s string) uint {
	v, _ := strconv.Atoi(s)
	if v < 0 {
		return 0
	}
	return uint(v)
}
