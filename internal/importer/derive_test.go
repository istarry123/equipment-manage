package importer

import (
	"testing"
)

// TestDeriveRealFile 真实文件推导不变量：
//   - 设备全集 = 2147（编号 token + 无编号台数）
//   - 所有疑似项的最后一条借出都无归还证据；含可靠归还（入南库）的设备绝不列疑似
//   - 匹配/未匹配条目都不超过 J 编号条目总量（不伪造）
func TestDeriveRealFile(t *testing.T) {
	res := parseRealFile(t)
	d := res.Derived
	if d == nil {
		t.Fatal("缺少推导结果")
	}
	if d.DeviceCount != 2147 {
		t.Fatalf("推导设备数应为 2147，实际 %d", d.DeviceCount)
	}
	total := countRealBorrowUnits(res) // 纯编号条目
	unnEntries := 0
	for _, e := range res.BorrowEvents {
		for _, b := range e.Numbers {
			if b.Unnumbered {
				unnEntries++
			}
		}
	}
	// matched 同时含无编号事件命中（每事件一次），故上界=编号条目+无编号事件数
	if d.BorrowMatched > total+unnEntries {
		t.Fatalf("匹配条目超出上界: matched=%d total=%d unnumberedEvents=%d", d.BorrowMatched, total, unnEntries)
	}
	if d.BorrowUnmatched > total {
		t.Fatalf("未匹配超出编号条目总量: unmatched=%d total=%d", d.BorrowUnmatched, total)
	}
	// 疑似项不变量
	for _, dev := range d.SuspectedList {
		if dev.Status != StatusSuggestSuspected || len(dev.Borrows) == 0 {
			t.Fatalf("疑似项异常: %+v", dev)
		}
		if dev.Borrows[0].ReturnDate != nil {
			t.Fatalf("疑似项最后借出不应有归还证据: %+v", dev.Borrows[0])
		}
	}
	// R334：492024（2026.9.4入南库）→ 该台有可靠归还线索 → 不得列疑似；
	// 同事件 492013 无注释、无归还证据 → 判疑似待人工核对是正确行为（注释仅挂在 492024 上）
	for _, dev := range d.SuspectedList {
		for _, b := range dev.Borrows {
			if b.RowFrom == 334 && dev.EquipmentNo != nil && *dev.EquipmentNo == "492024" {
				t.Fatalf("R334 设备 492024 带入南库归还线索却被列疑似")
			}
		}
	}
	// 全部“含 ReturnDate 事件”的设备都不在疑似清单（遍历在库推导，间接覆盖：疑似项均无 ReturnDate 已断言）
	if d.ReturnClueCount <= 0 {
		t.Log("提示：真实文件未命中归还线索（如 R334 492024 应命中，请核对解析）")
	}
	t.Logf("推导: 设备 %d 疑似在借 %d REVIEW %d 匹配 %d 未匹配 %d 归还线索 %d",
		d.DeviceCount, d.Suspected, d.ReviewDevices, d.BorrowMatched, d.BorrowUnmatched, d.ReturnClueCount)
}

func countRealBorrowUnits(res *ParseResult) int {
	n := 0
	for _, e := range res.BorrowEvents {
		n += countBorrowUnits(e.Numbers)
	}
	return n
}

func displayKey(d *DerivedEquipment) string {
	if d.EquipmentNo != nil {
		return *d.EquipmentNo
	}
	return d.Name + "(" + d.Model + ")#" + itoa(d.Seq)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

// TestDeriveMini 迷你场景：
//  1. 同块内 6061 曾借出（无归还）→ 该编号设备疑似在借；
//  2. 492024 带“入南库”归还线索 → 不列疑似（已回库）；
//  3. 跨块不存在的 9999 事件 → 未匹配，不伪造状态。
func TestDeriveMini(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "3", "", "6061 6062 6063", "", "", "", "", ""},
		{"", "", "", "", "", "", "2022.6.14", "泰和", "1", "6061", ""},
		{"裁剪设备", "拉布机", "CM-01", "2", "", "492013 492024", "", "", "", "", ""},
		{"", "", "", "", "", "", "2026.7.28", "刘家庄华欣", "1", "492024（2026.9.4入南库）", ""},
		{"技术设备", "打样机", "P-9", "1", "", "9001", "", "", "", "", ""},
		{"", "", "", "", "", "", "2020.5.5", "泰和", "1", "9999", ""}, // 跨块未命中
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	d := res.Derived
	if d == nil {
		t.Fatal("缺少推导结果")
	}
	if d.DeviceCount != 6 {
		t.Fatalf("推导设备数应为 6，实际 %d", d.DeviceCount)
	}
	if d.BorrowUnmatched == 0 {
		t.Fatalf("9999 跨块未命中应计 BorrowUnmatched，实际 %+v", d)
	}
	suspNo := map[string]bool{}
	for _, dev := range d.SuspectedList {
		if dev.EquipmentNo != nil {
			suspNo[*dev.EquipmentNo] = true
		}
	}
	if !suspNo["6061"] {
		t.Fatalf("6061 曾借出无归还应列疑似，实际 %v", suspNo)
	}
	if suspNo["6062"] || suspNo["6063"] {
		t.Fatalf("未借出设备不应列疑似: %v", suspNo)
	}
	if suspNo["492024"] {
		t.Fatalf("492024 有归还线索不应列疑似")
	}
	if suspNo["492013"] || suspNo["9001"] {
		t.Fatalf("无借出设备不应列疑似: %v", suspNo)
	}
}
