package service

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"equipment/internal/models"

	"github.com/xuri/excelize/v2"
)

// buildFlowXLSX 便捷构造并读回。
func buildFlowXLSX(t *testing.T, f FlowExportFilter) *excelize.File {
	t.Helper()
	db := openDB(t)
	data, err := BuildFlowExportXLSX(db, f)
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	xf, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	return xf
}

// TestFlowXLSXThreeSheets 三个 Sheet 名称与存在性。
func TestFlowXLSXThreeSheets(t *testing.T) {
	xf := buildFlowXLSX(t, FlowExportFilter{IncludeCurrent: true, IncludeHistory: true, IncludeSummary: true})
	for _, s := range []string{flowSheetCurrent, flowSheetHistory, flowSheetSummary} {
		if _, err := xf.GetSheetIndex(s); err != nil {
			t.Fatalf("缺少 Sheet %s: %v", s, err)
		}
	}
}

// TestFlowXLSXTextNumberAndHeader 表头 + 001/中文编号以文本写入（不被转数值）。
func TestFlowXLSXTextNumberAndHeader(t *testing.T) {
	db := openDB(t)
	if _, err := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: no("001"), Name: "y", Operator: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: no("车间A-01"), Name: "x", Operator: "a"}); err != nil {
		t.Fatal(err)
	}
	data, err := BuildFlowExportXLSX(db, FlowExportFilter{IncludeCurrent: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	xf, _ := excelize.OpenReader(bytes.NewReader(data))
	rows, err := xf.GetRows(flowSheetCurrent)
	if err != nil {
		t.Fatal(err)
	}
	// 第 2 行表头
	if len(rows) < 2 || rows[1][0] != "序号" || rows[1][3] != "设备编号" || rows[1][4] != "原始编号" {
		t.Fatalf("表头异常: %v", rows[1])
	}
	// 数据行的原始编号列（E）应原样 001 / 车间A-01
	seen := map[string]bool{}
	for i := 2; i < len(rows); i++ {
		if len(rows[i]) > 4 {
			seen[rows[i][4]] = true
		}
	}
	if !seen["001"] || !seen["车间A-01"] {
		t.Fatalf("001/中文编号应原样保留为文本: %v", rows)
	}
}

// TestFlowXLSXHiddenIDCol 设备ID列应隐藏（SetColVisible false）。
func TestFlowXLSXHiddenIDCol(t *testing.T) {
	db := openDB(t)
	if _, err := CreateEquipment(db, CreateEquipmentInput{Name: "x", Operator: "a"}); err != nil {
		t.Fatal(err)
	}
	data, err := BuildFlowExportXLSX(db, FlowExportFilter{IncludeCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	xf, _ := excelize.OpenReader(bytes.NewReader(data))
	vis, err := xf.GetColVisible(flowSheetCurrent, "L")
	if err != nil {
		t.Fatal(err)
	}
	if vis {
		t.Fatalf("设备ID列应隐藏")
	}
}

// TestFlowXLSXSummaryBlocks 统计 Sheet 含三个区块标题与导出时间。
func TestFlowXLSXSummaryBlocks(t *testing.T) {
	db := openDB(t)
	tm, _ := CreateTeam(db, "A班", "")
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	eq1, _ := CreateEquipment(db, CreateEquipmentInput{Name: "a", Operator: "a"})
	if _, err := Transition(db, eq1.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a", ToTeamID: &tm.ID}); err != nil {
		t.Fatal(err)
	}
	eq2, _ := CreateEquipment(db, CreateEquipmentInput{Name: "b", Operator: "a"})
	if _, err := Transition(db, eq2.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID}); err != nil {
		t.Fatal(err)
	}
	data, err := BuildFlowExportXLSX(db, FlowExportFilter{IncludeCurrent: true, IncludeSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	xf, _ := excelize.OpenReader(bytes.NewReader(data))
	rows, err := xf.GetRows(flowSheetSummary)
	if err != nil {
		t.Fatal(err)
	}
	// 平铺成字符串判定关键区块标题存在
	flat := ""
	for _, r := range rows {
		for _, c := range r {
			flat += c + "|"
		}
	}
	for _, kw := range []string{"设备流转情况统计", "导出时间：", "按状态统计", "按班组统计", "按外借公司统计"} {
		if !strings.Contains(flat, kw) {
			t.Fatalf("统计 Sheet 缺少 %s: %v", kw, rows)
		}
	}
}

// TestFlowXLSXEmptyFilter 空筛选仍生成可打开的三 Sheet（含占位文案）。
func TestFlowXLSXEmptyFilter(t *testing.T) {
	db := openDB(t)
	if _, err := CreateEquipment(db, CreateEquipmentInput{Name: "x", Operator: "a"}); err != nil {
		t.Fatal(err)
	}
	data, err := BuildFlowExportXLSX(db, FlowExportFilter{Status: models.StatusScrapped, IncludeCurrent: true, IncludeHistory: true, IncludeSummary: true})
	if err != nil || len(data) == 0 {
		t.Fatalf("空筛选生成失败: %v", err)
	}
	xf, _ := excelize.OpenReader(bytes.NewReader(data))
	rows, _ := xf.GetRows(flowSheetCurrent)
	// 第 3 行应含占位文案
	if len(rows) < 3 {
		t.Fatalf("空筛选应有占位行: %v", rows)
	}
}

// TestFlowXLSXDateFormats 外借日期 / 流转时间格式统一。
func TestFlowXLSXDateFormats(t *testing.T) {
	db := openDB(t)
	br, _ := CreateBorrower(db, "莒县双发", "", "")
	eq, _ := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: no("6041"), Name: "缝纫机", Model: "DDL-8700", Operator: "a"})
	bd := time.Date(2026, 8, 20, 0, 0, 0, 0, time.Local)
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a", BorrowerID: &br.ID, OccurredAt: &bd}); err != nil {
		t.Fatal(err)
	}
	data, err := BuildFlowExportXLSX(db, FlowExportFilter{IncludeCurrent: true, IncludeHistory: true})
	if err != nil {
		t.Fatal(err)
	}
	xf, _ := excelize.OpenReader(bytes.NewReader(data))
	cur, _ := xf.GetRows(flowSheetCurrent)
	// 外借日期列（J）应为 yyyy-mm-dd
	if len(cur) < 3 || len(cur[2]) <= 9 || cur[2][9] != "2026-08-20" {
		t.Fatalf("外借日期格式异常: %v", cur)
	}
	his, _ := xf.GetRows(flowSheetHistory)
	// 流转时间列（E）应为 yyyy-mm-dd hh:mm:ss
	found := false
	for i := 2; i < len(his); i++ {
		if len(his[i]) > 4 && len(his[i][4]) == len("2026-08-20 00:00:00") {
			found = true
		}
	}
	if !found {
		t.Fatalf("流转时间格式异常: %v", his)
	}
}
