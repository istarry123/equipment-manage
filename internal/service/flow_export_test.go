package service

import (
	"testing"
	"time"

	"equipment/internal/models"
)

// TestFlowExportCurrentInTeam 测试1/8：当前班组设备。
func TestFlowExportCurrentInTeam(t *testing.T) {
	db := openDB(t)
	tm, _ := CreateTeam(db, "A班", "")
	eq, err := CreateEquipment(db, CreateEquipmentInput{Name: "缝纫机", Model: "DDL-8700", Operator: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tm.ID}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Current) != 1 {
		t.Fatalf("当前应 1 台, got %d", len(data.Current))
	}
	r := data.Current[0]
	if r.TeamName != "A班" || r.StatusText != "班组使用" || r.Location != "A班" {
		t.Fatalf("班组设备字段异常: %+v", r)
	}
}

// TestFlowExportCurrentBorrowed 测试2/9：当前外借设备（含外借日期）。
func TestFlowExportCurrentBorrowed(t *testing.T) {
	db := openDB(t)
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6041"), Name: "缝纫机", Model: "DDL-8700", Operator: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	borrowAt := time.Date(2026, 8, 20, 0, 0, 0, 0, time.Local)
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID, OccurredAt: &borrowAt,
	}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Current) != 1 {
		t.Fatalf("当前应 1 台, got %d", len(data.Current))
	}
	r := data.Current[0]
	if r.StatusText != "外借" || r.Location != "莒县双发" || r.BorrowerName != "莒县双发" {
		t.Fatalf("外借字段异常: %+v", r)
	}
	if r.BorrowDate == nil || r.BorrowDate.Format("2006-01-02") != "2026-08-20" {
		t.Fatalf("外借日期异常: %v", r.BorrowDate)
	}

	// 测试9：外借公司筛选只返回当前借给莒县双发的设备
	data2, err := BuildFlowExportData(db, FlowExportFilter{
		BorrowerID: &br.ID, IncludeCurrent: true, IncludeSummary: true,
	})
	if err != nil || len(data2.Current) != 1 {
		t.Fatalf("外借公司筛选应命中 1 台: %v n=%d", err, len(data2.Current))
	}
}

// TestFlowExportReturnedDevice 测试3：已归还设备当前在库，但历史保留 BORROW+RETURN。
func TestFlowExportReturnedDevice(t *testing.T) {
	db := openDB(t)
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6061"), Name: "环形割刀", Model: "EBK-SA", Operator: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionReturnBorrow, Operator: "a"}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true, IncludeHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Current) != 1 {
		t.Fatalf("当前应 1 台")
	}
	c := data.Current[0]
	if c.BorrowerName != "" || c.BorrowDate != nil || c.Location != "在库" {
		t.Fatalf("已归还设备不应有当前外借: %+v", c)
	}
	// 历史应含 BORROW 与 RETURN
	var hasBorrow, hasReturn bool
	for _, h := range data.History {
		if h.Action == models.ActionBorrow {
			hasBorrow = true
		}
		if h.Action == models.ActionReturnBorrow {
			hasReturn = true
		}
	}
	if !hasBorrow || !hasReturn {
		t.Fatalf("历史应含 BORROW 与 RETURN: %+v", data.History)
	}
}

// TestFlowExportDuplicateDisplayNo 测试4：重复编号 6041×4 → 6041（1）..（4）。
func TestFlowExportDuplicateDisplayNo(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 4; i++ {
		if _, err := CreateEquipment(db, CreateEquipmentInput{
			EquipmentNo: no("6041"), Name: "缝纫机", Model: "DDL-8700", Operator: "a",
		}); err != nil {
			t.Fatal(err)
		}
	}
	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"6041（1）": false, "6041（2）": false, "6041（3）": false, "6041（4）": false}
	for _, r := range data.Current {
		if r.DisplayNo != "" {
			if _, ok := want[r.DisplayNo]; ok {
				want[r.DisplayNo] = true
			}
		}
	}
	for k, v := range want {
		if !v {
			t.Fatalf("缺少显示编号 %s: %+v", k, data.Current)
		}
	}
	// 原始编号都应是 6041
	for _, r := range data.Current {
		if r.EquipmentNo != "6041" {
			t.Fatalf("原始编号应为 6041: %+v", r)
		}
	}
}

// TestFlowExportUnnumberedDisplayNo 测试5：无编号按型号（n）展示。
func TestFlowExportUnnumberedDisplayNo(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 3; i++ {
		if _, err := CreateEquipment(db, CreateEquipmentInput{
			Name: "缝纫机", Model: "JUKI DDL-8700", Operator: "a",
		}); err != nil {
			t.Fatal(err)
		}
	}
	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"JUKI DDL-8700（1）": false, "JUKI DDL-8700（2）": false, "JUKI DDL-8700（3）": false}
	for _, r := range data.Current {
		if _, ok := want[r.DisplayNo]; ok {
			want[r.DisplayNo] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Fatalf("缺少无编号显示 %s: %+v", k, data.Current)
		}
	}
}

// TestFlowExportHistoryComplete 测试6：一条设备 4 条历史全部保留。
func TestFlowExportHistoryComplete(t *testing.T) {
	db := openDB(t)
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	tmA, _ := CreateTeam(db, "A班", "")
	tmB, _ := CreateTeam(db, "B班", "")
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6041"), Name: "缝纫机", Model: "DDL-8700", Operator: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)
	t2 := time.Date(2024, 2, 1, 0, 0, 0, 0, time.Local)
	t3 := time.Date(2025, 3, 1, 0, 0, 0, 0, time.Local)
	t4 := time.Date(2026, 8, 20, 0, 0, 0, 0, time.Local)
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID, OccurredAt: &t1}); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionReturnBorrow, Operator: "a", OccurredAt: &t2}); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tmA.ID, OccurredAt: &t3}); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionHandover, Operator: "a", ToTeamID: &tmB.ID, OccurredAt: &t4}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	// 历史完整不截断：至少含 BORROW / RETURN / OUT_TO_TEAM / HANDOVER 四条（另有 1 条 IMPORT_INIT 初始记录）
	seen := map[string]int{}
	for _, h := range data.History {
		seen[h.Action]++
	}
	for _, act := range []string{
		models.ActionBorrow, models.ActionReturnBorrow, models.ActionOutToTeam, models.ActionHandover,
	} {
		if seen[act] < 1 {
			t.Fatalf("历史缺少动作 %s: %+v", act, seen)
		}
	}
	if seen[models.ActionImportInit] < 1 {
		t.Fatalf("历史应含 IMPORT_INIT 初始记录: %+v", seen)
	}
}

// TestFlowExportFilterStatus 测试7：状态筛选作用于当前与汇总。
func TestFlowExportFilterStatus(t *testing.T) {
	db := openDB(t)
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	eq1, _ := CreateEquipment(db, CreateEquipmentInput{Name: "a", Operator: "a"}) // 在库
	eq2, _ := CreateEquipment(db, CreateEquipmentInput{Name: "b", Operator: "a"}) // 外借
	_, _ = eq1, eq2
	if _, err := Transition(db, eq2.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{Status: models.StatusBorrowed, IncludeCurrent: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Current) != 1 {
		t.Fatalf("外借筛选应 1 台, got %d", len(data.Current))
	}
	for _, r := range data.Current {
		if r.Status != models.StatusBorrowed {
			t.Fatalf("筛选后不应出现非 BORROWED: %+v", r)
		}
	}
	// 汇总状态也应匹配筛选（外借=1，其余=0）
	var borrowed int64
	for _, s := range data.Summary.ByStatus {
		if s.Name == "外借" {
			borrowed = s.Count
		}
	}
	if borrowed != 1 {
		t.Fatalf("筛选后外借统计应 1: %+v", data.Summary.ByStatus)
	}
}

// TestFlowExportChineseAndZeroNo 测试10/11：中文编号与 001 编号原样保留。
func TestFlowExportChineseAndZeroNo(t *testing.T) {
	db := openDB(t)
	if _, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("车间A-01"), Name: "x", Operator: "a",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("001"), Name: "y", Operator: "a",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, r := range data.Current {
		seen[r.EquipmentNo] = true
	}
	if !seen["车间A-01"] || !seen["001"] {
		t.Fatalf("中文/001 编号应原样保留: %+v", data.Current)
	}
}

// TestFlowExportEmptyFilter 测试12：无匹配结果的筛选仍成功，返回空切片非 nil。
func TestFlowExportEmptyFilter(t *testing.T) {
	db := openDB(t)
	if _, err := CreateEquipment(db, CreateEquipmentInput{Name: "x", Operator: "a"}); err != nil {
		t.Fatal(err)
	}
	data, err := BuildFlowExportData(db, FlowExportFilter{
		Status: models.StatusScrapped, IncludeCurrent: true, IncludeHistory: true, IncludeSummary: true,
	})
	if err != nil {
		t.Fatalf("空结果不应报错: %v", err)
	}
	if data.Current == nil || data.History == nil || data.Summary.ByStatus == nil {
		t.Fatalf("空结果各切片应为非 nil: %+v", data)
	}
	if len(data.Current) != 0 || data.Total != 0 {
		t.Fatalf("空结果应为 0 台: total=%d", data.Total)
	}
}

// TestFlowExportHistoryLocation 历史位置推导（出库/转交/借出/归还 的原/新位置）。
func TestFlowExportHistoryLocation(t *testing.T) {
	db := openDB(t)
	tmA, _ := CreateTeam(db, "A班", "")
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	eq, _ := CreateEquipment(db, CreateEquipmentInput{Name: "缝纫机", Model: "DDL-8700", Operator: "a"})
	// 出库给 A班
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tmA.ID}); err != nil {
		t.Fatal(err)
	}
	// 外借
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	// 最新在前：BORROW（新位置=莒县双发）
	if len(data.History) < 2 {
		t.Fatalf("历史应 >=2: %+v", data.History)
	}
	borrow := data.History[0]
	if borrow.Action != models.ActionBorrow {
		t.Fatalf("最新一条应为 BORROW: %+v", borrow)
	}
	if borrow.FromLocation != "A班" || borrow.ToLocation != "莒县双发" {
		t.Fatalf("BORROW 位置异常: %+v", borrow)
	}
}

// TestFlowExportFilterTeam 测试8：班组筛选只返回该班组设备。
func TestFlowExportFilterTeam(t *testing.T) {
	db := openDB(t)
	tmA, _ := CreateTeam(db, "A班", "")
	tmB, _ := CreateTeam(db, "B班", "")
	eq1, _ := CreateEquipment(db, CreateEquipmentInput{Name: "a", Operator: "a"})
	eq2, _ := CreateEquipment(db, CreateEquipmentInput{Name: "b", Operator: "a"})
	if _, err := Transition(db, eq1.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tmA.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq2.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tmB.ID}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{TeamID: &tmA.ID, IncludeCurrent: true, IncludeHistory: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Current) != 1 || data.Current[0].TeamName != "A班" {
		t.Fatalf("班组筛选应仅 A班 1 台: %+v", data.Current)
	}
	// 历史也只应含 A班设备（eq1）的历史
	for _, h := range data.History {
		if h.EquipmentID != eq1.ID {
			t.Fatalf("历史混入非筛选班组设备: %+v", h)
		}
	}
	// 汇总班组统计应仅 A班 1
	var aCount int64
	for _, s := range data.Summary.ByTeam {
		if s.Name == "A班" {
			aCount = s.Count
		}
	}
	if aCount != 1 {
		t.Fatalf("筛选后 A班统计应 1: %+v", data.Summary.ByTeam)
	}
}

// TestFlowExportTwoThousand 测试13：2000 台设备批量导出行数对账。
func TestFlowExportTwoThousand(t *testing.T) {
	db := openDB(t)
	const n = 2000
	now := models.Now()
	for i := 1; i <= n; i++ {
		eq := models.Equipment{
			EquipmentNo:  no("D" + itoa(i)),
			InternalCode: "EQ-" + itoa(i),
			Name:         "设备", Model: "M",
			Status: models.StatusInStock, EquipmentSeq: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true, IncludeHistory: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	if data.Total != n || len(data.Current) != n {
		t.Fatalf("总数/当前应 %d, got total=%d current=%d", n, data.Total, len(data.Current))
	}
	if len(data.Summary.ByStatus) != 6 {
		t.Fatalf("统计应含 6 状态: %+v", data.Summary.ByStatus)
	}
	// 在库统计应 2000
	var inStock int64
	for _, s := range data.Summary.ByStatus {
		if s.Name == "在库" {
			inStock = s.Count
		}
	}
	if inStock != n {
		t.Fatalf("在库应 %d, got %d", n, inStock)
	}
}

// TestFlowExportIntegration 集成测试（§六十九）：建→分配班组→借出→归还→再借出→导出。
func TestFlowExportIntegration(t *testing.T) {
	db := openDB(t)
	tm, _ := CreateTeam(db, "裁剪一组", "")
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	eq, err := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: no("6041"), Name: "缝纫机", Model: "DDL-8700", Operator: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// 建 → 分配班组
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tm.ID}); err != nil {
		t.Fatal(err)
	}
	// 借出
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID}); err != nil {
		t.Fatal(err)
	}
	// 归还
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionReturnBorrow, Operator: "a"}); err != nil {
		t.Fatal(err)
	}
	// 再借出
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID}); err != nil {
		t.Fatal(err)
	}

	data, err := BuildFlowExportData(db, FlowExportFilter{IncludeCurrent: true, IncludeHistory: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	// Sheet1：当前状态 = 外借
	if len(data.Current) != 1 || data.Current[0].StatusText != "外借" {
		t.Fatalf("当前应外借: %+v", data.Current)
	}
	// Sheet2：完整历史（IMPORT_INIT + 出库 + 借出 + 归还 + 借出 = 5）
	var borrow, returnB, outTeam int
	for _, h := range data.History {
		switch h.Action {
		case models.ActionBorrow:
			borrow++
		case models.ActionReturnBorrow:
			returnB++
		case models.ActionOutToTeam:
			outTeam++
		}
	}
	if borrow != 2 || returnB != 1 || outTeam != 1 {
		t.Fatalf("历史动作计数异常 borrow=%d return=%d out=%d: %+v", borrow, returnB, outTeam, data.History)
	}
	// Sheet3：状态统计当前外借 1
	var borrowed int64
	for _, s := range data.Summary.ByStatus {
		if s.Name == "外借" {
			borrowed = s.Count
		}
	}
	if borrowed != 1 {
		t.Fatalf("外借统计应 1: %+v", data.Summary.ByStatus)
	}
}
