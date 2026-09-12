package importer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Phase 2（2026-09-11）：外借明细（到达/外借方/数量/设备编号）解析测试。
// 口径依据决策 19：② 台数以 C 列为准；③ 无编号跳过；④ 括号注释只保留编号。

// writeDetailXLSX 构造外借明细测试文件（headers 写 R1，cells 为“坐标=值”，merges 为合并区间）。
func writeDetailXLSX(t *testing.T, headers []string, cells map[string]string, merges [][2]string) string {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, h := range headers {
		axis, _ := excelize.CoordinatesToCellName(i+1, detailHeaderRow)
		if err := f.SetCellStr(sheet, axis, h); err != nil {
			t.Fatal(err)
		}
	}
	for axis, v := range cells {
		if err := f.SetCellStr(sheet, axis, v); err != nil {
			t.Fatalf("写入 %s 失败: %v", axis, v)
		}
	}
	for _, m := range merges {
		if err := f.MergeCell(sheet, m[0], m[1]); err != nil {
			t.Fatalf("合并 %v 失败: %v", m, err)
		}
	}
	path := filepath.Join(t.TempDir(), "detail.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// detailHeaders 标准明细表头（A–D）。
func detailHeaders() []string {
	return []string{"到达时间", "外借方", "数量", "设备编号"}
}

// issueCount 统计指定编码/级别的问题条数。
func issueCount(res *DetailParseResult, code, level string) int {
	n := 0
	for _, is := range res.Issues {
		if (code == "" || is.Code == code) && (level == "" || is.Level == level) {
			n++
		}
	}
	return n
}

// TestParseBorrowDetailStructure 复现事故文件结构：合并块 + 嵌套括号 + 无编号 + 合计行。
func TestParseBorrowDetailStructure(t *testing.T) {
	cells := map[string]string{
		// 块1 R2-R3：C=10，编号 6+4
		"A2": "2012.12.22", "B2": "双发分厂（龙山贸易）", "C2": "10",
		"D2": "11221 11307 12583 11310 21163 11545",
		"D3": "11275 11304 11702 21983",
		// 块2 R4-R7：C=28，编号 8+8+8+4
		"A4": "2013.1.4", "B4": "双发分厂（龙山贸易）", "C4": "28",
		"D4": "12730 22241 22079 11493 11183 21049 21990 12531",
		"D5": "11259 12726 12717 22287 21009 22243 22307 11371",
		"D6": "11391 11337 22038 22300 12537 21211 11329 11466",
		"D7": "21121 11231 11327 11496",
		// 块3 R8：C=5，含嵌套括号 11612（11369（带拖布轮）
		"A8": "2015.4.12", "B8": "莒县双发", "C8": "5",
		"D8": "11612（11369（带拖布轮） 22242 22299 11229",
		// 块4 R9：C=4，前导括号 + 尾部注释
		"A9": "2021.9.25", "B9": "莒县双发", "C9": "4",
		"D9": "（3313 2029 2001 1905(带拖布轮）",
		// 块5 R10：无编号 14 台（按口径③跳过）
		"A10": "2015.4.12", "B10": "莒县双发", "C10": "14",
		"D10": "无编号",
		// 块6 R11：单台 + 前导零编号
		"A11": "2018.1.22", "B11": "莒县双发", "C11": "1",
		"D11": "0430701",
		// 表末合计行（与 C 列实际不一致 → V-D9 提示）
		"C12": "999",
	}
	merges := [][2]string{{"A2", "A3"}, {"B2", "B3"}, {"C2", "C3"}, {"A4", "A7"}, {"B4", "B7"}, {"C4", "C7"}}
	path := writeDetailXLSX(t, detailHeaders(), cells, merges)

	res, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.TotalCount != 62 {
		t.Fatalf("数量(C)合计应为 62，实际 %d", res.TotalCount)
	}
	if res.NumberedCount != 48 {
		t.Fatalf("编号台数应为 48，实际 %d", res.NumberedCount)
	}
	if res.SkippedUnnum != 14 {
		t.Fatalf("跳过无编号台数应为 14，实际 %d", res.SkippedUnnum)
	}
	if len(res.Devices) != 48 {
		t.Fatalf("可定位设备应为 48，实际 %d", len(res.Devices))
	}
	if len(res.Blocks) != 6 {
		t.Fatalf("块数应为 6，实际 %d", len(res.Blocks))
	}

	// 块1：合并块，日期解析、外借方、来源键
	b1 := res.Blocks[0]
	if b1.Count != 10 || len(b1.Numbers) != 10 || !b1.OK {
		t.Fatalf("块1异常: count=%d numbers=%d ok=%v", b1.Count, len(b1.Numbers), b1.OK)
	}
	if b1.ArriveAt == nil || b1.ArriveAt.Format("2006-01-02") != "2012-12-22" {
		t.Fatalf("块1到达时间解析异常: %v (%q)", b1.ArriveAt, b1.ArriveRaw)
	}
	if b1.BlockRows != "R2-R3" {
		t.Fatalf("块1行区间应为 R2-R3，实际 %s", b1.BlockRows)
	}
	if b1.Devices[0].SourceKey != "R2-R3#N1" {
		t.Fatalf("来源键异常: %s", b1.Devices[0].SourceKey)
	}

	// 块3：嵌套括号 → 两台（11612、11369），注释进备注
	b3 := res.Blocks[2]
	if len(b3.Numbers) != 5 {
		t.Fatalf("块3应展开 5 台，实际 %v", b3.Numbers)
	}
	if !containsStr(b3.Numbers, "11612") || !containsStr(b3.Numbers, "11369") {
		t.Fatalf("嵌套括号内编号应被提取: %v", b3.Numbers)
	}
	if !strings.Contains(b3.Remark, "带拖布轮") {
		t.Fatalf("括号注释应保留在备注: %q", b3.Remark)
	}
	if !b3.OK {
		t.Fatalf("块3数量自洽时不应阻断: %+v", b3)
	}

	// 块4：前导括号编号
	b4 := res.Blocks[3]
	want4 := []string{"3313", "2029", "2001", "1905"}
	if len(b4.Numbers) != 4 {
		t.Fatalf("块4应展开 4 台，实际 %v", b4.Numbers)
	}
	for i, w := range want4 {
		if b4.Numbers[i] != w {
			t.Fatalf("块4编号异常: %v", b4.Numbers)
		}
	}

	// 块5：无编号 → 跳过，不产设备
	b5 := res.Blocks[4]
	if b5.Unnumbered != 14 || len(b5.Devices) != 0 || !b5.OK {
		t.Fatalf("无编号块应跳过且不产设备: un=%d devices=%d ok=%v", b5.Unnumbered, len(b5.Devices), b5.OK)
	}
	if issueCount(res, codeDetailUnnumber, "WARN") != 1 {
		t.Fatalf("应记录 1 条无编号跳过提示，实际 %d", issueCount(res, codeDetailUnnumber, "WARN"))
	}

	// 块6：前导零编号原样保留
	if res.Blocks[5].Numbers[0] != "0430701" {
		t.Fatalf("前导零编号应原样保留: %v", res.Blocks[5].Numbers)
	}

	// 表末合计行与 C 列不一致 → WARN（合计行本身不计入台数）
	if res.TotalRowRaw != "999" {
		t.Fatalf("应识别表末合计行 999，实际 %q", res.TotalRowRaw)
	}
	if issueCount(res, codeDetailTotalRow, "WARN") != 1 {
		t.Fatalf("应提示合计与 C 列不一致，实际 %d", issueCount(res, codeDetailTotalRow, "WARN"))
	}
	// 嵌套/前导括号结构异常 → WARN（不阻断）
	if issueCount(res, codeDetailParen, "WARN") == 0 {
		t.Fatal("应记录括号结构异常提示（WARN）")
	}
	// 本 fixture 不应出现 BLOCK
	if n := issueCount(res, "", "BLOCK"); n != 0 {
		t.Fatalf("不应出现 BLOCK，实际 %d 条: %+v", n, res.Issues)
	}
	// 外借方按首见顺序
	if len(res.Companies) != 2 || res.Companies[0] != "双发分厂（龙山贸易）" || res.Companies[1] != "莒县双发" {
		t.Fatalf("外借方聚合异常: %v", res.Companies)
	}
}

// TestParseBorrowDetailCountMismatchBlocks 数量 C 与编号展开不一致 → 整块 BLOCK。
func TestParseBorrowDetailCountMismatchBlocks(t *testing.T) {
	path := writeDetailXLSX(t, detailHeaders(), map[string]string{
		"A2": "2020.5.1", "B2": "泰和", "C2": "5", "D2": "1001 1002 1003",
	}, nil)
	res, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Blocks[0].OK {
		t.Fatal("数量不一致的块应标记不可导入")
	}
	if issueCount(res, codeDetailCount, "BLOCK") != 1 {
		t.Fatalf("应记录 1 条 V-D1 BLOCK，实际 %d", issueCount(res, codeDetailCount, "BLOCK"))
	}
	if res.TotalCount != 5 || res.NumberedCount != 3 {
		t.Fatalf("汇总应以 C 列为准: total=%d numbered=%d", res.TotalCount, res.NumberedCount)
	}
}

// TestParseBorrowDetailMissingDateAndCompany 缺到达时间/外借方 → BLOCK。
func TestParseBorrowDetailMissingDateAndCompany(t *testing.T) {
	path := writeDetailXLSX(t, detailHeaders(), map[string]string{
		"C2": "1", "D2": "6040",
	}, nil)
	res, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Blocks[0].OK {
		t.Fatal("缺日期与外借方的块应不可导入")
	}
	if issueCount(res, codeDetailDate, "BLOCK") != 1 || issueCount(res, codeDetailCompany, "BLOCK") != 1 {
		t.Fatalf("应各记录 1 条 V-D2/V-D3 BLOCK: %+v", res.Issues)
	}
}

// TestParseBorrowDetailRejectsLedgerLayout 误传台账版式 → 明确拒绝并引导到台账通道。
func TestParseBorrowDetailRejectsLedgerLayout(t *testing.T) {
	headers := []string{"类别", "设备名称", "设备型号", "台账数量", "财务数量", "台账设备编号", "时间", "公司", "台数", "借出设备编号", "备注"}
	path := writeDetailXLSX(t, headers, map[string]string{
		"A5": "裁剪设备", "B5": "环形割刀", "C5": "EBK-SA", "D5": "9", "F5": "6040 6061",
	}, nil)
	_, err := ParseBorrowDetail(path)
	if !errors.Is(err, ErrDetailTemplateMismatch) {
		t.Fatalf("应返回 ErrDetailTemplateMismatch，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "台账") {
		t.Fatalf("错误应提示改用台账通道: %s", err.Error())
	}
}

// TestParseBorrowDetailRejectsWrongHeader 列名不符 → 拒绝（不改行硬解析）。
func TestParseBorrowDetailRejectsWrongHeader(t *testing.T) {
	path := writeDetailXLSX(t, []string{"到达时间", "外借方", "台数", "设备编号"}, map[string]string{
		"A2": "2020.5.1", "B2": "泰和", "C2": "1", "D2": "6040",
	}, nil)
	_, err := ParseBorrowDetail(path)
	if !errors.Is(err, ErrDetailTemplateMismatch) {
		t.Fatalf("应返回 ErrDetailTemplateMismatch，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "C列") {
		t.Fatalf("错误应指出 C 列不一致: %s", err.Error())
	}
}

// TestParseBorrowDetailOptionalNameModel 可选 E/F 列（消歧方案 A）应被解析到块级。
func TestParseBorrowDetailOptionalNameModel(t *testing.T) {
	headers := append(detailHeaders(), "设备名称", "设备型号")
	path := writeDetailXLSX(t, headers, map[string]string{
		"A2": "2020.5.1", "B2": "泰和", "C2": "1", "D2": "11231",
		"E2": "预分配1", "F2": "预分配1",
	}, nil)
	res, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Blocks[0].Name != "预分配1" || res.Blocks[0].Model != "预分配1" {
		t.Fatalf("可选列未解析: name=%q model=%q", res.Blocks[0].Name, res.Blocks[0].Model)
	}
}

// TestParseBorrowDetailRealWorkbook1 真实事故文件回归（文件存在时执行；缺失则跳过）。
// 期望：C 列合计 230 = 214 台有编号 + 16 台无编号（口径②③）。
func TestParseBorrowDetailRealWorkbook1(t *testing.T) {
	path := filepath.Join(repoRoot, "工作簿1.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("工作簿1.xlsx 不在仓库根目录（用户数据文件），跳过真实文件回归")
	}
	res, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.TotalCount != 230 {
		t.Errorf("数量(C)合计应为 230，实际 %d", res.TotalCount)
	}
	if res.NumberedCount != 214 {
		t.Errorf("编号台数应为 214，实际 %d", res.NumberedCount)
	}
	if res.SkippedUnnum != 16 {
		t.Errorf("跳过无编号应为 16，实际 %d", res.SkippedUnnum)
	}
	if len(res.Devices) != 214 {
		t.Errorf("可定位设备应为 214，实际 %d", len(res.Devices))
	}
	if n := issueCount(res, "", "BLOCK"); n != 0 {
		t.Errorf("真实文件不应有 BLOCK，实际 %d 条: %+v", n, res.Issues)
	}
	if res.TotalRowRaw != "208" {
		t.Errorf("应识别表末合计行 208，实际 %q", res.TotalRowRaw)
	}
}
