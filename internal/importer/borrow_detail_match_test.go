package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"equipment/internal/models"
)

// Phase 3（2026-09-11）：外借明细匹配测试（只读匹配：编号为主 + 名称/型号定位 + 预分配标签）。

// TestMatchBorrowDetailBuckets 分桶：唯一命中 / 同号多台（无文件定位）/ 顺序建议 / 标签定位 / 未匹配。
func TestMatchBorrowDetailBuckets(t *testing.T) {
	db := openMigratedDB(t, "detail-match")
	now := models.Now()
	code := 0
	mk := func(no, name, model string, seq int) {
		code++
		n := no
		eq := models.Equipment{
			EquipmentNo: &n, EquipmentSeq: seq,
			InternalCode: "EQ-" + no + "-" + string(rune('a'+code)),
			Name:         name, Model: model, Status: models.StatusInStock, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
	}
	mk("1001", "平车", "DDL-9000B", 1)     // 唯一
	mk("2002", "双针平车（重机）", "LH-3568", 1) // 同号多台：文件用 预分配2 标签定位第二台
	mk("2002", "平车", "DDL-9000B", 1)
	mk("2003", "平车", "DDL-9000S", 1) // 同号多台：文件只出现 1 次 → 无法定位（无建议）
	mk("2003", "平车", "DDL-9000S", 2)
	mk("3003", "平车", "DDL-9000S", 1) // 同号多台 + 文件内出现 2 次 → 顺序建议
	mk("3003", "平车", "DDL-9000S", 2)
	mk("9998", "钉扣机", "LK-1903", 1) // 供 MISSING 旁证（与 9999 差 1 位）
	if err := db.Create(&models.Borrower{Name: "泰和", IsActive: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	cells := map[string]string{
		"A2": "2020.5.1", "B2": "泰和", "C2": "1", "D2": "1001",
		// 文件用 预分配2 标签定位 2002 的第二台
		"A3": "2020.5.2", "B3": "新公司X", "C3": "1", "D3": "2002", "E3": "预分配2",
		"A4": "2020.5.3", "B4": "泰和", "C4": "1", "D4": "2003",
		"A5": "2020.5.4", "B5": "泰和", "C5": "1", "D5": "3003",
		"A6": "2020.5.5", "B6": "泰和", "C6": "1", "D6": "3003",
		"A7": "2020.5.6", "B7": "泰和", "C7": "1", "D7": "9999",
	}
	headers := append(detailHeaders(), "设备名称", "设备型号")
	res, err := ParseBorrowDetail(writeDetailXLSX(t, headers, cells, nil))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	m, err := MatchBorrowDetail(db, res)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}
	if m.Total != 6 {
		t.Fatalf("明细设备应为 6，实际 %d", m.Total)
	}
	if m.Unique != 1 || m.Ambiguous != 3 || m.ByLabel != 1 || m.Missing != 1 {
		t.Fatalf("分桶异常: unique=%d ambiguous=%d byLabel=%d missing=%d",
			m.Unique, m.Ambiguous, m.ByLabel, m.Missing)
	}
	if len(m.SameNoMultiNo) != 3 {
		t.Fatalf("库中同号多台编号应为 3（2002/2003/3003），实际 %v", m.SameNoMultiNo)
	}
	if len(m.DupNos) != 1 || m.DupNos[0] != "3003" {
		t.Fatalf("文件内重复编号应为 [3003]，实际 %v", m.DupNos)
	}
	if m.MinBorrowDate != "2020-05-01" || m.MaxBorrowDate != "2020-05-06" {
		t.Fatalf("外借日期区间异常: %s ~ %s", m.MinBorrowDate, m.MaxBorrowDate)
	}

	byNo := map[string][]*DetailMatchItem{}
	for _, it := range m.Items {
		byNo[it.EquipmentNo] = append(byNo[it.EquipmentNo], it)
	}
	// 唯一命中
	if it := byNo["1001"][0]; it.Status != MatchUnique || it.Chosen == nil || it.Chosen.Name != "平车" {
		t.Fatalf("1001 应唯一命中: %+v", it)
	}
	// 3003：文件出现 2 次、库中 2 台 → 顺序建议（预分配1/预分配2），但仍需人工确认
	for i, it := range byNo["3003"] {
		if it.Status != MatchAmbiguous {
			t.Fatalf("3003 应为 AMBIGUOUS（需确认）: %+v", it)
		}
		if len(it.Candidates) != 2 || it.Candidates[0].Label != "预分配1" || it.Candidates[1].Label != "预分配2" {
			t.Fatalf("3003 候选标签异常: %+v", it.Candidates)
		}
		if it.SuggestedID != it.Candidates[i].EquipmentID {
			t.Fatalf("3003 第 %d 次出现应建议 %s，实际 %d", i+1, it.Candidates[i].Label, it.SuggestedID)
		}
		if !strings.Contains(it.Note, "按出现顺序") {
			t.Fatalf("3003 应给出顺序建议说明: %q", it.Note)
		}
	}
	// 2003：库中 2 台、文件只出现 1 次 → 无法定位，无顺序建议
	it2003 := byNo["2003"][0]
	if it2003.Status != MatchAmbiguous || it2003.SuggestedID != 0 {
		t.Fatalf("2003（文件仅 1 次 vs 库 2 台）不应有顺序建议: %+v", it2003)
	}
	if !strings.Contains(it2003.Note, "请人工选择") {
		t.Fatalf("2003 应提示人工选择: %q", it2003.Note)
	}
	// 2002 带 预分配2 → BY_LABEL 命中第二台（平车 DDL-9000B）
	it2002 := byNo["2002"][0]
	if it2002.Status != MatchByLabel || it2002.Chosen == nil || it2002.Chosen.Model != "DDL-9000B" {
		t.Fatalf("2002 带 预分配2 应按标签命中第二台: %+v", it2002)
	}
	if it2002.Chosen.Label != "预分配2" {
		t.Fatalf("命中候选应带 预分配2 标签: %+v", it2002.Chosen)
	}
	// 9999 未匹配 + 旁证
	it9999 := byNo["9999"][0]
	if it9999.Status != MatchMissing {
		t.Fatalf("9999 应为 MISSING: %+v", it9999)
	}
	if !strings.Contains(it9999.Evidence, "9998") {
		t.Fatalf("9999 应给出旁证（9998）: %q", it9999.Evidence)
	}
	// 外借方计划：泰和已存在、新公司X 需新建
	plan := map[string]*BorrowerPlan{}
	for _, b := range m.Borrowers {
		plan[b.Name] = b
	}
	if p := plan["泰和"]; p == nil || !p.Exists || p.Devices != 5 {
		t.Fatalf("泰和计划异常: %+v", p)
	}
	if p := plan["新公司X"]; p == nil || p.Exists || p.Devices != 1 {
		t.Fatalf("新公司X 计划异常: %+v", p)
	}
}

// TestMatchBorrowDetailRealFile 真实文件对照真实总账库（只读）：
// 198 唯一命中 + 15 同号多台 + 1 未匹配（0430701）；外借方为 4 个双发系单位且均未建档。
func TestMatchBorrowDetailRealFile(t *testing.T) {
	path := filepath.Join(repoRoot, "工作簿1.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("工作簿1.xlsx 不在仓库根目录（用户数据文件），跳过真实文件匹配")
	}
	db := openMigratedDB(t, "real-match")
	ledger := parseRealFile(t)
	ledger.Filename = "设备借出总账.xlsx"
	if _, err := Import(db, ledger); err != nil {
		t.Fatalf("准备真实总账库失败: %v", err)
	}
	res, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	m, err := MatchBorrowDetail(db, res)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}
	if m.Total != 214 {
		t.Fatalf("明细设备应为 214，实际 %d", m.Total)
	}
	if m.Unique != 198 || m.Ambiguous != 15 || m.Missing != 1 || m.ByLabel != 0 {
		t.Fatalf("分桶异常: unique=%d ambiguous=%d missing=%d byLabel=%d",
			m.Unique, m.Ambiguous, m.Missing, m.ByLabel)
	}
	if len(m.SameNoMultiNo) != 12 {
		t.Fatalf("库中同号多台编号应为 12，实际 %d: %v", len(m.SameNoMultiNo), m.SameNoMultiNo)
	}
	if len(m.DupNos) != 3 {
		t.Fatalf("文件内重复编号应为 3（12538/2015/2029），实际 %v", m.DupNos)
	}
	var missingNo string
	for _, it := range m.Items {
		if it.Status == MatchMissing {
			missingNo = it.EquipmentNo
		}
	}
	if missingNo != "0430701" {
		t.Fatalf("未匹配编号应为 0430701，实际 %q", missingNo)
	}
	if len(m.Borrowers) != 4 {
		t.Fatalf("外借方应为 4 个双发系单位，实际 %+v", m.Borrowers)
	}
	for _, b := range m.Borrowers {
		if b.Exists {
			t.Fatalf("双发系单位不应已存在于外借方字典: %+v", b)
		}
	}
}
