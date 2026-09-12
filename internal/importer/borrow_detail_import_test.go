package importer

import (
	"errors"
	"strings"
	"testing"

	"equipment/internal/models"
)

// Phase 4（2026-09-11）：外借明细补录写入测试（单事务、外借N 标签、同号多台按序分配）。

// TestImportBorrowDetailHappyPath 正常补录：唯一命中 + 同号多台按序 + 未匹配按标签新建。
func TestImportBorrowDetailHappyPath(t *testing.T) {
	db := openMigratedDB(t, "detail-import")
	now := models.Now()
	code := 0
	mk := func(no, eqName, model, status string) uint {
		code++
		n := no
		eq := models.Equipment{
			EquipmentNo: &n, EquipmentSeq: 1,
			InternalCode: "EQ-00" + string(rune('a'+code)),
			Name:         eqName, Model: model, Status: status, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
		// 模拟真实台账：每台设备都带 IMPORT_INIT 初始流转
		if err := db.Create(&models.Transaction{
			EquipmentID: eq.ID, Action: models.ActionImportInit, FromStatus: "-", ToStatus: models.StatusInStock,
			OccurredAt: now, Operator: operatorImport, CreatedAt: now,
		}).Error; err != nil {
			t.Fatal(err)
		}
		return eq.ID
	}
	idUnique := mk("1001", "平车", "DDL-9000B", models.StatusInStock)
	mk("2002", "双针平车（重机）", "LH-3568", models.StatusInStock) // 候选第 1 台
	mk("2002", "平车", "DDL-9000B", models.StatusInStock)     // 候选第 2 台
	if err := db.Create(&models.Borrower{Name: "泰和", IsActive: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	path := writeDetailXLSX(t, detailHeaders(), map[string]string{
		"A2": "2020.5.1", "B2": "泰和", "C2": "1", "D2": "1001",
		"A3": "2020.5.2", "B3": "泰和", "C3": "1", "D3": "2002",
		"A4": "2020.5.3", "B4": "新公司Y", "C4": "1", "D4": "9999",
	}, nil)
	parse, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	parse.Filename = "工作簿1.xlsx"
	match, err := MatchBorrowDetail(db, parse)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}

	// 外借标签按外借方顺序：泰和 外借1、外借2；新公司Y 外借1（各自从 1 起）
	if match.Items[0].Label != "外借1" || match.Items[1].Label != "外借2" {
		t.Fatalf("泰和外借标签应为 外借1/外借2，实际 %q/%q", match.Items[0].Label, match.Items[1].Label)
	}
	if match.Items[2].Label != "外借1" {
		t.Fatalf("新公司Y 应从 外借1 起，实际 %q", match.Items[2].Label)
	}
	if len(match.Borrowers) != 2 || match.Borrowers[0].Labeled != 2 || match.Borrowers[0].LabelFrom != "外借1" || match.Borrowers[0].LabelTo != "外借2" {
		t.Fatalf("外借方计划异常: %+v", match.Borrowers)
	}

	rep, err := ImportBorrowDetail(db, parse, match, BorrowDetailOptions{})
	if err != nil {
		t.Fatalf("补录失败: %v", err)
	}
	if rep.Borrowed != 3 || rep.Created != 1 || rep.Labeled != 3 || rep.Skipped != 0 || rep.Blocked != 0 || rep.HistoryOnly != 0 {
		t.Fatalf("报告异常: %+v", rep)
	}
	if rep.Items[0].Action != "BORROW_FULL" || rep.Items[2].Action != "CREATED_BORROW" {
		t.Fatalf("逐台动作异常: %s / %s", rep.Items[0].Action, rep.Items[2].Action)
	}

	// 唯一命中：状态 BORROWED、外借日期取到达时间、名称型号未被改动
	var eq models.Equipment
	if err := db.First(&eq, idUnique).Error; err != nil {
		t.Fatal(err)
	}
	if eq.Status != models.StatusBorrowed || eq.CurrentBorrowerID == nil || eq.CurrentBorrowRecordID == nil {
		t.Fatalf("设备未置外借: %+v", eq)
	}
	if !eq.CurrentSince.Valid || eq.CurrentSince.Time.Format("2006-01-02") != "2020-05-01" {
		t.Fatalf("current_since 应为 2020-05-01，实际 %v", eq.CurrentSince)
	}
	if eq.Name != "平车" || eq.Model != "DDL-9000B" {
		t.Fatalf("台账名称/型号不应被改动: %s/%s", eq.Name, eq.Model)
	}
	// 备注应含 外借N 标签
	var rec models.BorrowRecord
	if err := db.First(&rec, *eq.CurrentBorrowRecordID).Error; err != nil {
		t.Fatal(err)
	}
	if rec.Status != models.BorrowOutstanding || rec.BorrowDate.Time.Format("2006-01-02") != "2020-05-01" {
		t.Fatalf("外借单异常: %+v", rec)
	}
	if !strings.Contains(rec.Remark, "外借1") {
		t.Fatalf("外借单备注应含外借标签: %q", rec.Remark)
	}
	// 流转历史：IMPORT_INIT + BORROW
	var flows []models.Transaction
	db.Where("equipment_id = ?", idUnique).Order("id ASC").Find(&flows)
	if len(flows) != 2 || flows[0].Action != models.ActionImportInit || flows[1].Action != models.ActionBorrow {
		t.Fatalf("流转历史异常: %+v", flows)
	}
	if flows[1].OccurredAt.Format("2006-01-02") != "2020-05-01" || flows[1].BorrowerName != "泰和" {
		t.Fatalf("流转记录内容异常: %+v", flows[1])
	}

	// 未匹配：按外借标签新建（名称＝型号＝外借1）
	var created models.Equipment
	if err := db.Where("equipment_no = ?", "9999").First(&created).Error; err != nil {
		t.Fatalf("未新建设备 9999: %v", err)
	}
	if created.Name != "外借1" || created.Model != "外借1" || created.Status != models.StatusBorrowed {
		t.Fatalf("新建设备异常: %+v", created)
	}
	if created.EquipmentSeq == 0 {
		t.Fatal("新建后 equipment_seq 应已重算")
	}

	// 外借方：泰和复用、新公司Y 新建
	var borrowers []models.Borrower
	db.Find(&borrowers)
	if len(borrowers) != 2 {
		t.Fatalf("外借方应为 2 个（泰和 + 新公司Y），实际 %d", len(borrowers))
	}
	var recCount, auditCount int64
	db.Model(&models.BorrowRecord{}).Count(&recCount)
	db.Model(&models.AuditLog{}).Where("action = ?", "IMPORT_BORROW_DETAIL").Count(&auditCount)
	if recCount != 3 || auditCount != 1 {
		t.Fatalf("外借单应为 3、audit 应为 1，实际 %d/%d", recCount, auditCount)
	}

	// 幂等：同文件再补录 → ErrBatchImported
	if _, err := ImportBorrowDetail(db, parse, match, BorrowDetailOptions{}); !errors.Is(err, ErrBatchImported) {
		t.Fatalf("同文件二次补录应 ErrBatchImported，实际 %v", err)
	}
}

// TestImportBorrowDetailStagedByCompany 分批补录：先补一部分，再补其余；
// 同文件同范围重复提交被拒，已补录的台不会重复写入（用户 2026-09-11 按外借方选择导入）。
func TestImportBorrowDetailStagedByCompany(t *testing.T) {
	db := openMigratedDB(t, "detail-import-staged")
	now := models.Now()
	for i, no := range []string{"1001", "1002"} {
		n := no
		eq := models.Equipment{
			EquipmentNo: &n, EquipmentSeq: 1, InternalCode: "EQ-00030" + string(rune('0'+i)),
			Name: "平车", Model: "DDL-9000B", Status: models.StatusInStock, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
	}
	path := writeDetailXLSX(t, detailHeaders(), map[string]string{
		"A2": "2020.5.1", "B2": "甲公司", "C2": "1", "D2": "1001",
		"A3": "2020.5.2", "B3": "乙公司", "C3": "1", "D3": "1002",
	}, nil)
	parse, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	match, err := MatchBorrowDetail(db, parse)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}
	// 只补「甲公司」：跳过乙公司那台
	skipYi := map[string]bool{match.Items[1].SourceKey: true}
	rep1, err := ImportBorrowDetail(db, parse, match, BorrowDetailOptions{Skip: skipYi})
	if err != nil {
		t.Fatalf("第一批补录失败: %v", err)
	}
	if rep1.Borrowed != 1 || rep1.Skipped != 1 {
		t.Fatalf("第一批报告异常: borrowed=%d skipped=%d", rep1.Borrowed, rep1.Skipped)
	}
	// 同范围重复提交 → 幂等拒绝
	if _, err := ImportBorrowDetail(db, parse, match, BorrowDetailOptions{Skip: skipYi}); !errors.Is(err, ErrBatchImported) {
		t.Fatalf("同范围重复补录应 ErrBatchImported，实际 %v", err)
	}
	// 换范围（补乙公司、跳过甲公司）→ 允许；甲公司那台虽是已补录设备，也不会重复写
	skipJia := map[string]bool{match.Items[0].SourceKey: true}
	rep2, err := ImportBorrowDetail(db, parse, match, BorrowDetailOptions{Skip: skipJia})
	if err != nil {
		t.Fatalf("第二批补录失败: %v", err)
	}
	if rep2.Borrowed != 1 || rep2.Skipped != 1 {
		t.Fatalf("第二批报告异常: borrowed=%d skipped=%d", rep2.Borrowed, rep2.Skipped)
	}
	if rep2.ScopeHash == rep1.ScopeHash {
		t.Fatal("不同范围的批次键应不同")
	}
	// 全量再补一次（甲乙都选）：两台的来源键都已写过 → 全部 ALREADY_WRITTEN，不重复建单
	rep3, err := ImportBorrowDetail(db, parse, match, BorrowDetailOptions{})
	if err != nil {
		t.Fatalf("全量补录失败: %v", err)
	}
	if rep3.Borrowed != 0 || rep3.AlreadyWritten != 2 {
		t.Fatalf("重复补录应全部识别为已写入: borrowed=%d already=%d", rep3.Borrowed, rep3.AlreadyWritten)
	}
	var recs int64
	db.Model(&models.BorrowRecord{}).Count(&recs)
	if recs != 2 {
		t.Fatalf("外借单应仍为 2 张（不重复），实际 %d", recs)
	}
}

// TestImportBorrowDetailHistoryOnlyAndSkip 已外借设备只补历史；用户跳过的台不写入。
func TestImportBorrowDetailHistoryOnlyAndSkip(t *testing.T) {
	db := openMigratedDB(t, "detail-import-2")
	now := models.Now()
	n1, n2 := "1001", "1002"
	for i, no := range []string{n1, n2} {
		status := models.StatusInStock
		if i == 0 {
			status = models.StatusBorrowed // 已外借（如 Phase 9 勾选确认的在借设备）
		}
		eq := models.Equipment{
			EquipmentNo: &no, InternalCode: "EQ-00010" + string(rune('0'+i)), EquipmentSeq: 1,
			Name: "平车", Model: "DDL-9000B", Status: status, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
	}
	path := writeDetailXLSX(t, detailHeaders(), map[string]string{
		"A2": "2020.5.1", "B2": "泰和", "C2": "1", "D2": "1001",
		"A3": "2020.5.2", "B3": "泰和", "C3": "1", "D3": "1002",
	}, nil)
	parse, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	match, err := MatchBorrowDetail(db, parse)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}
	skipKey := match.Items[1].SourceKey
	rep, err := ImportBorrowDetail(db, parse, match, BorrowDetailOptions{Skip: map[string]bool{skipKey: true}})
	if err != nil {
		t.Fatalf("补录失败: %v", err)
	}
	if rep.HistoryOnly != 1 || rep.Skipped != 1 || rep.Borrowed != 0 {
		t.Fatalf("报告异常: borrowed=%d history=%d skipped=%d", rep.Borrowed, rep.HistoryOnly, rep.Skipped)
	}
	// 已外借设备：状态不变、不再新建外借单，仅多一条 BORROW 历史
	var eq1 models.Equipment
	if err := db.Where("equipment_no = ?", "1001").First(&eq1).Error; err != nil {
		t.Fatal(err)
	}
	if eq1.Status != models.StatusBorrowed {
		t.Fatalf("已外借设备状态不应变化: %s", eq1.Status)
	}
	var recCount int64
	db.Model(&models.BorrowRecord{}).Count(&recCount)
	if recCount != 0 {
		t.Fatalf("不应为已外借设备新建外借单，实际 %d", recCount)
	}
	var f models.Transaction
	if err := db.Where("equipment_id = ?", eq1.ID).First(&f).Error; err != nil {
		t.Fatalf("应补记流转历史: %v", err)
	}
	if f.Action != models.ActionBorrow || !strings.Contains(f.Remark, "仅补记历史") {
		t.Fatalf("历史记录异常: %+v", f)
	}
	// 跳过的台：不建外借单、不改状态
	var eq2 models.Equipment
	if err := db.Where("equipment_no = ?", "1002").First(&eq2).Error; err != nil {
		t.Fatal(err)
	}
	if eq2.Status != models.StatusInStock {
		t.Fatalf("跳过的设备状态不应变化: %s", eq2.Status)
	}
}
