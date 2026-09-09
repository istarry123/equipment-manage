package importer

import (
	"testing"

	"equipment/internal/models"
)

// TestBuildReviewViewMini 设备级预览 + 疑似候选（仅外部公司）+ 事件计数。
func TestBuildReviewViewMini(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "3", "", "6061 6062 6063", "", "", "", "", ""},
		{"", "", "", "", "", "", "2022.6.14", "泰和", "1", "6061", ""}, // 外部公司 → 疑似
		{"缝纫设备", "缝制熨斗", "JUKI", "2", "", "无编号", "", "", "", "", ""},
		{"", "", "", "", "", "", "2020.5.5", "莒县双发", "1", "无编号", ""}, // 内部单位 → 不入疑似
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	devices, suspected, _, borrowEv, internalEv := BuildReviewView(res)
	if len(devices) != 5 { // 3 编号 + 2 无编号
		t.Fatalf("设备级预览应为 5 行，实际 %d", len(devices))
	}
	if borrowEv != 2 || internalEv != 1 {
		t.Fatalf("事件计数异常: borrow=%d internal=%d", borrowEv, internalEv)
	}
	if len(suspected) != 1 || suspected[0].DisplayNo != "6061" || suspected[0].Company != "泰和" {
		t.Fatalf("疑似候选异常: %+v", suspected)
	}
}

// TestImportConfirmSuspect 勾选疑似 → 同事务置 BORROWED + borrow_record + flow。
func TestImportConfirmSuspect(t *testing.T) {
	rows := [][]string{
		{"缝纫设备", "平缝机", "M1", "3", "", "6061 6062 6063", "", "", "", "", ""},
		{"", "", "", "", "", "", "2022.6.14", "泰和", "1", "6061", ""},
		{"裁剪设备", "拉布机", "CM-01", "2", "", "492013 492024", "", "", "", "", ""},
		{"", "", "", "", "", "", "2026.7.28", "刘家庄华欣", "1", "492024（2026.9.4入南库）", ""},
	}
	path := writeMiniXLSX(t, rows)
	res, err := Parse(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	_, suspected, _, _, _ := BuildReviewView(res)
	if len(suspected) != 1 {
		t.Fatalf("疑似候选应为 1（6061），实际 %+v", suspected)
	}
	db := openMigratedDB(t, "confirm")
	res.Filename = "mini.xlsx"
	report, err := ImportWithOptions(db, res, ImportOptions{
		ConfirmedSuspects: []string{suspected[0].SourceKey},
	})
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	if report.BorrowedNow != 1 {
		t.Fatalf("应确认在借 1 台，实际 %d", report.BorrowedNow)
	}
	var eq models.Equipment
	if err := db.Where("equipment_no = ?", "6061").First(&eq).Error; err != nil {
		t.Fatalf("6061 缺失: %v", err)
	}
	if eq.Status != models.StatusBorrowed {
		t.Fatalf("6061 应置 BORROWED，实际 %s", eq.Status)
	}
	if eq.CurrentBorrowerID == nil || eq.CurrentBorrowRecordID == nil {
		t.Fatalf("6061 应绑定外借方与外借单")
	}
	var br models.BorrowRecord
	if err := db.First(&br, *eq.CurrentBorrowRecordID).Error; err != nil {
		t.Fatal(err)
	}
	if br.BorrowDate.Format("2006-01-02") != "2022-06-14" {
		t.Fatalf("borrow_date 应为 2022-06-14，实际 %s", br.BorrowDate.Format("2006-01-02"))
	}
	// 外借方字典已建
	var b models.Borrower
	if err := db.First(&b, *eq.CurrentBorrowerID).Error; err != nil || b.Name != "泰和" {
		t.Fatalf("外借方异常: %+v err=%v", b, err)
	}
	// flow：IMPORT_INIT + BORROW
	var n int64
	db.Model(&models.Transaction{}).Where("equipment_id = ? AND action = ?", eq.ID, models.ActionBorrow).Count(&n)
	if n != 1 {
		t.Fatalf("应有 1 条 BORROW flow，实际 %d", n)
	}
	// 未勾选设备保持 IN_STOCK
	var eq2 models.Equipment
	if err := db.Where("equipment_no = ?", "6062").First(&eq2).Error; err != nil {
		t.Fatal(err)
	}
	if eq2.Status != models.StatusInStock {
		t.Fatalf("6062 不应受影响，实际 %s", eq2.Status)
	}
}
