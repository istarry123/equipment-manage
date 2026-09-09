package service

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// 设备身份展示（决策 18）：供各聚合视图复用。
// EquipmentSelectNoGrp 同组(no,name,model)计数表达式（有编号恒>=1；无编号恒 0）。
const EquipmentSelectNoGrp = `CASE WHEN equipment.equipment_no IS NOT NULL
  THEN (SELECT COUNT(*) FROM equipment e2
        WHERE e2.equipment_no = equipment.equipment_no
          AND e2.name = equipment.name AND e2.model = equipment.model)
  ELSE 0 END AS no_grp_count`

// displayableRow 聚合行需要的展示字段基座（Scan 时由调用方 SELECT 带入）。
type displayableRow struct {
	EquipmentNo  *string `gorm:"column:equipment_no"`
	EquipmentSeq int     `gorm:"column:equipment_seq"`
	Name         string  `gorm:"column:name"`
	Model        string  `gorm:"column:model"`
	NoGrpCount   int64   `gorm:"column:no_grp_count"`
}

// DisplayNoOf 计算一行的展示编号（供聚合视图填充 DTO）。
func DisplayNoOf(r displayableRow) string {
	return DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount)
}

// ---------- 班组设备视图（Team Equipment View） ----------
// 数据原则（Feature 约束，禁止新增 team_equipment 类关系表）：
//   - 当前班组 = equipment.current_team_id，当前状态 = equipment.status，类别 = equipment.category_id；
//   - 仅 status=IN_TEAM 且有当前班组的设备进入“班组”分组；
//   - current_team_id IS NULL 的设备进入“未分配”（在库/外借/维修等；报废默认排除）；
//   - 不读 transaction 推断当前归属；history 零改动。

// TeamViewOptions 筛选（与既有设备查询风格一致）。
type TeamViewOptions struct {
	TeamID     *uint
	Unassigned bool
	CategoryID *uint
	Status     string
	Keyword    string
}

// TeamViewStats 顶部统计（全局口径）。
type TeamViewStats struct {
	TeamCount           int64 `json:"team_count"`           // 启用班组数
	TotalEquipment      int64 `json:"total_equipment"`      // 设备总数（= Dashboard 设备总数）
	InTeamEquipment     int64 `json:"in_team_equipment"`    // 班组使用中（= Dashboard IN_TEAM）
	UnassignedEquipment int64 `json:"unassigned_equipment"` // 未分配（current_team_id IS NULL 且非报废）
}

// TeamViewDevice 设备条目（点击编号可进既有设备详情）。
type TeamViewDevice struct {
	ID           uint    `json:"id"`
	EquipmentNo  *string `json:"equipment_no"`
	EquipmentSeq int     `json:"equipment_seq"`
	DisplayNo    string  `json:"display_no"`
	InternalCode string  `json:"internal_code"`
	Name         string  `json:"name"`
	Model        string  `json:"model"`
	Category     string  `json:"category"`
	CategoryID   *uint   `json:"category_id"`
	Status       string  `json:"status"`
	CurrentSince string  `json:"current_since"`
}

// CategoryGroup 类别分组。
type CategoryGroup struct {
	Category string           `json:"category"`
	Count    int              `json:"count"`
	Devices  []TeamViewDevice `json:"devices"`
}

// TeamGroup 班组分组（班组 → 类别 → 设备）。
type TeamGroup struct {
	ID         uint            `json:"id"`
	Name       string          `json:"name"`
	Total      int             `json:"total"`
	Categories []CategoryGroup `json:"categories"`
}

// TeamView 聚合结果。
type TeamView struct {
	Stats      TeamViewStats `json:"stats"`
	Teams      []TeamGroup   `json:"teams"`
	Unassigned CategoryGroup `json:"unassigned"` // 未分配设备（平铺设备列表）
	TotalRows  int           `json:"total_rows"`
}

type tvRow struct {
	ID            uint
	EquipmentNo   *string
	EquipmentSeq  int
	InternalCode  string
	Name          string
	Model         string
	Category      string
	CategoryID    *uint
	Status        string
	CurrentTeamID *uint
	TeamName      string
	CurrentSince  models.NullTime
	NoGrpCount    int64
}

func hasViewFilter(opt TeamViewOptions) bool {
	return opt.TeamID != nil || opt.Unassigned || opt.CategoryID != nil ||
		opt.Status != "" || strings.TrimSpace(opt.Keyword) != ""
}

// TeamEquipmentView 班组设备视图聚合：一次查询取数（避免 N+1），分组与自然排序在内存完成。
func TeamEquipmentView(db *gorm.DB, opt TeamViewOptions) (*TeamView, error) {
	q := db.Table("equipment").
		Select(`equipment.id, equipment.equipment_no, equipment.equipment_seq, equipment.internal_code, equipment.name, equipment.model,
			COALESCE(category.name,'') AS category, equipment.category_id, equipment.status,
			equipment.current_team_id, COALESCE(team.name,'') AS team_name, equipment.current_since, ` + EquipmentSelectNoGrp).
		Joins("LEFT JOIN category ON category.id = equipment.category_id").
		Joins("LEFT JOIN team ON team.id = equipment.current_team_id")

	if opt.Status != "" {
		q = q.Where("equipment.status = ?", opt.Status)
	} else {
		q = q.Where("equipment.status <> ?", models.StatusScrapped)
	}
	if opt.CategoryID != nil {
		q = q.Where("equipment.category_id = ?", *opt.CategoryID)
	}
	if opt.TeamID != nil {
		q = q.Where("equipment.current_team_id = ?", *opt.TeamID)
	}
	if opt.Unassigned {
		q = q.Where("equipment.current_team_id IS NULL")
	}
	if kw := strings.TrimSpace(opt.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(equipment.equipment_no LIKE ? OR equipment.name LIKE ? OR equipment.model LIKE ?)",
			like, like, like)
	}

	var rows []tvRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	view := &TeamView{
		Stats:      TeamViewStats{},
		Teams:      []TeamGroup{},
		Unassigned: CategoryGroup{Category: "未分配设备", Devices: []TeamViewDevice{}},
	}
	// 全局统计（与 Dashboard 同一口径）
	if err := db.Model(&models.Team{}).Where("is_active = ?", true).Count(&view.Stats.TeamCount).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.Equipment{}).Count(&view.Stats.TotalEquipment).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.Equipment{}).Where("status = ?", models.StatusInTeam).
		Count(&view.Stats.InTeamEquipment).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&models.Equipment{}).Where("current_team_id IS NULL AND status <> ?", models.StatusScrapped).
		Count(&view.Stats.UnassignedEquipment).Error; err != nil {
		return nil, err
	}

	teamIdx := map[uint]int{}
	for _, r := range rows {
		dev := TeamViewDevice{
			ID: r.ID, EquipmentNo: r.EquipmentNo, EquipmentSeq: r.EquipmentSeq,
			InternalCode: r.InternalCode,
			Name:         r.Name, Model: r.Model,
			Category:   orLabel(r.Category),
			CategoryID: r.CategoryID,
			Status:     r.Status,
			DisplayNo:  DisplayNo(r.EquipmentNo, r.Name, r.Model, r.EquipmentSeq, r.NoGrpCount),
		}
		if r.CurrentSince.Valid {
			dev.CurrentSince = r.CurrentSince.Time.Format("2006-01-02 15:04:05")
		}
		switch {
		case !opt.Unassigned && r.CurrentTeamID != nil && r.Status == models.StatusInTeam:
			// 班组使用中：进班组分组
			i, ok := teamIdx[*r.CurrentTeamID]
			if !ok {
				i = len(view.Teams)
				teamIdx[*r.CurrentTeamID] = i
				view.Teams = append(view.Teams, TeamGroup{ID: *r.CurrentTeamID, Name: orLabel(r.TeamName), Categories: []CategoryGroup{}})
			}
			g := &view.Teams[i]
			g.Total++
			view.TotalRows++
			addToCategory(&g.Categories, dev)
		case r.CurrentTeamID == nil:
			// 未分配设备（在库/外借/维修等）
			view.Unassigned.Count++
			view.Unassigned.Devices = append(view.Unassigned.Devices, dev)
			view.TotalRows++
		default:
			// 有班组引用但状态并非班组使用（异常态）：不计入任一分组，保持数据谨慎
		}
	}

	// 排序与空态补全
	if !hasViewFilter(opt) {
		// 无筛选时补全 0 设备班组（“暂无设备”空态）
		var teams []models.Team
		if err := db.Where("is_active = ?", true).Order("name ASC").Find(&teams).Error; err != nil {
			return nil, err
		}
		for _, t := range teams {
			if _, ok := teamIdx[t.ID]; !ok {
				view.Teams = append(view.Teams, TeamGroup{ID: t.ID, Name: t.Name, Categories: []CategoryGroup{}})
			}
		}
	}
	sort.Slice(view.Teams, func(i, j int) bool { return view.Teams[i].Name < view.Teams[j].Name })
	for i := range view.Teams {
		sort.Slice(view.Teams[i].Categories, func(a, b int) bool {
			return view.Teams[i].Categories[a].Category < view.Teams[i].Categories[b].Category
		})
		for j := range view.Teams[i].Categories {
			sort.Slice(view.Teams[i].Categories[j].Devices, func(a, b int) bool {
				return deviceLess(view.Teams[i].Categories[j].Devices[a], view.Teams[i].Categories[j].Devices[b])
			})
		}
	}
	sort.Slice(view.Unassigned.Devices, func(i, j int) bool {
		return deviceLess(view.Unassigned.Devices[i], view.Unassigned.Devices[j])
	})
	return view, nil
}

// addToCategory 追加设备到（或新建）类别分组。
func addToCategory(groups *[]CategoryGroup, d TeamViewDevice) {
	for i := range *groups {
		if (*groups)[i].Category == d.Category {
			(*groups)[i].Count++
			(*groups)[i].Devices = append((*groups)[i].Devices, d)
			return
		}
	}
	*groups = append(*groups, CategoryGroup{Category: d.Category, Count: 1, Devices: []TeamViewDevice{d}})
}

func orLabel(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(未分类)"
	}
	return strings.TrimSpace(s)
}

// deviceLess 设备自然排序：编号（数字段按数值、字母段按字典），无编号设备排在末尾。
func deviceLess(a, b TeamViewDevice) bool {
	ak, an := deviceSortKey(a)
	bk, bn := deviceSortKey(b)
	if !an && bn {
		return false
	}
	if an && !bn {
		return true
	}
	if an || bn {
		return a.ID < b.ID
	}
	return naturalLess(ak, bk)
}

func deviceSortKey(d TeamViewDevice) (string, bool) {
	if d.EquipmentNo == nil || strings.TrimSpace(*d.EquipmentNo) == "" {
		return "", true // no number → 末尾
	}
	return *d.EquipmentNo, false
}

// naturalLess 数字感知比较："6061"<"6062"、"6062"<"60610"、"7020"<"8021"。
func naturalLess(a, b string) bool {
	sa, sb := splitKey(a), splitKey(b)
	for i := 0; i < len(sa) && i < len(sb); i++ {
		ca, cb := sa[i], sb[i]
		if ca.digit != cb.digit {
			// 数字段与文本段：文本 > 数字
			if ca.digit {
				return true
			}
			if cb.digit {
				return false
			}
		}
		if ca.digit {
			if ca.num != cb.num {
				return ca.num < cb.num
			}
		} else if ca.text != cb.text {
			return ca.text < cb.text
		}
	}
	return len(sa) < len(sb)
}

type keySeg struct {
	digit bool
	num   int64
	text  string
}

func splitKey(s string) []keySeg {
	var out []keySeg
	for i := 0; i < len(s); {
		r := rune(s[i])
		if isDigitRune(r) {
			j := i
			for j < len(s) && isDigitRune(rune(s[j])) {
				j++
			}
			n, _ := strconv.ParseInt(s[i:j], 10, 64)
			out = append(out, keySeg{digit: true, num: n})
			i = j
		} else {
			j := i
			for j < len(s) && !isDigitRune(rune(s[j])) {
				j++
			}
			out = append(out, keySeg{digit: false, text: s[i:j]})
			i = j
		}
	}
	return out
}

func isDigitRune(r rune) bool { return unicode.IsDigit(r) }
