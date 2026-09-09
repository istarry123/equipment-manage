package importer

import (
	"fmt"
	"sort"
	"time"
)

// 设备最终状态推导（决策 18 §十九/§二十/§二十一；供 Preview(Phase 9) 与清空重导(Phase 10) 消费）。
// 规则：
//   - 导入默认全部 IN_STOCK（决策 18 ③）；绝不凭空制造归还记录（§二十）。
//   - 若设备的最后一条借出事件带有可可靠解析的归还日期（“入南库/回库”线索，
//     BorrowNumber.ReturnDate）→ 该次借出可判已归 → 状态建议仍 IN_STOCK，且历史保留。
//   - 若最后一条借出事件无归还证据 → 「疑似在借」候选（suspected），供 Preview 勾选；
//     勾选后才置 BORROWED（不自动设置，§二十：不能假设当前仍然借出）。
//   - 无法定位设备（REVIEW）/日期缺日/跨块编号 → needs_review 标记，不猜测。
// 本推导只读 ParseResult，不写库。

// DerivedStatus 一台设备推导出的最终状态建议。
const (
	StatusSuggestInStock   = "IN_STOCK"  // 建议在库（默认）
	StatusSuggestSuspected = "SUSPECTED" // 疑似在借：最后事件=借出且无可靠归还证据，需人工确认
	StatusSuggestReview    = "REVIEW"    // 无法可靠推导（事件解析/归属/日期问题），需人工核对
)

// EquipmentBorrow 一台设备匹配到的一条借出历史（时间倒序）。
type EquipmentBorrow struct {
	RowFrom     int        `json:"row_from"`
	Date        *time.Time `json:"date,omitempty"`
	DateRaw     string     `json:"date_raw"`
	Company     string     `json:"company"`
	Remark      string     `json:"remark,omitempty"`
	ReturnDate  *time.Time `json:"return_date,omitempty"` // 该次借出可靠解析的归还线索日期
	No          string     `json:"no,omitempty"`          // 事件引用的编号
	Unnumbered  bool       `json:"unnumbered"`
	EventReview bool       `json:"event_review"`
}

// DerivedEquipment 一台“拟导入设备”的状态推导结果（决策 18）。
type DerivedEquipment struct {
	GroupKey    string            `json:"group_key"` // name+model
	Name        string            `json:"name"`
	Model       string            `json:"model"`
	EquipmentNo *string           `json:"equipment_no"` // 编号设备才有；无编号为 nil
	Seq         int               `json:"seq"`          // 组内序号（无编号用）
	Unnumbered  bool              `json:"unnumbered"`
	Status      string            `json:"status"`  // StatusSuggest*
	Borrows     []EquipmentBorrow `json:"borrows"` // 命中事件（倒序）
	ReviewWhy   string            `json:"review_why,omitempty"`
}

// DerivationReport 汇总统计（Preview §三十一 口径）。
type DerivationReport struct {
	DeviceCount     int                 `json:"device_count"` // 拟导入设备台数
	Suspected       int                 `json:"suspected"`    // 疑似在借（最后借出无归还证据）
	SuspectedList   []*DerivedEquipment `json:"suspected_list,omitempty"`
	ReviewDevices   int                 `json:"review_devices"`    // 无法可靠推导的设备台数
	BorrowMatched   int                 `json:"borrow_matched"`    // 已匹配到设备的借出编号条目数
	BorrowUnmatched int                 `json:"borrow_unmatched"`  // 未能匹配到任何设备的借出编号条目数
	ReturnClueCount int                 `json:"return_clue_count"` // 解析到可靠归还日期的事件条目数
}

// DeriveEquipmentStatus 依据 F 资产全集 + J 事件推导每台拟导入设备的最终状态建议。
// 匹配规则（§四十原则 + W-4 跨块同号）：同一编号可能出现在多个 (name,model) 块，
// 因此事件首先按“事件所在块”匹配（BorrowEvent.Name/Model）；块内匹配不上时，
// 若该编号在全局设备中唯一（仅一个块出现过）也可跨块唯一命中，否则视为无法定位 → REVIEW。
func DeriveEquipmentStatus(res *ParseResult) *DerivationReport {
	rep := &DerivationReport{}

	// 1) 设备全集：group → 有编号 unit（编号可能重复=多台真机，decision18）与无编号台数
	devices := make([]*DerivedEquipment, 0, res.TotalD)
	// 按编号索引 → 命中的设备（全局，用于跨块唯一回退）
	byNo := map[string][]*DerivedEquipment{}
	// 块键 name+model
	type eq struct {
		dev *DerivedEquipment
	}
	blockUnnumbered := map[string]int{} // 块内无编号台数
	for _, g := range res.Groups {
		key := g.Name + "\x00" + g.Model
		// 有编号：Numbers 允许重复，每 token 一台真机
		for _, no := range g.Numbers {
			dev := &DerivedEquipment{
				GroupKey: key, Name: g.Name, Model: g.Model,
				EquipmentNo: ptr(no), Unnumbered: false,
				Status: StatusSuggestInStock,
			}
			devices = append(devices, dev)
			byNo[no] = append(byNo[no], dev)
		}
		// 无编号：逐台展开（Unnumbered 计数）
		blockUnnumbered[key] += g.Unnumbered
		for i := 0; i < g.Unnumbered; i++ {
			dev := &DerivedEquipment{
				GroupKey: key, Name: g.Name, Model: g.Model,
				EquipmentNo: nil, Seq: i + 1, Unnumbered: true,
				Status: StatusSuggestInStock,
			}
			devices = append(devices, dev)
		}
	}
	rep.DeviceCount = len(devices)

	// 编号 → 所在块数（跨块同号判定）
	noBlockCount := map[string]int{}
	for no, ds := range byNo {
		seen := map[string]bool{}
		for _, d := range ds {
			seen[d.GroupKey] = true
		}
		noBlockCount[no] = len(seen)
	}

	// 2) 事件匹配
	appendMatch := func(dev *DerivedEquipment, e *BorrowEvent, num *BorrowNumber) {
		eb := EquipmentBorrow{
			RowFrom: e.RowFrom, Date: e.Date, DateRaw: e.DateRaw,
			Company: e.Company, Remark: num.Remark,
			ReturnDate: num.ReturnDate, No: num.No,
			Unnumbered: num.Unnumbered, EventReview: e.NeedsReview || num.NeedsReview,
		}
		dev.Borrows = append(dev.Borrows, eb)
	}
	for _, e := range res.BorrowEvents {
		blockKey := e.Name + "\x00" + e.Model
		for i := range e.Numbers {
			num := &e.Numbers[i]
			matched := false
			if num.Unnumbered {
				// 无编号事件 → 匹配“事件所在块”的无编号设备（逐台不定位：全部该块无编号设备视为候选）
				if blockUnnumbered[blockKey] > 0 {
					rep.BorrowMatched++
					matched = true
					// 状态推导不逐台断言：这些无编号设备若有借出历史且无归还证据 → 疑似
					_ = num
				}
				continue
			}
			no := num.No
			if no == "" {
				if num.NeedsReview {
					rep.BorrowUnmatched++
				}
				continue
			}
			// 优先块内命中
			inBlock := false
			for _, dev := range byNo[no] {
				if dev.GroupKey == blockKey {
					inBlock = true
					appendMatch(dev, e, num)
					matched = true
				}
			}
			if matched {
				rep.BorrowMatched++
				_ = inBlock
				continue
			}
			// 跨块唯一回退：全局只出现一次的编号 → 命中唯一设备
			if noBlockCount[no] == 1 && len(byNo[no]) == 1 {
				appendMatch(byNo[no][0], e, num)
				rep.BorrowMatched++
				continue
			}
			rep.BorrowUnmatched++
		}
	}

	// 3) 排序 + 状态推导
	for _, dev := range devices {
		if len(dev.Borrows) == 0 {
			continue // 默认 IN_STOCK
		}
		// 时间倒序（无日期者置后，交 REVIEW）
		sort.SliceStable(dev.Borrows, func(i, j int) bool {
			di, dj := dev.Borrows[i].Date, dev.Borrows[j].Date
			if di == nil || dj == nil {
				if di == nil && dj == nil {
					return dev.Borrows[i].RowFrom > dev.Borrows[j].RowFrom
				}
				return dj == nil // 无日期排后
			}
			if !di.Equal(*dj) {
				return di.After(*dj)
			}
			return dev.Borrows[i].RowFrom > dev.Borrows[j].RowFrom
		})
		last := dev.Borrows[0]
		if last.ReturnDate != nil {
			// 最后一次借出带可靠归还日期 → 已回库（保持 IN_STOCK），历史保留
			continue
		}
		if last.Date == nil || last.EventReview {
			// 缺日/事件需人工 → 无法可靠推导，REVIEW
			dev.Status = StatusSuggestReview
			dev.ReviewWhy = "最近一条借出无法可靠判断是否已归还（日期缺失或事件需人工确认）"
			rep.ReviewDevices++
			continue
		}
		// 最后一次借出无归还证据 → 疑似在借（默认仍 IN_STOCK，Preview 勾选后才置 BORROWED）
		dev.Status = StatusSuggestSuspected
		rep.Suspected++
		rep.SuspectedList = append(rep.SuspectedList, dev)
	}
	sort.SliceStable(rep.SuspectedList, func(i, j int) bool {
		return suspectedSortKey(rep.SuspectedList[i]) < suspectedSortKey(rep.SuspectedList[j])
	})
	// 归还线索统计
	for _, e := range res.BorrowEvents {
		for i := range e.Numbers {
			if e.Numbers[i].ReturnDate != nil {
				rep.ReturnClueCount++
			}
		}
	}
	return rep
}

func suspectedSortKey(d *DerivedEquipment) string {
	no := ""
	if d.EquipmentNo != nil {
		no = *d.EquipmentNo
	}
	return fmt.Sprintf("%s|%s|%s|%d", d.GroupKey, no, d.Name, d.Seq)
}
