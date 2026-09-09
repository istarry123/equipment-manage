package importer

import (
	"fmt"
	"sort"
	"strings"

	"equipment/internal/service"
)

// Preview 设备级导入预览（决策 18 §三十二/§三十三；供 Import Preview/Review，Phase 9）。
// 预览展开必须与最终导入完全一致：编号每 token 一台、无编号按数量逐台、
// equipment_seq 分组键 = service.SeqGroupKey（与 RenumberAllSeq/导入一致）。

// PreviewDevice 一台待导入设备的预览行。
type PreviewDevice struct {
	SourceKey  string  `json:"source_key"` // 来源定位（R..-R..#Nn / #Un），导入后写入 equipment.source_key
	Category   string  `json:"category"`
	Name       string  `json:"name"`
	Model      string  `json:"model"`
	No         *string `json:"equipment_no"`  // 编号原样；无编号为 nil
	Seq        int     `json:"equipment_seq"` // 组内展示序号（预览口径 = 导入后）
	DisplayNo  string  `json:"display_no"`
	Unnumbered bool    `json:"unnumbered"`
	GroupRows  string  `json:"group_rows"` // 源行区间（如 R141-R193），供定位
	OK         bool    `json:"ok"`         // 该组是否可导入（OK 组）
	Review     string  `json:"review_why,omitempty"`
}

// ExpandPreviewDevices 把 ParseResult 展开为设备级预览行（顺序 = 导入写入顺序）。
func ExpandPreviewDevices(res *ParseResult) []*PreviewDevice {
	var out []*PreviewDevice
	type bucket struct{ rows []*PreviewDevice }
	buckets := map[string]*bucket{}
	order := []string{}

	for _, g := range res.Groups {
		rowsSpan := fmt.Sprintf("R%d-R%d", g.RowFrom, g.RowTo)
		ok := g.OK
		reviewWhy := ""
		if !g.OK {
			reviewWhy = "该组存在 BLOCK/REVIEW 校验问题，请先处理"
		}
		// 有编号：每 token 一台真机（含同号多台）
		for i, no := range g.Numbers {
			noV := no
			dev := &PreviewDevice{
				SourceKey: fmt.Sprintf("R%d-R%d#N%d", g.RowFrom, g.RowTo, i+1),
				Category:  g.Category, Name: g.Name, Model: g.Model,
				No: &noV, GroupRows: rowsSpan, OK: ok, Review: reviewWhy,
			}
			out = append(out, dev)
			bk := service.SeqGroupKey(&noV, g.Name, g.Model)
			b, found := buckets[bk]
			if !found {
				b = &bucket{}
				buckets[bk] = b
				order = append(order, bk)
			}
			b.rows = append(b.rows, dev)
		}
		// 无编号：逐台展开
		for i := 0; i < g.Unnumbered; i++ {
			dev := &PreviewDevice{
				SourceKey: fmt.Sprintf("R%d-R%d#U%d", g.RowFrom, g.RowTo, i+1),
				Category:  g.Category, Name: g.Name, Model: g.Model,
				No: nil, GroupRows: rowsSpan, OK: ok, Review: reviewWhy,
				Unnumbered: true,
			}
			out = append(out, dev)
			bk := service.SeqGroupKey(nil, g.Name, g.Model)
			b, found := buckets[bk]
			if !found {
				b = &bucket{}
				buckets[bk] = b
				order = append(order, bk)
			}
			b.rows = append(b.rows, dev)
		}
	}
	// 回填 seq 与 display_no（组大小 = 桶行数）
	for _, bk := range order {
		b := buckets[bk]
		n := len(b.rows)
		for i, dev := range b.rows {
			dev.Seq = i + 1
			dev.DisplayNo = service.DisplayNo(dev.No, dev.Name, dev.Model, dev.Seq, int64(n))
		}
	}
	return out
}

// SuspectCandidate 「疑似在借」勾选候选（仅外部公司，决策 15/18；§三十二）。
type SuspectCandidate struct {
	SourceKey  string `json:"source_key"`
	DisplayNo  string `json:"display_no"`
	Name       string `json:"name"`
	Model      string `json:"model"`
	Company    string `json:"company"`     // 最近一次借出公司
	BorrowDate string `json:"borrow_date"` // 最近一次借出日期
	Row        int    `json:"row"`         // 最近事件行
	Remark     string `json:"remark,omitempty"`
}

// ReviewItem 「需人工确认」条目（§三十三）：REVIEW 未清点前禁止最终导入。
type ReviewItem struct {
	Key        string `json:"key"` // 稳定 id（前端回传用）
	Level      string `json:"level"`
	Group      string `json:"group,omitempty"`
	Row        int    `json:"row"`
	Message    string `json:"message"`
	Raw        string `json:"raw,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// isInternalCompany 判断公司是否为内部单位（含“双发”，决策 15）。
func isInternalCompany(company string) bool {
	return strings.Contains(company, "双发")
}

// BuildReviewView 汇总 Preview/Review 数据：
//   - devices：设备级预览行（§三十二）
//   - suspected：外部公司、最后借出无归还证据的疑似在借候选（勾选 → 导入置 BORROWED）
//   - reviews：REVIEW 项清单（§三十三，未清点禁止导入）
//   - borrowEvents / internalEvents 计数（§三十一 汇总；内部=双发系）
func BuildReviewView(res *ParseResult) (devices []*PreviewDevice, suspected []*SuspectCandidate, reviews []*ReviewItem, borrowEvents, internalEvents int) {
	devices = ExpandPreviewDevices(res)
	// REVIEW 项集合（顺序累积）
	var revs []*ReviewItem
	seen := map[string]bool{}
	add := func(k, group string, row int, msg, raw, sugg string) {
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		revs = append(revs, &ReviewItem{Key: k, Level: IssueLevelReview,
			Group: group, Row: row, Message: msg, Raw: raw, Suggestion: sugg})
	}

	for _, e := range res.BorrowEvents {
		borrowEvents++
		if isInternalCompany(e.Company) {
			internalEvents++
		}
	}

	if res.Derived != nil {
		for _, su := range res.Derived.SuspectedList {
			if len(su.Borrows) == 0 {
				continue
			}
			last := su.Borrows[0]
			if isInternalCompany(last.Company) {
				continue // 内部单位 → 内部调拨历史，不入外部疑似清单
			}
			// 同号多台（组内 >1 台）无法定位具体一台 → 不猜，转 REVIEW
			if su.EquipmentNo != nil && previewCountInGroup(devices, su) > 1 {
				add(fmt.Sprintf("amb:%s:%s", *su.EquipmentNo, su.GroupKey), su.Name+"/"+su.Model, last.RowFrom,
					fmt.Sprintf("编号 %s 在该组多台（共 %d 台），借出事件无法定位到具体一台，请人工核对后处理", *su.EquipmentNo, previewCountInGroup(devices, su)),
					last.Remark, "")
				continue
			}
			found := locatePreviewDevice(devices, su)
			if found == nil {
				continue
			}
			date := ""
			if last.Date != nil {
				date = last.Date.Format("2006-01-02")
			} else {
				date = last.DateRaw
			}
			suspected = append(suspected, &SuspectCandidate{
				SourceKey: found.SourceKey,
				DisplayNo: found.DisplayNo,
				Name:      found.Name, Model: found.Model,
				Company:    last.Company,
				BorrowDate: date,
				Row:        last.RowFrom,
				Remark:     last.Remark,
			})
		}
		sort.SliceStable(suspected, func(i, j int) bool {
			if suspected[i].DisplayNo != suspected[j].DisplayNo {
				return suspected[i].DisplayNo < suspected[j].DisplayNo
			}
			return suspected[i].SourceKey < suspected[j].SourceKey
		})
	}

	// ParseResult.Issues 中的 REVIEW
	for _, is := range res.Issues {
		if is.Level == IssueLevelReview {
			add(fmt.Sprintf("issue:%d:%s", is.Row, is.Code), is.Group, is.Row, is.Message, "", "")
		}
	}
	// 事件级 REVIEW
	for _, e := range res.BorrowEvents {
		if !e.NeedsReview {
			continue
		}
		raw := e.JRaw
		if raw == "" {
			raw = e.DateRaw + " " + e.Company
		}
		for _, why := range e.ReviewWhys {
			add(fmt.Sprintf("event:%d:%s", e.RowFrom, why), e.Name+"/"+e.Model, e.RowFrom,
				fmt.Sprintf("借出事件 R%d 需人工确认：%s", e.RowFrom, why), raw, "")
		}
	}
	reviews = revs
	return devices, suspected, reviews, borrowEvents, internalEvents
}

// previewCountInGroup 组内(no+name+model 或同无编号组)的预览行数。
func previewCountInGroup(devices []*PreviewDevice, su *DerivedEquipment) int {
	n := 0
	for _, d := range devices {
		if d.Name != su.Name || d.Model != su.Model {
			continue
		}
		if su.EquipmentNo != nil {
			if d.No != nil && *d.No == *su.EquipmentNo {
				n++
			}
		} else if d.Unnumbered {
			n++
		}
	}
	return n
}

// locatePreviewDevice 在预览设备行中定位推导命中的设备。
// 编号设备：组内编号唯一（歧义已由 previewCountInGroup 转 REVIEW），按 组+编号 匹配；
// 无编号设备：按 组+seq 匹配（seq 由推导按无编号组序号生成，与预览一致）。
func locatePreviewDevice(devices []*PreviewDevice, su *DerivedEquipment) *PreviewDevice {
	if su.EquipmentNo != nil {
		for _, d := range devices {
			if d.Name == su.Name && d.Model == su.Model &&
				d.No != nil && *d.No == *su.EquipmentNo {
				return d
			}
		}
		return nil
	}
	for _, d := range devices {
		if d.Name == su.Name && d.Model == su.Model && d.Unnumbered && d.Seq == su.Seq {
			return d
		}
	}
	return nil
}
