package importer

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"equipment/internal/database"
	"equipment/internal/models"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// repoRoot 测试数据文件位于仓库根目录。
const repoRoot = "../.."

func parseRealFile(t *testing.T) *ParseResult {
	t.Helper()
	path := filepath.Join(repoRoot, "设备借出总账.xlsx")
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	return res
}

// TestParseRealFileInvariants 真实文件解析不变量（对照 Phase 0 统计）。
// v1.1（决策 18）：重复编号不再 BLOCK（WARN + 展开）；描述文本按无编号+备注。
func TestParseRealFileInvariants(t *testing.T) {
	res := parseRealFile(t)
	if res.Sheet == "" {
		t.Fatal("缺少 sheet")
	}
	if res.TotalD != 2147 {
		t.Fatalf("台账数量(D)合计应为 2147，实际 %d", res.TotalD)
	}
	if len(res.Groups) < 150 {
		t.Fatalf("分组数异常偏少: %d", len(res.Groups))
	}

	// 环形割刀 EBK-SA：9 台有编号、无重复、应可导入
	var found *Group
	for _, g := range res.Groups {
		if g.Name == "环形割刀" && g.Model == "EBK-SA" {
			found = g
			break
		}
	}
	if found == nil {
		t.Fatal("未找到 环形割刀/EBK-SA 分组")
	}
	if found.DSum != 9 || len(found.Numbers) != 9 || !found.OK {
		t.Fatalf("环形割刀分组异常: DSum=%d numbers=%d ok=%v", found.DSum, len(found.Numbers), found.OK)
	}
	if found.Numbers[0] != "6040" || found.Numbers[8] != "6064" {
		t.Fatalf("环形割刀编号异常: %v", found.Numbers)
	}

	hasV10Warn := false
	hasV3Warn := false
	hasBlock := false
	hasReview := false
	for _, is := range res.Issues {
		switch {
		case is.Level == "BLOCK":
			hasBlock = true
		case is.Level == IssueLevelReview:
			hasReview = true
		case is.Code == "V10":
			hasV10Warn = true
		case is.Code == "V3":
			hasV3Warn = true
		}
	}
	// 描述文本充当编号（R16 拉布机配件）→ V10 WARN（不 BLOCK）
	if !hasV10Warn {
		t.Error("缺少 V10 WARN（描述文本充当编号）问题记录")
	}
	// 真实文件存在同组重复编号（平车组个别号多录）→ 决策 18 后为 WARN（展开），不再 BLOCK
	if !hasV3Warn {
		t.Error("缺少 V3 WARN（同组重复编号）问题记录")
	}
	// 决策 18：真实文件不应因重复编号 BLOCK；也不应出现 REVIEW（全部可判定）
	if hasBlock {
		t.Error("决策 18 下真实文件不应再出现 BLOCK 级阻断（重复=多台真机）")
	}
	if hasReview {
		t.Error("真实文件不应出现 REVIEW（F 列内容应全部可判定）")
	}

	// 无编号台数合计 567 = 566（F 列"无编号"按 D 展开） + 1（R16 描述文本"拉布机配件"）
	un := 0
	for _, g := range res.Groups {
		un += g.Unnumbered
	}
	if un != 567 {
		t.Fatalf("无编号台数合计应为 567，实际 %d", un)
	}
}

func openMigratedDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := database.Open("mem://imp-" + name)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(sqlDB); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

// TestImportRealFile 真实文件导入：单事务写入、初始流转记录、编号/无编号台账、批次与 seq。
func TestImportRealFile(t *testing.T) {
	res := parseRealFile(t)
	res.Filename = "设备借出总账.xlsx"
	db := openMigratedDB(t, "real")

	report, err := Import(db, res)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	t.Logf("导入报告: 台账源=%d 分组=%d 导入=%d 跳过=%d Block分组=%d 重复=%d 无编号=%d 类别=%v 内部码=%s",
		report.TotalSourceD, report.Groups, report.Imported, report.Skipped, report.BlockGroups,
		report.Duplicates, report.Unnumbered, report.Categories, report.InternalCode)
	if report.Imported <= 0 {
		t.Fatalf("导入台数异常: %d", report.Imported)
	}
	// 决策 18：真实文件重复编号不再 BLOCK → 全量可导入 2147 台
	if report.BlockGroups != 0 || report.Skipped != 0 {
		t.Fatalf("决策 18 下真实文件不应有 BLOCK/跳过: block=%d skip=%d", report.BlockGroups, report.Skipped)
	}
	if report.Unnumbered != 567 {
		t.Fatalf("导入无编号台数应为 567，实际 %d", report.Unnumbered)
	}

	var eqCount, txnCount int64
	db.Model(&models.Equipment{}).Count(&eqCount)
	db.Model(&models.Transaction{}).Count(&txnCount)
	if eqCount != int64(report.Imported) {
		t.Fatalf("数据库设备数 %d != 报告导入数 %d", eqCount, report.Imported)
	}
	if txnCount != eqCount {
		t.Fatalf("初始流转记录数 %d != 设备数 %d（决策 7 应逐台生成 IMPORT_INIT）", txnCount, eqCount)
	}

	// 锚点：6061 在库 & 属于裁剪设备
	var eq models.Equipment
	if err := db.Where("equipment_no = ?", "6061").First(&eq).Error; err != nil {
		t.Fatalf("未导入设备 6061: %v", err)
	}
	if eq.Status != "IN_STOCK" {
		t.Fatalf("6061 初始状态应为 IN_STOCK，实际 %s", eq.Status)
	}
	if !eq.CurrentSince.Valid {
		t.Fatal("6061 current_since 不应为空（交付时间口径）")
	}
	if eq.SourceKey == "" || eq.ImportBatchID == nil || *eq.ImportBatchID != report.BatchID {
		t.Fatalf("设备 6061 应带来源批次与 source_key: key=%q batch=%v", eq.SourceKey, eq.ImportBatchID)
	}
	var cat models.Category
	if err := db.First(&cat, eq.CategoryID).Error; err != nil {
		t.Fatalf("类别缺失: %v", err)
	}
	if cat.Name != "裁剪设备" {
		t.Fatalf("6061 类别应为 裁剪设备，实际 %s", cat.Name)
	}

	// 无编号批次：缝制熨斗 T-3NS 102 台
	var count int64
	db.Model(&models.Equipment{}).Where("name = ? AND model = ? AND equipment_no IS NULL", "缝制熨斗", "T-3NS").Count(&count)
	if count != 102 {
		t.Fatalf("缝制熨斗 T-3NS 无编号应为 102 台，实际 %d", count)
	}
	// 内部码生成
	var first models.Equipment
	db.Order("id ASC").First(&first)
	if first.InternalCode != "EQ-000001" {
		t.Fatalf("内部码应从 EQ-000001 起，实际 %s", first.InternalCode)
	}
	// audit 留痕
	var audit int64
	db.Model(&models.AuditLog{}).Where("action = ?", "IMPORT").Count(&audit)
	if audit != 1 {
		t.Fatalf("应写入 1 条 IMPORT audit，实际 %d", audit)
	}
	// 批次记录（决策 18 §二十五）
	if report.BatchID == 0 || report.SourceHash == "" {
		t.Fatalf("导入报告应含 batch_id 与 source_hash: %+v", report)
	}
	var batch models.ImportBatch
	if err := db.First(&batch, report.BatchID).Error; err != nil {
		t.Fatalf("import_batch 缺失: %v", err)
	}
	if batch.SourceHash != res.SourceHash || batch.ImportedCount != report.Imported {
		t.Fatalf("import_batch 内容异常: %+v", batch)
	}
	// 全部设备应有非 0 equipment_seq（事务内 RenumberAllSeq 已执行）
	var zeroSeq int64
	db.Model(&models.Equipment{}).Where("equipment_seq = 0").Count(&zeroSeq)
	if zeroSeq != 0 {
		t.Fatalf("导入后不应存在 equipment_seq=0 的设备: %d", zeroSeq)
	}
	// 同号多台真机（决策 18）：平车 DDL-9000B 某重复号应 seq 1..n 且内部码不同
	var dup models.Equipment
	if err := db.Where("equipment_no = ? AND name = ? AND model = ?", "11472", "平车", "DDL-9000B").First(&dup).Error; err != nil {
		t.Logf("提示：11472 未找到（不影响断言）")
	} else {
		var ids []uint
		db.Model(&models.Equipment{}).Where("equipment_no = ? AND name = ? AND model = ?", "11472", "平车", "DDL-9000B").
			Order("equipment_seq ASC").Pluck("id", &ids)
		if len(ids) < 2 {
			t.Fatalf("11472 应有多台（同号真机），实际 %d 台", len(ids))
		}
	}
	// 幂等（§二十六）：同文件再次导入 → ErrBatchImported
	if _, err := Import(db, res); !errors.Is(err, ErrBatchImported) {
		t.Fatalf("同文件二次导入应 ErrBatchImported，实际 %v", err)
	}
}

// TestImportGuardNonEmpty 二次导入应被拒绝（防覆盖）。
func TestImportGuardNonEmpty(t *testing.T) {
	db := openMigratedDB(t, "guard")
	// 手工插入一台设备模拟非空库
	now := models.Now()
	eq := models.Equipment{EquipmentNo: nil, InternalCode: "EQ-000001", Name: "x", Status: "IN_STOCK",
		CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&eq).Error; err != nil {
		t.Fatal(err)
	}
	res := &ParseResult{Filename: "t.xlsx", Groups: []*Group{{
		Name: "y", Model: "", DSum: 1, HasNumericD: true, Unnumbered: 1, OK: true, RowFrom: 5, RowTo: 5,
	}}}
	_, err := Import(db, res)
	if !errors.Is(err, ErrDBNotEmpty) {
		t.Fatalf("期望 ErrDBNotEmpty，实际 %v", err)
	}
}

// TestClassifyF 决策 18 §五/§六：F 列编号单元分类。
func TestClassifyF(t *testing.T) {
	cases := []struct {
		in   string
		want fTokenKind
	}{
		{"6041", tkNumber}, {"001", tkNumber}, {"A01", tkNumber}, {"JUKI-8700", tkNumber},
		{"ABC-001", tkNumber}, {"车间A-01", tkNumber}, {"缝制A-02", tkNumber},
		{"设备一号", tkNumber}, {"中文编号", tkNumber}, // 情况 A 中文编号（含“号”收尾）
		{"无编号", tkNoNumber},
		{"拉布机配件", tkDesc}, {"拖布轮", tkDesc}, // 情况 B 明显描述
		{"", tkReview}, {"①②③", tkReview}, {"精密台面", tkReview}, // 情况 C 无法判断
	}
	for _, c := range cases {
		got, _ := classifyF(c.in)
		if got != c.want {
			t.Errorf("classifyF(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

// writeMiniXLSX 生成一个最小 xlsx（无合并单元格），R2 表头、R5 起数据。
func writeMiniXLSX(t *testing.T, rows [][]string) string {
	t.Helper()
	headers := []string{"类别", "设备名称", "设备型号", "台账数量", "财务数量", "台账设备编号", "时间", "公司", "台数", "借出设备编号", "备注"}
	return writeMiniXLSXWithHeaders(t, headers, rows)
}

// writeMiniXLSXWithHeaders 生成指定 R2 表头的最小 xlsx（用于模板校验测试）。
func writeMiniXLSXWithHeaders(t *testing.T, headers []string, rows [][]string) string {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellStr(sheet, cell, h); err != nil {
			t.Fatal(err)
		}
	}
	for ri, r := range rows {
		for ci, v := range r {
			cell, _ := excelize.CoordinatesToCellName(ci+1, sheetDataStart+ri)
			if err := f.SetCellStr(sheet, cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "mini.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestParseV11Semantics 决策 18 解析语义（Test 1/2/3/4）：
// 重复编号展开不 BLOCK、无编号按数量展开、中文编号保留、描述文本→无编号+备注。
func TestParseV11Semantics(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "4", "", "6041 6041 6041 6041"}, // Test1 同号 4 台真机
		{"缝纫设备", "缝制熨斗", "JUKI DDL-8700", "3", "", "无编号"},     // Test2 无编号展开 3
		{"裁剪设备", "裁床", "CUT-1", "1", "", "车间A-01"},            // Test3 中文编号保留
		{"裁剪设备", "程控器", "FX2NC", "1", "", "拉布机配件"},            // Test4 描述文本→无编号
		{"技术设备", "打样机", "P-9", "1", "", "精密台面"},               // 情况 C REVIEW
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.TotalD != 10 {
		t.Fatalf("台账数量合计应为 10，实际 %d", res.TotalD)
	}
	byKey := map[string]*Group{}
	for _, g := range res.Groups {
		byKey[g.Key] = g
	}
	// 平缝机/M1：Numbers 应为 4 个 6041（重复展开、允许），组 OK
	g := byKey["平缝机\x00M1"]
	if g == nil {
		t.Fatal("缺少 平缝机/M1 分组")
	}
	if len(g.Numbers) != 4 {
		t.Fatalf("6041 应展开 4 台，实际 %v", g.Numbers)
	}
	for _, n := range g.Numbers {
		if n != "6041" {
			t.Fatalf("重复展开编号异常: %q", n)
		}
	}
	if !g.OK {
		t.Fatal("重复编号不应 BLOCK 该组（决策18）")
	}
	// 无编号展开：缝制熨斗/JUKI DDL-8700 → Unnumbered 3
	g = byKey["缝制熨斗\x00JUKI DDL-8700"]
	if g == nil || g.Unnumbered != 3 || !g.OK {
		t.Fatalf("无编号展开异常: %+v", g)
	}
	// 中文编号原样保留
	g = byKey["裁床\x00CUT-1"]
	if g == nil || len(g.Numbers) != 1 || g.Numbers[0] != "车间A-01" || !g.OK {
		t.Fatalf("中文编号应原样保留: %+v", g)
	}
	// 描述文本→无编号+原文备注
	g = byKey["程控器\x00FX2NC"]
	if g == nil || g.Unnumbered != 1 || !strings.Contains(g.DescRemark, "拉布机配件") {
		t.Fatalf("描述文本应转无编号并保留原文: %+v", g)
	}
	if !g.OK {
		t.Fatal("描述文本不应 BLOCK 该组（决策18 情况B）")
	}
	// REVIEW：纯中文无法判断 → 组不自动导入，REVIEW 计数 +1
	g = byKey["打样机\x00P-9"]
	if g == nil || g.ReviewN != 1 || g.OK {
		t.Fatalf("REVIEW 组应标记不导入: %+v", g)
	}
	if res.ReviewN != 1 {
		t.Fatalf("ParseResult.ReviewN 应为 1，实际 %d", res.ReviewN)
	}
	hasReviewIssue := false
	for _, is := range res.Issues {
		if is.Level == IssueLevelReview {
			hasReviewIssue = true
		}
	}
	if !hasReviewIssue {
		t.Error("缺少 REVIEW 级问题")
	}
}

// TestImportV11DuplicateExpansion 同号多台真机可完整导入（含重复展开与 seq）。
func TestImportV11DuplicateExpansion(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "4", "", "6041 6041 6041 6041"},
		{"缝纫设备", "缝制熨斗", "JUKI DDL-8700", "2", "", "无编号"},
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	db := openMigratedDB(t, "dup")
	res.Filename = "mini.xlsx"
	report, err := Import(db, res)
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if report.Imported != 6 {
		t.Fatalf("应导入 6 台（4 同号+2 无编号），实际 %d", report.Imported)
	}
	if report.Unnumbered != 2 || report.BlockGroups != 0 {
		t.Fatalf("报告异常: un=%d block=%d", report.Unnumbered, report.BlockGroups)
	}
	var n int64
	db.Model(&models.Equipment{}).Where("equipment_no = ?", "6041").Count(&n)
	if n != 4 {
		t.Fatalf("6041 应 4 台，实际 %d", n)
	}
	// 同号 4 台的 equipment_seq 应 1..4（事务内 RenumberAllSeq，决策18 §七）
	var seqs []int
	db.Model(&models.Equipment{}).Where("equipment_no = ?", "6041").
		Order("id ASC").Pluck("equipment_seq", &seqs)
	if len(seqs) != 4 || seqs[0] != 1 || seqs[3] != 4 {
		t.Fatalf("6041 seq 应为 [1 2 3 4]，实际 %v", seqs)
	}
	// 无编号 seq：缝制熨斗/JUKI DDL-8700 → 1..2（按 model 分组）
	var useqs []int
	db.Model(&models.Equipment{}).Where("equipment_no IS NULL AND name = ? AND model = ?", "缝制熨斗", "JUKI DDL-8700").
		Order("id ASC").Pluck("equipment_seq", &useqs)
	if len(useqs) != 2 || useqs[0] != 1 || useqs[1] != 2 {
		t.Fatalf("无编号 seq 应为 [1 2]，实际 %v", useqs)
	}
	var un int64
	db.Model(&models.Equipment{}).Where("equipment_no IS NULL AND name = ? AND model = ?", "缝制熨斗", "JUKI DDL-8700").Count(&un)
	if un != 2 {
		t.Fatalf("无编号应 2 台，实际 %d", un)
	}
	// source key 与批次（决策18 §二十五/四十）
	var src int64
	db.Model(&models.Equipment{}).Where("import_batch_id = ? AND source_key <> ''", report.BatchID).Count(&src)
	if src != 6 {
		t.Fatalf("6 台设备应全部带批次来源键，实际 %d", src)
	}
	// 幂等（§二十六）：同 ParseResult 再导 → ErrBatchImported（库非空先命中批次指纹）
	if _, err := Import(db, res); !errors.Is(err, ErrBatchImported) {
		t.Fatalf("同文件二次导入应 ErrBatchImported，实际 %v", err)
	}
	// 不同文件（换文件名与 SourceHash）再导入 → 空库守卫 ErrDBNotEmpty
	res2 := &ParseResult{Filename: "other.xlsx", SourceHash: "ffffffff", Groups: res.Groups}
	if _, err := Import(db, res2); !errors.Is(err, ErrDBNotEmpty) {
		t.Fatalf("非空库导入应 ErrDBNotEmpty，实际 %v", err)
	}
}
