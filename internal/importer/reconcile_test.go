package importer

import (
	"testing"

	"equipment/internal/models"
)

// TestReconcilePass 全量导入后对账 PASS（无缺失/多出）。
func TestReconcilePass(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "3", "", "6061 6062 6063", "", "", "", "", ""},
		{"缝纫设备", "缝制熨斗", "JUKI", "2", "", "无编号", "", "", "", "", ""},
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	db := openMigratedDB(t, "recon_pass")
	res.Filename = "mini.xlsx"
	if _, err := Import(db, res); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	rep, err := Reconcile(db, res)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if !rep.Pass {
		t.Fatalf("应 PASS: %+v", rep)
	}
	if rep.ExcelOKDevs != 5 || rep.DBBatchDevs != 5 ||
		rep.NumExcel != 3 || rep.NumDB != 3 ||
		rep.UnExcel != 2 || rep.UnDB != 2 {
		t.Fatalf("对账计数异常: %+v", rep)
	}
	if len(rep.Missing) != 0 || len(rep.Extra) != 0 {
		t.Fatalf("不应有差异: missing=%d extra=%d", len(rep.Missing), len(rep.Extra))
	}
}

// TestReconcileFailAfterDelete 人为删除一台 → FAIL 并列出缺失。
func TestReconcileFailAfterDelete(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "3", "", "6061 6062 6063", "", "", "", "", ""},
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	db := openMigratedDB(t, "recon_fail")
	res.Filename = "mini.xlsx"
	if _, err := Import(db, res); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	// 删除 6062（模拟漏写；先删关联 flow/borrow 满足外键）
	var eq models.Equipment
	if err := db.Where("equipment_no = ?", "6062").First(&eq).Error; err != nil {
		t.Fatal(err)
	}
	db.Where("equipment_id = ?", eq.ID).Delete(&models.Transaction{})  //nolint:errcheck
	db.Where("equipment_id = ?", eq.ID).Delete(&models.BorrowRecord{}) //nolint:errcheck
	if err := db.Delete(&eq).Error; err != nil {
		t.Fatal(err)
	}
	rep, err := Reconcile(db, res)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if rep.Pass {
		t.Fatal("删除后对账应 FAIL")
	}
	if len(rep.Missing) != 1 || rep.Missing[0].EquipmentNo != "6062" {
		t.Fatalf("缺失清单异常: %+v", rep.Missing)
	}
}
