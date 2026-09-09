package importer

import (
	"strings"
	"testing"
	"time"
)

// TestBorrowParseGDate G 列日期解析（YYYY.M.D；缺日/异常 → REVIEW）。
func TestBorrowParseGDate(t *testing.T) {
	cases := []struct {
		in     string
		wantY  int
		review bool
	}{
		{"2022.6.14", 2022, false},
		{"2015.4.12", 2015, false},
		{"2021.11.04", 2021, false},
		{"2018.4", 0, true},    // 缺日 → REVIEW（不猜补日）
		{"", 0, true},          // 空 → REVIEW
		{"2026.13.1", 0, true}, // 非法月
	}
	for _, c := range cases {
		got, rev := parseGDate(c.in)
		if rev != c.review {
			t.Errorf("parseGDate(%q) review=%v want %v", c.in, rev, c.review)
		}
		if c.review {
			continue
		}
		if got == nil || got.Year() != c.wantY {
			t.Errorf("parseGDate(%q)=%v want year %d", c.in, got, c.wantY)
		}
	}
}

// TestBorrowParseJCell 决策 18 §十四/§十五：J 列典型形态解析。
func TestBorrowParseJCell(t *testing.T) {
	cases := []struct {
		in      string
		wantNos int  // 期望解析出的编号台数（不含无编号/纯注释）
		unnum   bool // 是否含无编号
		anyRev  bool // 是否至少一台 needs_review（结构异常）
		wantRet int  // 期望解析出的 returnDate 数
	}{
		{"6061 6062", 2, false, false, 0},
		{"2183（带拖布轮）", 1, false, false, 0},
		{"492024（2026.9.4入南库）", 1, false, false, 1},
		{"292020(2026.9.4入南库）", 1, false, false, 1},
		{"（3313 2029 2001 1905(带拖布轮）", 4, false, true, 0},
		{"（2024 2027 4325（带拖布轮）", 3, false, true, 0},
		{"11612（11369（带拖布轮）", 2, false, true, 0},
		{"无编号", 0, true, false, 0},
		{"0475", 1, false, false, 0},
		{"拉布机配件", 0, false, true, 0}, // 描述文本 → REVIEW 保留原文
	}
	for _, c := range cases {
		nums := parseJCell(c.in)
		cnt := countBorrowUnits(nums)
		if cnt != c.wantNos {
			t.Errorf("parseJCell(%q) 编号台数=%d want %d (全部: %+v)", c.in, cnt, c.wantNos, nums)
		}
		hasUn, hasRev, rets := false, false, 0
		for _, n := range nums {
			if n.Unnumbered {
				hasUn = true
			}
			if n.NeedsReview {
				hasRev = true
			}
			if n.ReturnDate != nil {
				rets++
				want := time.Date(2026, 9, 4, 0, 0, 0, 0, time.Local)
				if !n.ReturnDate.Equal(want) {
					t.Errorf("parseJCell(%q) returnDate=%v want 2026-09-04", c.in, n.ReturnDate)
				}
			}
		}
		if hasUn != c.unnum {
			t.Errorf("parseJCell(%q) 无编号=%v want %v", c.in, hasUn, c.unnum)
		}
		if hasRev != c.anyRev {
			t.Errorf("parseJCell(%q) needsReview=%v want %v (nums=%+v)", c.in, hasRev, c.anyRev, nums)
		}
		if rets != c.wantRet {
			t.Errorf("parseJCell(%q) returnDates=%d want %d", c.in, rets, c.wantRet)
		}
	}
}

// TestParseRealFileBorrowEvents 真实文件 J 列事件不变量（Phase 0/决策15 口径：G 锚定 144）。
func TestParseRealFileBorrowEvents(t *testing.T) {
	res := parseRealFile(t)
	if len(res.BorrowEvents) != 144 {
		t.Fatalf("借出事件应为 144 条（G 锚定；决策15/数据口径），实际 %d", len(res.BorrowEvents))
	}
	internal := 0
	hasMultiRow := 0
	hasReview := 0
	for _, e := range res.BorrowEvents {
		if strings.Contains(e.Company, "双发") {
			internal++
		}
		if e.RowTo > e.RowFrom {
			hasMultiRow++
		}
		if e.NeedsReview {
			hasReview++
		}
		if e.JRaw == "" && e.CountRaw == "" {
			t.Fatalf("事件 R%d 无任何原文（J/I 全空）", e.RowFrom)
		}
	}
	if internal != 101 {
		t.Fatalf("内部单位（含“双发”）事件应为 101，实际 %d", internal)
	}
	if hasMultiRow < 10 {
		t.Logf("提示：跨行续 J 事件 %d 条（正常 ≥10）", hasMultiRow)
	}
	if hasReview == 0 {
		t.Log("提示：未命中任何 REVIEW 事件")
	}
	// 抽样：R334 应为 2 台（492013 与 492024），492024 带 2026.9.4入南库 归还线索
	for _, e := range res.BorrowEvents {
		if e.RowFrom == 334 {
			if len(e.Numbers) != 2 || e.Numbers[1].No != "492024" {
				t.Fatalf("R334 事件解析异常: %+v", e.Numbers)
			}
			if e.Numbers[1].ReturnDate == nil {
				t.Fatal("R334 第二台应解析出 2026-09-04 归还线索")
			}
			return
		}
	}
	t.Fatal("真实文件缺少 R334 事件")
}

// TestParseMiniFileBorrowEvents 迷你 xlsx：G 锚定事件 + 跨行续 J 合并 + 入南库线索。
func TestParseMiniFileBorrowEvents(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "4", "", "6041 6041 6041 6041", "", "", "", "", ""},
		{"", "", "", "", "", "", "2022.6.14", "莒县双发", "2", "6061 6062", ""},
		{"", "", "", "", "", "", "", "", "", "6063", ""}, // 续 J 行（无 G）
		{"", "", "", "", "", "", "", "", "", "6088", ""}, // 再续 J
		{"", "", "", "", "", "", "2015.4.12", "泰和", "1", "492024（2026.9.4入南库）", ""},
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(res.BorrowEvents) != 2 {
		t.Fatalf("应 2 个 G 锚定事件，实际 %d", len(res.BorrowEvents))
	}
	e1 := res.BorrowEvents[0]
	if e1.Company != "莒县双发" || e1.Count != 2 || e1.Date == nil || e1.Date.Year() != 2022 {
		t.Fatalf("事件1 头解析异常: %+v", e1)
	}
	if e1.RowTo != e1.RowFrom+2 {
		t.Fatalf("事件1 应合并 3 行（R锚+2续J），实际 RowTo=%d RowFrom=%d", e1.RowTo, e1.RowFrom)
	}
	if got := countBorrowUnits(e1.Numbers); got != 4 {
		t.Fatalf("事件1 应含 4 台（6061/6062/6063/6088），实际 %d，numbers=%+v", got, e1.Numbers)
	}
	// 事件2：入南库线索
	e2 := res.BorrowEvents[1]
	if e2.Date == nil || e2.Date.Year() != 2015 {
		t.Fatalf("事件2 日期异常: %+v", e2)
	}
	if len(e2.Numbers) != 1 || e2.Numbers[0].ReturnDate == nil {
		t.Fatalf("事件2 应带归还线索: %+v", e2.Numbers)
	}
}
