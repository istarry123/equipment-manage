package service

import (
	"time"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// 设备流转情况导出（决策：Equipment Flow Export v1.1）——查询层。
// 数据原则：equipment 当前字段 = 当前状态；flow_record = 历史；绝不重读原始 Excel。
// 设备身份：equipment.id 唯一；display_no 仅展示（复用 service.DisplayNo）。

// FlowExportFilter 流转导出筛选（与查询层一致；三 Sheet 共用同一筛选范围）。
type FlowExportFilter struct {
	TeamID     *uint  // 当前班组筛选
	Status     string // 当前状态筛选（状态码）
	BorrowerID *uint  // 当前外借公司筛选
	// 数据范围开关（决定返回哪些部分；统计始终在筛选范围内计算）
	IncludeCurrent  bool
	IncludeHistory  bool
	IncludeSummary  bool
}

// FlowCurrentRow Sheet1「当前流转情况」一行 = 一台真实设备。
type FlowCurrentRow struct {
	ID           uint       // equipment.id（技术身份，Sheet 隐藏列用）
	Name         string     // 设备名称
	Model        string     // 设备型号
	DisplayNo    string     // 后端计算的显示编号（6041（1）/ JUKI DDL-8700（1））
	EquipmentNo  string     // 原始编号；无编号设备为空
	TeamName     string     // 当前所属班组（非 IN_TEAM 时空）
	Status       string     // 状态码
	StatusText   string     // 状态中文（复用 exportStatusText）
	Location     string     // 当前所在位置（复用现有口径计算）
	BorrowerName string     // 当前外借公司（非 BORROWED 时空）
	BorrowDate   *time.Time // 当前外借日期（非 BORROWED 时 nil）
	Remark       string     // 备注
}

// FlowHistoryRow Sheet2「流转历史」一行 = 一条 flow_record。
type FlowHistoryRow struct {
	EquipmentID  uint      // equipment.id（历史按此关联，绝不按 equipment_no）
	Name         string    // 设备名称（v1.1 锁定口径：显示当前名称）
	Model        string    // 设备型号（当前值）
	DisplayNo    string    // 设备显示编号（当前值）
	OccurredAt   time.Time // 流转时间
	Action       string    // 动作码
	ActionText   string    // 动作中文（复用 flowActionText）
	FromLocation string    // 原位置（由 from_status + 名称快照推导）
	ToLocation   string    // 新位置（由 to_status + 名称快照推导）
	TeamName     string    // 班组名快照（to_team_name，空则 from_team_name）
	BorrowerName string    // 外借公司名快照
	Remark       string    // 操作备注
}

// FlowCount 一组「名称 → 数量」统计项。
type FlowCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// FlowSummary Sheet3「统计汇总」（在筛选范围内统计）。
type FlowSummary struct {
	ByStatus   []FlowCount // 按状态统计（补全六态零值）
	ByTeam     []FlowCount // 按班组统计（仅当前 IN_TEAM）
	ByBorrower []FlowCount // 按外借公司统计（仅当前 BORROWED）
}

// FlowExportData 流转导出的完整查询结果。
type FlowExportData struct {
	Current  []FlowCurrentRow  // 当前流转情况
	History  []FlowHistoryRow  // 流转历史
	Summary  FlowSummary       // 统计汇总
	Total    int64             // 筛选范围内设备总数
}

// flowCurrentScan 当前设备批量查询的 Scan 结构（含 display_no 所需的同组计数）。
type flowCurrentScan struct {
	ID           uint
	EquipmentNo  *string
	EquipmentSeq int
	Name         string
	Model        string
	Status       string
	TeamName     string
	BorrowerName string
	BorrowDate   models.NullTime
	Remark       string
	NoGrpCount   int64
}

// flowHistoryScan 历史记录批量查询的 Scan 结构。
type flowHistoryScan struct {
	EquipmentID  uint
	EquipmentNo  *string
	EquipmentSeq int
	Name         string
	Model        string
	Action       string
	FromStatus   string
	ToStatus     string
	FromTeamName string
	ToTeamName   string
	BorrowerName string
	OccurredAt   models.Time
	Remark       string
	NoGrpCount   int64
}

// flowFilterWhere 把筛选条件应用到 equipment 基础查询（当前/历史/统计三处复用，保证口径一致）。
func flowFilterWhere(q *gorm.DB, f FlowExportFilter) *gorm.DB {
	if f.TeamID != nil {
		q = q.Where("equipment.current_team_id = ?", *f.TeamID)
	}
	if f.Status != "" {
		q = q.Where("equipment.status = ?", f.Status)
	}
	if f.BorrowerID != nil {
		q = q.Where("equipment.current_borrower_id = ?", *f.BorrowerID)
	}
	return q
}

// BuildFlowExportData 一次取数组装三部分数据（避免 N+1：当前/历史/统计各一次批量查询）。
func BuildFlowExportData(db *gorm.DB, f FlowExportFilter) (*FlowExportData, error) {
	out := &FlowExportData{
		Current: []FlowCurrentRow{},
		History: []FlowHistoryRow{},
		Summary: FlowSummary{ByStatus: []FlowCount{}, ByTeam: []FlowCount{}, ByBorrower: []FlowCount{}},
	}

	if f.IncludeCurrent || f.IncludeSummary {
		rows, err := queryFlowCurrent(db, f)
		if err != nil {
			return nil, err
		}
		out.Total = int64(len(rows))
		if f.IncludeCurrent {
			out.Current = toCurrentRows(rows)
		}
		if f.IncludeSummary {
			sum, err := summarizeRows(db, f, rows)
			if err != nil {
				return nil, err
			}
			out.Summary = sum
		}
	} else {
		// 仅历史：仍需总数（统计范围一致），不查询整组当前行以省内存
		if err := flowCountEquipment(db, f, &out.Total); err != nil {
			return nil, err
		}
	}

	if f.IncludeHistory {
		rows, err := queryFlowHistory(db, f)
		if err != nil {
			return nil, err
		}
		out.History = toHistoryRows(rows)
	}

	return out, nil
}

// queryFlowCurrent 当前设备批量查询（LEFT JOIN team/borrower/borrow_record，一次取全）。
func queryFlowCurrent(db *gorm.DB, f FlowExportFilter) ([]flowCurrentScan, error) {
	q := db.Table("equipment").
		Select(`equipment.id, equipment.equipment_no, equipment.equipment_seq, equipment.name, equipment.model,
			equipment.status, COALESCE(team.name,'') AS team_name, COALESCE(borrower.name,'') AS borrower_name,
			borrow_record.borrow_date AS borrow_date, equipment.remark, ` + EquipmentSelectNoGrp).
		Joins("LEFT JOIN team ON team.id = equipment.current_team_id").
		Joins("LEFT JOIN borrower ON borrower.id = equipment.current_borrower_id").
		Joins("LEFT JOIN borrow_record ON borrow_record.id = equipment.current_borrow_record_id AND borrow_record.status = ?", models.BorrowOutstanding)
	q = flowFilterWhere(q, f)
	// 稳定排序：名称 → 型号 → 组内序号 → id（与台账导出一致）
	q = q.Order("equipment.name ASC, equipment.model ASC, equipment.equipment_seq ASC, equipment.id ASC")

	var rows []flowCurrentScan
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// toCurrentRows 转 DTO，计算 display_no / 状态中文 / 当前位置。
func toCurrentRows(rows []flowCurrentScan) []FlowCurrentRow {
	out := make([]FlowCurrentRow, 0, len(rows))
	for _, r := range rows {
		no := ""
		if r.EquipmentNo != nil {
			no = *r.EquipmentNo
		}
		var bd *time.Time
		if r.BorrowDate.Valid {
			t := r.BorrowDate.Time
			bd = &t
		}
		out = append(out, FlowCurrentRow{
			ID:           r.ID,
			Name:         r.Name,
			Model:        r.Model,
			DisplayNo:    DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount),
			EquipmentNo:  no,
			TeamName:     r.TeamName,
			Status:       r.Status,
			StatusText:   statusText(r.Status),
			Location:     currentLocation(r.Status, r.TeamName, r.BorrowerName),
			BorrowerName: r.BorrowerName,
			BorrowDate:   bd,
			Remark:       r.Remark,
		})
	}
	return out
}

// queryFlowHistory 历史记录批量查询（JOIN equipment 取当前名称/编号/display_no）。
// 关键：按 equipment.id 关联；筛选条件作用于 equipment 当前状态（§二十六 语义）。
func queryFlowHistory(db *gorm.DB, f FlowExportFilter) ([]flowHistoryScan, error) {
	q := db.Table("flow_record").
		Select(`flow_record.equipment_id, equipment.equipment_no, equipment.equipment_seq,
			equipment.name, equipment.model, flow_record.action, flow_record.from_status, flow_record.to_status,
			flow_record.from_team_name, flow_record.to_team_name, flow_record.borrower_name,
			flow_record.occurred_at, flow_record.remark, ` + EquipmentSelectNoGrp).
		Joins("JOIN equipment ON equipment.id = flow_record.equipment_id")
	q = flowFilterWhere(q, f)
	// 最新 → 最旧（Prompt §十五推荐），同刻按 id 稳定
	q = q.Order("flow_record.occurred_at DESC, flow_record.id DESC")

	var rows []flowHistoryScan
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// toHistoryRows 转 DTO，推导原/新位置。
func toHistoryRows(rows []flowHistoryScan) []FlowHistoryRow {
	out := make([]FlowHistoryRow, 0, len(rows))
	for _, r := range rows {
		// 外借公司名在「借出」时属于新位置、在「归还」时属于原位置
		fromBorrower, toBorrower := "", ""
		switch {
		case r.FromStatus == models.StatusBorrowed:
			fromBorrower = r.BorrowerName
		case r.ToStatus == models.StatusBorrowed:
			toBorrower = r.BorrowerName
		}
		teamName := r.ToTeamName
		if teamName == "" {
			teamName = r.FromTeamName
		}
		out = append(out, FlowHistoryRow{
			EquipmentID:  r.EquipmentID,
			Name:         r.Name,
			Model:        r.Model,
			DisplayNo:    DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount),
			OccurredAt:   r.OccurredAt.Time,
			Action:       r.Action,
			ActionText:   flowActionText[r.Action],
			FromLocation: historyLocation(r.FromStatus, r.FromTeamName, fromBorrower),
			ToLocation:   historyLocation(r.ToStatus, r.ToTeamName, toBorrower),
			TeamName:     teamName,
			BorrowerName: r.BorrowerName,
			Remark:       r.Remark,
		})
	}
	return out
}

// statusText 状态码 → 中文（复用 exportStatusText，保证与页面/Dashboard/台账导出一致）。
func statusText(code string) string {
	if s, ok := exportStatusText[code]; ok {
		return s
	}
	return code
}

// currentLocation 当前所在位置（复用现有口径，不新增存储字段）。
func currentLocation(status, teamName, borrowerName string) string {
	switch status {
	case models.StatusInStock:
		return "在库"
	case models.StatusInTeam:
		return teamName
	case models.StatusBorrowed:
		return borrowerName
	case models.StatusMaintenance:
		return "维修"
	case models.StatusScrapped:
		return "报废"
	default:
		return "未分配"
	}
}

// historyLocation 历史位置：由状态 + 名称快照推导（from 侧与 to 侧分别调用）。
func historyLocation(status, teamName, borrowerName string) string {
	switch status {
	case models.StatusInStock:
		return "在库"
	case models.StatusInTeam:
		if teamName != "" {
			return teamName
		}
		return "班组"
	case models.StatusBorrowed:
		if borrowerName != "" {
			return borrowerName
		}
		return "外借"
	case models.StatusMaintenance:
		return "维修"
	case models.StatusScrapped:
		return "报废"
	case "-": // IMPORT_INIT 初始：无历史位置
		return "—"
	default:
		return status
	}
}

// flowCountEquipment 仅统计筛选范围内设备数（用于只导出历史、不查整组当前行的场景）。
func flowCountEquipment(db *gorm.DB, f FlowExportFilter, out *int64) error {
	q := db.Model(&models.Equipment{})
	q = flowFilterWhere(q, f)
	return q.Count(out).Error
}

// summarizeRows 在筛选范围内做三组统计（直接基于已查出的当前行，避免额外 SQL）。
func summarizeRows(db *gorm.DB, f FlowExportFilter, rows []flowCurrentScan) (FlowSummary, error) {
	sum := FlowSummary{ByStatus: []FlowCount{}, ByTeam: []FlowCount{}, ByBorrower: []FlowCount{}}

	// 1) 按状态统计（补全六态零值，与 Dashboard 同口径）
	statusCount := map[string]int64{}
	teamCount := map[string]int64{}
	borrowerCount := map[string]int64{}
	for _, r := range rows {
		statusCount[r.Status]++
		if r.Status == models.StatusInTeam && r.TeamName != "" {
			teamCount[r.TeamName]++
		}
		if r.Status == models.StatusBorrowed && r.BorrowerName != "" {
			borrowerCount[r.BorrowerName]++
		}
	}
	for _, code := range []string{
		models.StatusInStock, models.StatusInTeam, models.StatusBorrowed,
		models.StatusMaintenance, models.StatusScrapped, models.StatusOther,
	} {
		sum.ByStatus = append(sum.ByStatus, FlowCount{Name: statusText(code), Count: statusCount[code]})
	}
	sum.ByTeam = sortedNameCount(teamCount)
	sum.ByBorrower = sortedNameCount(borrowerCount)
	return sum, nil
}

// sortedNameCount 把 map 转为按数量降序、名称升序的稳定列表。
func sortedNameCount(m map[string]int64) []FlowCount {
	out := make([]FlowCount, 0, len(m))
	for name, c := range m {
		out = append(out, FlowCount{Name: name, Count: c})
	}
	// 冒泡稳定排序：数量降序；同数量按名称升序
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Count > out[i].Count || (out[j].Count == out[i].Count && out[j].Name < out[i].Name) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
