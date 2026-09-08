package importer

import (
	"errors"
	"path/filepath"
	"testing"

	"equipment/internal/database"
	"equipment/internal/models"

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

	// 存在描述文本充当编号的 V10 WARN（R16 拉布机配件）
	hasV10 := false
	// 存在同组重复编号的 V3 BLOCK（源数据疑似重复）
	hasV3 := false
	for _, is := range res.Issues {
		if is.Code == "V10" {
			hasV10 = true
		}
		if is.Code == "V3" {
			hasV3 = true
		}
	}
	if !hasV10 {
		t.Error("缺少 V10（描述文本充当编号）问题记录")
	}
	if !hasV3 {
		t.Error("缺少 V3（同组重复编号）问题记录——源数据应存在疑似重复")
	}

	// 无编号台数合计 567 = 566（F 列"无编号"85 行） + 1（R16 描述文本"拉布机配件"）
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

// TestImportRealFile 真实文件导入：单事务写入、初始流转记录、编号/无编号台账。
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
	if report.BlockGroups == 0 {
		t.Log("提示：本次未命中任何 BLOCK 分组（源数据重复疑点未触发）")
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
