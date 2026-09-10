package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"equipment/internal/database"

	"github.com/xuri/excelize/v2"
)

// TestFlowExportReconcileRealDB 真实库对账（§七十八/§七十九）：
// 把工作区 equipment.db 拷贝到临时目录（绝不改原库），全量导出后核对
//   DB equipment 数 ↔ Sheet1 数据行数；DB flow_record 数 ↔ Sheet2 数据行数。
// 仅当工作区存在 equipment.db 时执行（本地终验）；CI 无该文件则跳过。
func TestFlowExportReconcileRealDB(t *testing.T) {
	src := filepath.Join("..", "..", "equipment.db")
	if _, err := os.Stat(src); err != nil {
		t.Skip("工作区无 equipment.db，跳过真实库对账")
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "equipment.db")
	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, srcData, 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := database.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	// DB 实际计数（SQL 直查，作为对账基准）
	var dbEquipment, dbFlow int64
	if err := db.Raw("SELECT COUNT(*) FROM equipment").Scan(&dbEquipment).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT COUNT(*) FROM flow_record").Scan(&dbFlow).Error; err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true, IncludeHistory: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	// 查询层计数与 DB 一致
	if int64(len(data.Current)) != dbEquipment {
		t.Fatalf("查询层当前 %d != DB equipment %d", len(data.Current), dbEquipment)
	}
	if int64(len(data.History)) != dbFlow {
		t.Fatalf("查询层历史 %d != DB flow_record %d", len(data.History), dbFlow)
	}

	xlsx, err := RenderFlowExportXLSX(data)
	if err != nil {
		t.Fatal(err)
	}
	xf, err := excelize.OpenReader(bytes.NewReader(xlsx))
	if err != nil {
		t.Fatal(err)
	}
	curRows, _ := xf.GetRows(flowSheetCurrent)
	hisRows, _ := xf.GetRows(flowSheetHistory)
	// 去掉标题行(1) + 表头行(1)
	curData := len(curRows) - 2
	hisData := len(hisRows) - 2
	if curData < 0 {
		curData = 0
	}
	if hisData < 0 {
		hisData = 0
	}
	if int64(curData) != dbEquipment {
		t.Fatalf("Sheet1 数据行 %d != DB equipment %d", curData, dbEquipment)
	}
	if int64(hisData) != dbFlow {
		t.Fatalf("Sheet2 数据行 %d != DB flow_record %d", hisData, dbFlow)
	}
	t.Logf("真实库对账 PASS：equipment %d ↔ Sheet1 %d；flow_record %d ↔ Sheet2 %d",
		dbEquipment, curData, dbFlow, hisData)
}
