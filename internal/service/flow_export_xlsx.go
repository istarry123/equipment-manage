package service

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// 设备流转情况 Excel 生成（Equipment Flow Export v1.1）——生成层。
// 依赖查询层 BuildFlowExportData；职责：三 Sheet 布局 + 专业样式 + 文本/日期格式安全。

const (
	flowSheetCurrent = "当前流转情况"
	flowSheetHistory = "流转历史"
	flowSheetSummary = "统计汇总"

	dateLayout  = "2006-01-02"
	dateTimeLay = "2006-01-02 15:04:05"
)

// flowSty 一次构建的样式 id（excelize 样式 id 按 File 实例独立，须随构建传入，不可全局缓存）。
type flowSty struct {
	title  int // 标题：加粗、14 号
	header int // 表头：加粗、居中、自动换行、浅灰底
	cell   int // 普通单元格：垂直居中、自动换行
}

// newFlowSty 注册本次构建的三类样式。
func newFlowSty(x *excelize.File) (flowSty, error) {
	var st flowSty
	var err error
	if st.title, err = x.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	}); err != nil {
		return st, err
	}
	if st.header, err = x.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#F2F2F2"}, Pattern: 1},
	}); err != nil {
		return st, err
	}
	if st.cell, err = x.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	}); err != nil {
		return st, err
	}
	return st, nil
}

// BuildFlowExportXLSX 生成「设备流转情况.xlsx」字节内容。
func BuildFlowExportXLSX(db *gorm.DB, f FlowExportFilter) ([]byte, error) {
	data, err := BuildFlowExportData(db, f)
	if err != nil {
		return nil, err
	}

	x := excelize.NewFile()
	st, err := newFlowSty(x)
	if err != nil {
		return nil, err
	}
	// 删除默认 Sheet1，改由各 Sheet 自建
	_ = x.SetSheetName("Sheet1", flowSheetCurrent)
	if err := buildCurrentSheet(x, st, data); err != nil {
		return nil, err
	}
	if _, err := x.NewSheet(flowSheetHistory); err != nil {
		return nil, err
	}
	if err := buildHistorySheet(x, st, data); err != nil {
		return nil, err
	}
	if _, err := x.NewSheet(flowSheetSummary); err != nil {
		return nil, err
	}
	if err := buildSummarySheet(x, st, data); err != nil {
		return nil, err
	}
	// 第一个 Sheet 为活动页
	if idx, err := x.GetSheetIndex(flowSheetCurrent); err == nil {
		x.SetActiveSheet(idx)
	}
	buf, err := x.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// setTitleRow 写合并标题行（第 1 行），返回表头所在行号（=2）。
func setTitleRow(x *excelize.File, st flowSty, sheet, title string, colCount int) (int, error) {
	last, _ := excelize.CoordinatesToCellName(colCount, 1)
	if err := x.MergeCell(sheet, "A1", last); err != nil {
		return 0, err
	}
	if err := x.SetCellValue(sheet, "A1", title); err != nil {
		return 0, err
	}
	if err := x.SetCellStyle(sheet, "A1", last, st.title); err != nil {
		return 0, err
	}
	if err := x.SetRowHeight(sheet, 1, 26); err != nil {
		return 0, err
	}
	return 2, nil
}

// writeHeader 写表头行，返回表头行号。
func writeHeader(x *excelize.File, st flowSty, sheet string, row int, headers []string) error {
	cell, _ := excelize.CoordinatesToCellName(1, row)
	if err := x.SetSheetRow(sheet, cell, &headers); err != nil {
		return err
	}
	last, _ := excelize.CoordinatesToCellName(len(headers), row)
	if err := x.SetCellStyle(sheet, cell, last, st.header); err != nil {
		return err
	}
	return x.SetRowHeight(sheet, row, 24)
}

// setColWidths 按列设置宽度（A 起）。
func setColWidths(x *excelize.File, sheet string, widths []float64) error {
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		if err := x.SetColWidth(sheet, col, col, w); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Sheet1 当前流转情况 ----------

func buildCurrentSheet(x *excelize.File, st flowSty, data *FlowExportData) error {
	headers := []string{
		"序号", "设备名称", "设备型号", "设备编号", "原始编号", "所属班组",
		"当前状态", "当前所在位置", "外借公司", "外借日期", "备注", "设备ID",
	}
	headerRow, err := setTitleRow(x, st, flowSheetCurrent, "设备当前流转情况", len(headers))
	if err != nil {
		return err
	}
	if err := writeHeader(x, st, flowSheetCurrent, headerRow, headers); err != nil {
		return err
	}
	if err := setColWidths(x, flowSheetCurrent, []float64{
		6, 16, 16, 14, 14, 12, 10, 14, 16, 12, 20, 10,
	}); err != nil {
		return err
	}

	dataRow := headerRow + 1
	if len(data.Current) == 0 {
		if err := x.SetCellStr(flowSheetCurrent, "A"+itoa(dataRow), "当前筛选条件下没有设备。"); err != nil {
			return err
		}
	} else {
		for i, r := range data.Current {
			row := dataRow + i
			vals := []any{
				i + 1, r.Name, r.Model, r.DisplayNo, r.EquipmentNo, r.TeamName,
				r.StatusText, r.Location, r.BorrowerName, "", r.Remark, r.ID,
			}
			if r.BorrowDate != nil {
				vals[9] = r.BorrowDate.Format(dateLayout)
			}
			cell, _ := excelize.CoordinatesToCellName(1, row)
			if err := x.SetSheetRow(flowSheetCurrent, cell, &vals); err != nil {
				return err
			}
			// 编号两列强制文本（001/中文编号安全）
			if err := x.SetCellStr(flowSheetCurrent, "D"+itoa(row), r.DisplayNo); err != nil {
				return err
			}
			if err := x.SetCellStr(flowSheetCurrent, "E"+itoa(row), r.EquipmentNo); err != nil {
				return err
			}
			last, _ := excelize.CoordinatesToCellName(len(headers), row)
			if err := x.SetCellStyle(flowSheetCurrent, cell, last, st.cell); err != nil {
				return err
			}
		}
	}

	// 冻结表头（标题+表头共 2 行，数据从第 3 行开始）
	if err := x.SetPanes(flowSheetCurrent, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 0, YSplit: headerRow, TopLeftCell: "A" + itoa(headerRow+1), ActivePane: "bottomLeft",
	}); err != nil {
		return err
	}
	// 自动筛选（覆盖表头到最后一个数据行，不含隐藏 id 列）
	lastDataRow := headerRow + len(data.Current)
	if lastDataRow < headerRow {
		lastDataRow = headerRow
	}
	lastCol, _ := excelize.ColumnNumberToName(len(headers) - 1)
	ref := fmt.Sprintf("A%d:%s%d", headerRow, lastCol, lastDataRow)
	if err := x.AutoFilter(flowSheetCurrent, ref, []excelize.AutoFilterOptions{}); err != nil {
		return err
	}
	// 隐藏设备ID列（第 12 列 L，仅技术核对用）
	return x.SetColVisible(flowSheetCurrent, "L", false)
}

// ---------- Sheet2 流转历史 ----------

func buildHistorySheet(x *excelize.File, st flowSty, data *FlowExportData) error {
	headers := []string{
		"序号", "设备名称", "设备型号", "设备编号", "流转时间", "流转类型",
		"原位置", "新位置", "班组", "外借公司", "操作备注",
	}
	headerRow, err := setTitleRow(x, st, flowSheetHistory, "设备流转历史", len(headers))
	if err != nil {
		return err
	}
	if err := writeHeader(x, st, flowSheetHistory, headerRow, headers); err != nil {
		return err
	}
	if err := setColWidths(x, flowSheetHistory, []float64{
		6, 16, 16, 14, 18, 12, 14, 14, 12, 16, 24,
	}); err != nil {
		return err
	}

	dataRow := headerRow + 1
	if len(data.History) == 0 {
		if err := x.SetCellStr(flowSheetHistory, "A"+itoa(dataRow), "当前筛选条件下没有历史流转记录。"); err != nil {
			return err
		}
	} else {
		for i, r := range data.History {
			row := dataRow + i
			vals := []any{
				i + 1, r.Name, r.Model, r.DisplayNo, r.OccurredAt.Format(dateTimeLay),
				r.ActionText, r.FromLocation, r.ToLocation, r.TeamName, r.BorrowerName, r.Remark,
			}
			cell, _ := excelize.CoordinatesToCellName(1, row)
			if err := x.SetSheetRow(flowSheetHistory, cell, &vals); err != nil {
				return err
			}
			if err := x.SetCellStr(flowSheetHistory, "D"+itoa(row), r.DisplayNo); err != nil {
				return err
			}
			last, _ := excelize.CoordinatesToCellName(len(headers), row)
			if err := x.SetCellStyle(flowSheetHistory, cell, last, st.cell); err != nil {
				return err
			}
		}
	}

	if err := x.SetPanes(flowSheetHistory, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 0, YSplit: headerRow, TopLeftCell: "A" + itoa(headerRow+1), ActivePane: "bottomLeft",
	}); err != nil {
		return err
	}
	lastDataRow := headerRow + len(data.History)
	if lastDataRow < headerRow {
		lastDataRow = headerRow
	}
	lastCol, _ := excelize.ColumnNumberToName(len(headers))
	ref := fmt.Sprintf("A%d:%s%d", headerRow, lastCol, lastDataRow)
	return x.AutoFilter(flowSheetHistory, ref, []excelize.AutoFilterOptions{})
}

// ---------- Sheet3 统计汇总 ----------

func buildSummarySheet(x *excelize.File, st flowSty, data *FlowExportData) error {
	if err := setColWidths(x, flowSheetSummary, []float64{20, 12}); err != nil {
		return err
	}

	row := 1
	if err := x.MergeCell(flowSheetSummary, "A1", "B1"); err != nil {
		return err
	}
	if err := x.SetCellValue(flowSheetSummary, "A1", "设备流转情况统计"); err != nil {
		return err
	}
	if err := x.SetCellStyle(flowSheetSummary, "A1", "B1", st.title); err != nil {
		return err
	}
	if err := x.SetRowHeight(flowSheetSummary, 1, 26); err != nil {
		return err
	}
	row++
	if err := x.SetCellStr(flowSheetSummary, "A"+itoa(row), "导出时间："+time.Now().Format(dateTimeLay)); err != nil {
		return err
	}
	row += 2

	// 区块 1：按状态统计
	row = writeSummaryBlock(x, st, flowSheetSummary, row, "按状态统计", []string{"当前状态", "设备数量"}, data.Summary.ByStatus)
	// 区块 2：按班组统计
	row = writeSummaryBlock(x, st, flowSheetSummary, row, "按班组统计", []string{"班组", "设备数量"}, data.Summary.ByTeam)
	// 区块 3：按外借公司统计
	writeSummaryBlock(x, st, flowSheetSummary, row, "按外借公司统计", []string{"外借公司", "当前设备数量"}, data.Summary.ByBorrower)
	return nil
}

// writeSummaryBlock 写一个统计区块，返回下一区块起始行号。
func writeSummaryBlock(x *excelize.File, st flowSty, sheet string, startRow int, title string, headers []string, rows []FlowCount) int {
	row := startRow
	// 区块标题（跨两列，加粗）
	_ = x.MergeCell(sheet, "A"+itoa(row), "B"+itoa(row))
	_ = x.SetCellValue(sheet, "A"+itoa(row), title)
	_ = x.SetCellStyle(sheet, "A"+itoa(row), "B"+itoa(row), st.header)
	row++
	// 表头
	_ = x.SetSheetRow(sheet, "A"+itoa(row), &headers)
	_ = x.SetCellStyle(sheet, "A"+itoa(row), "B"+itoa(row), st.header)
	row++
	// 数据
	for _, c := range rows {
		_ = x.SetCellValue(sheet, "A"+itoa(row), c.Name)
		_ = x.SetCellValue(sheet, "B"+itoa(row), c.Count)
		row++
	}
	return row + 1 // 区块间空一行
}

// itoa 整数转字符串。
func itoa(i int) string { return fmt.Sprintf("%d", i) }
