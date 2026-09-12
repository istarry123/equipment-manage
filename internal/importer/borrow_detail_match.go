package importer

import (
	"fmt"
	"sort"
	"strings"

	"equipment/internal/models"
	"equipment/internal/service"

	"gorm.io/gorm"
)

// 外借明细匹配层 —— v1.3 Phase 3（决策 19）。只读：不写库、不改状态。
//
// 匹配原则（用户口径 2026-09-11）：
//   - 以**编号为主**：equipment_no 精确命中现有设备台账；
//   - 同号多台时用 **编号 + 设备名称 + 型号** 进一步定位；
//   - 文件未给出名称/型号时，系统按库中候选顺序给出占位标签「预分配1、预分配2…」，
//     由用户在预览中确认（绝不静默替用户猜是哪一台）。
//
// 状态机（本层输出）：
//
//	UNIQUE    编号在库中唯一 → 可直接补录
//	BY_LABEL  编号多台，但文件给出的名称/型号（或 预分配N 标签）唯一命中 → 可直接补录
//	AMBIGUOUS 编号多台且无法定位 → 需人工选择（带 预分配N 候选与顺序建议）
//	MISSING   编号不在库中 → 需人工决定（口径⑤：按新建设备处理或跳过）
const (
	MatchUnique    = "UNIQUE"
	MatchByLabel   = "BY_LABEL"
	MatchAmbiguous = "AMBIGUOUS"
	MatchMissing   = "MISSING"
)

// PreAssignLabel 同号多台的占位标签（方案 A）：预分配1、预分配2…
func PreAssignLabel(i int) string { return fmt.Sprintf("预分配%d", i) }

// MatchCandidate 一个候选设备（同号多台时携带 预分配N 标签）。
type MatchCandidate struct {
	EquipmentID  uint   `json:"equipment_id"`
	InternalCode string `json:"internal_code"`
	EquipmentNo  string `json:"equipment_no"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	Category     string `json:"category"`
	Status       string `json:"status"`
	Seq          int    `json:"equipment_seq"`
	Label        string `json:"label"` // 预分配N（仅多台候选时有值）
	DisplayNo    string `json:"display_no"`
	IsCurrent    bool   `json:"is_current"` // 当前状态非在库（补录会改变现状，需确认）
}

// DetailMatchItem 一台明细设备的匹配结果。
type DetailMatchItem struct {
	SourceKey   string            `json:"source_key"`
	Row         int               `json:"row"`
	BlockRow    int               `json:"block_row"`
	BlockRows   string            `json:"block_rows"`
	EquipmentNo string            `json:"equipment_no"`
	Company     string            `json:"company"`
	BorrowDate  string            `json:"borrow_date"` // YYYY-MM-DD
	BorrowRaw   string            `json:"borrow_raw"`
	Occurrence  int               `json:"occurrence"`  // 该编号在本文件第几次出现（1 起）
	Occurrences int               `json:"occurrences"` // 该编号在本文件出现总次数
	Status      string            `json:"status"`
	Chosen      *MatchCandidate   `json:"chosen,omitempty"`
	Candidates  []*MatchCandidate `json:"candidates,omitempty"`
	SuggestedID uint              `json:"suggested_equipment_id,omitempty"` // 顺序建议（需用户确认）
	Note        string            `json:"note,omitempty"`
	Evidence    string            `json:"evidence,omitempty"` // MISSING 行的旁证（仅供参考）
}

// BorrowerPlan 外借方落库计划（Phase 4 执行；本层只做核对）。
type BorrowerPlan struct {
	Name       string `json:"name"`
	Exists     bool   `json:"exists"`
	BorrowerID uint   `json:"borrower_id,omitempty"`
	Devices    int    `json:"devices"`
	Blocks     int    `json:"blocks"`
}

// DetailMatchResult 匹配结果（供预览与后续写入）。
type DetailMatchResult struct {
	Filename   string `json:"filename"`
	SourceHash string `json:"source_hash"`
	Total      int    `json:"total"`

	Unique    int `json:"unique"`
	ByLabel   int `json:"by_label"`
	Ambiguous int `json:"ambiguous"`
	Missing   int `json:"missing"`

	Items     []*DetailMatchItem `json:"items"`
	Borrowers []*BorrowerPlan    `json:"borrowers"`
	Issues    []Issue            `json:"issues"`

	DupNos        []string `json:"dup_nos"`          // 文件内重复出现的编号
	SameNoMultiNo []string `json:"same_no_multi_no"` // 库中同号多台的编号
	MinBorrowDate string   `json:"min_borrow_date"`
	MaxBorrowDate string   `json:"max_borrow_date"`
}

// eqLabel 台账标签（供同号多台显示序号与 MISSING 旁证使用）。
type eqLabel struct {
	EquipmentNo *string
	Name        string
	Model       string
}

// MatchBorrowDetail 把解析结果与现有设备台账/外借方字典做匹配（只读）。
func MatchBorrowDetail(db *gorm.DB, res *DetailParseResult) (*DetailMatchResult, error) {
	out := &DetailMatchResult{Filename: res.Filename, SourceHash: res.SourceHash, Total: len(res.Devices)}

	// 1) 一次性取回涉及的设备（避免 N+1）
	nos := make([]string, 0, len(res.Devices))
	seenNo := map[string]bool{}
	for _, d := range res.Devices {
		if !seenNo[d.EquipmentNo] {
			seenNo[d.EquipmentNo] = true
			nos = append(nos, d.EquipmentNo)
		}
	}
	var allLabels []eqLabel
	if err := db.Model(&models.Equipment{}).Select("equipment_no, name, model").Find(&allLabels).Error; err != nil {
		return nil, fmt.Errorf("查询设备台账失败: %w", err)
	}
	groupCount := map[string]int64{}
	for i := range allLabels {
		l := allLabels[i]
		groupCount[service.SeqGroupKey(l.EquipmentNo, l.Name, l.Model)]++
	}

	byNo := map[string][]*MatchCandidate{}
	if len(nos) > 0 {
		var eqs []models.Equipment
		if err := db.Where("equipment_no IN ?", nos).Order("id ASC").Find(&eqs).Error; err != nil {
			return nil, fmt.Errorf("查询设备台账失败: %w", err)
		}
		catNames, err := categoryNameMap(db)
		if err != nil {
			return nil, err
		}
		for i := range eqs {
			e := &eqs[i]
			if e.EquipmentNo == nil {
				continue
			}
			no := *e.EquipmentNo
			c := &MatchCandidate{
				EquipmentID: e.ID, InternalCode: e.InternalCode, EquipmentNo: no,
				Name: e.Name, Model: e.Model, Status: e.Status, Seq: e.EquipmentSeq,
				IsCurrent: e.Status != models.StatusInStock,
			}
			if e.CategoryID != nil {
				c.Category = catNames[*e.CategoryID]
			}
			byNo[no] = append(byNo[no], c)
		}
	}

	// 2) 同号多台：按库中顺序赋 预分配N 标签（顺序稳定 = equipment.id 升序）
	for no, cands := range byNo {
		if len(cands) > 1 {
			out.SameNoMultiNo = append(out.SameNoMultiNo, no)
		}
		for i, c := range cands {
			noPtr := c.EquipmentNo
			c.DisplayNo = service.DisplayNo(&noPtr, c.Name, c.Model, c.Seq,
				groupCount[service.SeqGroupKey(&noPtr, c.Name, c.Model)])
			if len(cands) > 1 {
				c.Label = PreAssignLabel(i + 1)
			}
		}
	}
	sort.Strings(out.SameNoMultiNo)

	// 3) 文件内出现次数（用于顺序建议）
	occ := map[string]int{}
	for _, d := range res.Devices {
		occ[d.EquipmentNo]++
	}
	for no, n := range occ {
		if n > 1 {
			out.DupNos = append(out.DupNos, no)
		}
	}
	sort.Strings(out.DupNos)

	blockByRow := map[int]*BorrowDetailBlock{}
	for _, b := range res.Blocks {
		blockByRow[b.Row] = b
	}

	// 4) 逐台匹配（保持文件顺序）
	occurIdx := map[string]int{}
	borrowerAgg := map[string]*BorrowerPlan{}
	borrowerOrder := []string{}
	for _, d := range res.Devices {
		occurIdx[d.EquipmentNo]++
		blk := blockByRow[d.BlockRow]
		item := &DetailMatchItem{
			SourceKey: d.SourceKey, Row: d.Row, BlockRow: d.BlockRow, BlockRows: d.BlockRows,
			EquipmentNo: d.EquipmentNo, Occurrence: occurIdx[d.EquipmentNo], Occurrences: occ[d.EquipmentNo],
		}
		if blk != nil {
			item.Company = blk.Company
			item.BorrowRaw = blk.ArriveRaw
			if blk.ArriveAt != nil {
				item.BorrowDate = blk.ArriveAt.Format("2006-01-02")
				if out.MinBorrowDate == "" || item.BorrowDate < out.MinBorrowDate {
					out.MinBorrowDate = item.BorrowDate
				}
				if out.MaxBorrowDate == "" || item.BorrowDate > out.MaxBorrowDate {
					out.MaxBorrowDate = item.BorrowDate
				}
			}
		}
		cands := byNo[d.EquipmentNo]
		var blockName, blockModel string
		if blk != nil {
			blockName, blockModel = blk.Name, blk.Model
		}
		switch {
		case len(cands) == 0:
			item.Status = MatchMissing
			out.Missing++
			item.Note = "编号在现有台账中不存在；按口径⑤需新建该设备或跳过（名称/型号需人工确认）"
			item.Evidence = siblingEvidence(allLabels, d.EquipmentNo)
		case len(cands) == 1:
			item.Status = MatchUnique
			out.Unique++
			item.Chosen = cands[0]
		default:
			item.Candidates = cands
			if hit := byLabelMatch(cands, blockName, blockModel); hit != nil {
				item.Status = MatchByLabel
				out.ByLabel++
				item.Chosen = hit
			} else {
				item.Status = MatchAmbiguous
				out.Ambiguous++
				if item.Occurrences == len(cands) {
					// 文件内出现次数与库中台数相同 → 给出“按出现顺序对应 预分配N”的建议（仍需用户确认）
					item.SuggestedID = cands[item.Occurrence-1].EquipmentID
					item.Note = fmt.Sprintf("库中该编号 %d 台、文件内出现 %d 次：建议按出现顺序对应 %s（需确认）",
						len(cands), item.Occurrences, cands[item.Occurrence-1].Label)
				} else {
					item.Note = fmt.Sprintf("库中该编号 %d 台、文件内出现 %d 次，无法定位到具体设备 → 请人工选择",
						len(cands), item.Occurrences)
				}
			}
		}
		if item.Chosen != nil && item.Chosen.IsCurrent {
			out.Issues = append(out.Issues, Issue{Row: d.Row, Code: "V-M1", Level: "WARN",
				Message: fmt.Sprintf("设备 %s（%s）当前状态为 %s，补录外借会改变现状，请人工确认",
					item.Chosen.DisplayNo, item.Chosen.InternalCode, item.Chosen.Status)})
		}
		out.Items = append(out.Items, item)

		if blk != nil && blk.Company != "" {
			if borrowerAgg[blk.Company] == nil {
				borrowerAgg[blk.Company] = &BorrowerPlan{Name: blk.Company}
				borrowerOrder = append(borrowerOrder, blk.Company)
			}
			borrowerAgg[blk.Company].Devices++
		}
	}

	// 5) 外借方核对（复用/新建）
	var borrowers []models.Borrower
	if err := db.Find(&borrowers).Error; err != nil {
		return nil, fmt.Errorf("查询外借方字典失败: %w", err)
	}
	byName := map[string]uint{}
	for _, b := range borrowers {
		byName[strings.TrimSpace(b.Name)] = b.ID
	}
	blkCount := map[string]int{}
	for _, b := range res.Blocks {
		if b.Company != "" {
			blkCount[b.Company]++
		}
	}
	for _, name := range borrowerOrder {
		p := borrowerAgg[name]
		p.Blocks = blkCount[name]
		if id, ok := byName[name]; ok {
			p.Exists = true
			p.BorrowerID = id
		}
		out.Borrowers = append(out.Borrowers, p)
	}

	return out, nil
}

// byLabelMatch 编号多台时，用文件给出的 名称/型号（或 预分配N 标签）唯一定位一台；不唯一 → nil（交人工）。
func byLabelMatch(cands []*MatchCandidate, name, model string) *MatchCandidate {
	name = strings.TrimSpace(name)
	model = strings.TrimSpace(model)
	if name == "" && model == "" {
		return nil
	}
	// (1) 预分配N 标签（方案 A 的文件写法）
	for _, c := range cands {
		if c.Label != "" && (name == c.Label || model == c.Label) {
			return c
		}
	}
	// (2) 名称 + 型号 精确匹配（文件给出真实名称/型号时）
	var hit *MatchCandidate
	for _, c := range cands {
		if name != "" && c.Name != name {
			continue
		}
		if model != "" && c.Model != model {
			continue
		}
		if hit != nil {
			return nil
		}
		hit = c
	}
	return hit
}

// siblingEvidence MISSING 行的旁证：同长度、仅 1 位不同的编号在库中的名称/型号（仅供参考，不自动采用）。
func siblingEvidence(labels []eqLabel, no string) string {
	type nm struct{ name, model string }
	seen := map[nm]bool{}
	var list []string
	for i := range labels {
		l := labels[i]
		if l.EquipmentNo == nil || len([]rune(*l.EquipmentNo)) != len([]rune(no)) {
			continue
		}
		if diffRunes(no, *l.EquipmentNo) != 1 {
			continue
		}
		k := nm{l.Name, l.Model}
		if seen[k] {
			continue
		}
		seen[k] = true
		list = append(list, fmt.Sprintf("%s（%s，%s）", *l.EquipmentNo, l.Name, l.Model))
	}
	if len(list) == 0 {
		return ""
	}
	sort.Strings(list)
	if len(list) > 4 {
		list = list[:4]
	}
	return "同长度仅 1 位之差的编号在库中为：" + strings.Join(list, "、")
}

// diffRunes 两个等长字符串的不同字符数（长度不同返回 -1）。
func diffRunes(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) != len(rb) {
		return -1
	}
	n := 0
	for i := range ra {
		if ra[i] != rb[i] {
			n++
		}
	}
	return n
}

// categoryNameMap 类别 ID → 名称。
func categoryNameMap(db *gorm.DB) (map[uint]string, error) {
	var cats []models.Category
	if err := db.Find(&cats).Error; err != nil {
		return nil, fmt.Errorf("查询类别字典失败: %w", err)
	}
	m := make(map[uint]string, len(cats))
	for _, c := range cats {
		m[c.ID] = c.Name
	}
	return m, nil
}
