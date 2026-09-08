package service

import (
	"os"
	"path/filepath"
	"testing"

	"equipment/internal/database"
	"equipment/internal/models"
)

// fileDB 占位说明：备份类测试需要临时工作目录（backup/ 相对 cwd），见各用例。

func TestBackupPruneAndList(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old) //nolint:errcheck

	dbFile := filepath.Join(dir, "equipment.db")
	db, err := database.Open(dbFile)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := database.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	now := models.Now()
	if err := db.Create(&models.Equipment{
		InternalCode: "EQ-000001", Name: "x", Status: models.StatusInStock,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	name1, err := BackupNow(db, dbFile, 0)
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	list, err := ListBackups()
	if err != nil || len(list) != 1 {
		t.Fatalf("应 1 份备份: %v len=%d", err, len(list))
	}
	// 再备份一份并修剪到 1 份（保留最新 = name2）
	name2, err := BackupNow(db, dbFile, 0)
	if err != nil {
		t.Fatal(err)
	}
	if name1 == name2 {
		t.Fatal("同名备份不应发生")
	}
	PruneBackups(1)
	list, _ = ListBackups()
	if len(list) != 1 || list[0].Name != name2 {
		t.Fatalf("修剪后应保留最新 1 份: %+v", list)
	}
	// FileReplace 校验（用保留中的 name2）
	path, err := BackupFilePath(name2)
	if err != nil || path == "" {
		t.Fatalf("BackupFilePath(name2) 应存在: %v", err)
	}
	if _, err := BackupFilePath("../evil.db"); err == nil {
		t.Fatal("目录穿越应拒绝")
	}
}

func TestExportAndDashboard(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer os.Chdir(old) //nolint:errcheck

	dbFile := filepath.Join(dir, "equipment.db")
	db, err := database.Open(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.SQLDB(db)
	defer sqlDB.Close()
	if err := database.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	cid := catID(t, db, "裁剪设备")
	tm, _ := CreateTeam(db, "裁剪一组", "")
	for i := 0; i < 2; i++ {
		_, err := CreateEquipment(db, CreateEquipmentInput{
			Name: "环形割刀", Model: "EBK-SA", CategoryID: cid, Operator: "a",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	eqs, _ := CreateEquipment(db, CreateEquipmentInput{
		Name: "缝制熨斗", Model: "T-3NS", CategoryID: cid, Operator: "a",
	})
	_ = eqs
	if _, err := Transition(db, eqs.ID, FlowRequest{
		Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tm.ID,
	}); err != nil {
		t.Fatal(err)
	}

	// 导出非空且含表头
	data, err := ExportEquipmentXLSX(db, ExportFilter{})
	if err != nil || len(data) == 0 {
		t.Fatalf("导出失败: %v", err)
	}
	// 统计
	d, err := DashboardStats(db)
	if err != nil {
		t.Fatal(err)
	}
	if d.Total != 3 {
		t.Fatalf("总数应为 3, got %d", d.Total)
	}
	var inStock, inTeam int64
	for _, s := range d.ByStatus {
		if s.Status == models.StatusInStock {
			inStock = s.Count
		}
		if s.Status == models.StatusInTeam {
			inTeam = s.Count
		}
	}
	if inStock != 2 || inTeam != 1 {
		t.Fatalf("状态分布异常 in_stock=%d in_team=%d", inStock, inTeam)
	}
	if len(d.ByTeam) != 1 || d.ByTeam[0].Count != 1 {
		t.Fatalf("班组统计异常: %+v", d.ByTeam)
	}
	hasFlow := false
	for _, f := range d.RecentFlows {
		if f.Action == models.ActionOutToTeam && f.ToTeamName == "裁剪一组" {
			hasFlow = true
		}
	}
	if len(d.RecentFlows) < 3 || !hasFlow {
		t.Fatalf("最近流转异常: %+v", d.RecentFlows)
	}
}

// TestDashboardEmptyArrays 回归：空库时 dashboard 各列表须为非 nil 数组（避免前端 .length/白屏）。
func TestDashboardEmptyArrays(t *testing.T) {
	db := openDB(t)
	d, err := DashboardStats(db)
	if err != nil {
		t.Fatal(err)
	}
	if d.ByStatus == nil || d.ByCategory == nil || d.ByTeam == nil ||
		d.RecentFlows == nil || d.CurrentBorrows == nil {
		t.Fatalf("空库统计列表不得为 nil: %+v", d)
	}
	if len(d.ByStatus) != 6 {
		t.Fatalf("状态分布应含 6 个枚举: %d", len(d.ByStatus))
	}
	if len(d.ByCategory) != 0 || len(d.RecentFlows) != 0 {
		t.Fatalf("空库类别/最近流转应为空数组且非 nil")
	}
}
