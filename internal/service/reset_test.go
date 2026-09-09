package service

import (
	"testing"

	"equipment/internal/models"
)

// TestClearImportDataKeepsDictionaries 清空只删业务数据，保留字典与 audit。
func TestClearImportDataKeepsDictionaries(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "裁剪设备")
	// 字典
	team, err := CreateTeam(db, "裁剪一组", "")
	if err != nil {
		t.Fatal(err)
	}
	borrower, err := CreateBorrower(db, "泰和", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// 设备 + 初始流转
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6061"), Name: "环形割刀", Model: "EBK-SA", CategoryID: cid, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 出库到班组 → 追加一条 flow（共 IMPORT_INIT + OUT_TO_TEAM = 2）
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &team.ID}); err != nil {
		t.Fatal(err)
	}
	// 外借单（直接构造一笔在库借出，构造 borrow_record 独立于状态机的简单方式）
	now := models.Now()
	if err := db.Create(&models.BorrowRecord{
		EquipmentID: eq.ID, BorrowerID: borrower.ID,
		BorrowDate: now, Status: models.BorrowOutstanding,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// 批次记录
	if err := db.Create(&models.ImportBatch{
		SourceName: "x.xlsx", SourceHash: "abc", Status: "DONE",
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	db.Create(&models.AuditLog{Action: "TEST", Target: "x", Operator: "t", CreatedAt: now}) //nolint:errcheck

	res, err := ClearImportData(db)
	if err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if res.EquipmentDeleted != 1 || res.FlowDeleted != 2 ||
		res.BorrowDeleted != 1 || res.BatchDeleted != 1 {
		t.Fatalf("清空计数异常: %+v", res)
	}
	var eqN, brN, flN, btN int64
	db.Model(&models.Equipment{}).Count(&eqN)
	db.Model(&models.BorrowRecord{}).Count(&brN)
	db.Model(&models.Transaction{}).Count(&flN)
	db.Model(&models.ImportBatch{}).Count(&btN)
	if eqN+brN+flN+btN != 0 {
		t.Fatalf("业务数据应清空: equipment=%d borrow=%d flow=%d batch=%d", eqN, brN, flN, btN)
	}
	// 字典与 audit 保留
	var catN, teamN, borrN, audN int64
	db.Model(&models.Category{}).Count(&catN)
	db.Model(&models.Team{}).Count(&teamN)
	db.Model(&models.Borrower{}).Count(&borrN)
	db.Model(&models.AuditLog{}).Count(&audN)
	if catN != 1 || teamN != 1 || borrN != 1 || audN < 1 {
		t.Fatalf("字典/audit 不应被清空: cat=%d team=%d borrower=%d audit=%d", catN, teamN, borrN, audN)
	}
}
