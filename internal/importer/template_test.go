package importer

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Phase 1（2026-09-11）：导入模板校验测试。
// 目标：列布局不符合导入模板的文件必须被拒绝解析（ErrTemplateMismatch），
// 绝不允许按固定列位硬解析出错位数据。

// writeXLSXAt 按“坐标=值”写入稀疏单元格，用于构造非模板版式的测试文件。
func writeXLSXAt(t *testing.T, cells map[string]string) string {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Sheet1"
	for axis, v := range cells {
		if err := f.SetCellStr(sheet, axis, v); err != nil {
			t.Fatalf("写入 %s 失败: %v", axis, v)
		}
	}
	path := filepath.Join(t.TempDir(), "custom.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestParseRejectsWrongTemplate 复现事故场景：工作簿1.xlsx 版式
// （到达时间/外借方/数量/设备编号）被当作总账模板解析。
func TestParseRejectsWrongTemplate(t *testing.T) {
	path := writeXLSXAt(t, map[string]string{
		"A1": "到达时间", "B1": "外借方", "C1": "数量", "D1": "设备编号",
		"A2": "2012.12.22", "B2": "双发分厂（龙山贸易）", "C2": "10",
		"D2": "11221 11307 12583 11310 21163 11545",
		"A4": "2023.3.6", "B4": "双发分厂（华锦）", "C4": "1", "D4": "1507034",
	})
	res, err := Parse(path)
	if err == nil {
		t.Fatalf("列布局不符的模板必须拒绝解析，实际解析出 %d 个分组、台账数量合计 %d", len(res.Groups), res.TotalD)
	}
	if !errors.Is(err, ErrTemplateMismatch) {
		t.Fatalf("错误应为 ErrTemplateMismatch，实际 %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"不符合导入模板", "到达时间", "设备名称", "台账数量"} {
		if !strings.Contains(msg, want) {
			t.Errorf("错误信息应回显实际表头/期望列，缺少 %q：%s", want, msg)
		}
	}
}

// TestParseRejectsSingleColumnRenamed 任一列文案不符也必须拒绝（含 F 列台账设备编号）。
func TestParseRejectsSingleColumnRenamed(t *testing.T) {
	headers := []string{"类别", "设备名称", "设备型号", "台账数量", "财务数量", "设备编号", "时间", "公司", "台数", "借出设备编号", "备注"}
	path := writeMiniXLSXWithHeaders(t, headers, [][]string{
		{"裁剪设备", "环形割刀", "EBK-SA", "1", "1", "6040"},
	})
	_, err := Parse(path)
	if !errors.Is(err, ErrTemplateMismatch) {
		t.Fatalf("F 列改名应被拒绝，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "F列") {
		t.Errorf("错误信息应指出 F 列不一致：%s", err.Error())
	}
}

// TestParseRejectsEmptyHeader 表头行为空（纯数据文件）必须拒绝。
func TestParseRejectsEmptyHeader(t *testing.T) {
	path := writeXLSXAt(t, map[string]string{
		"A5": "裁剪设备", "B5": "环形割刀", "C5": "EBK-SA", "D5": "1", "F5": "6040",
	})
	if _, err := Parse(path); !errors.Is(err, ErrTemplateMismatch) {
		t.Fatalf("缺少表头应被拒绝，实际 %v", err)
	}
}

// TestValidateTemplateAcceptsTemplate 模板表头本身必须通过（正例，防误拒）。
func TestValidateTemplateAcceptsTemplate(t *testing.T) {
	if err := validateTemplate(nil, templateHeader); err != nil {
		t.Fatalf("标准模板表头不应被拒绝: %v", err)
	}
	if err := validateTemplate([]string{"青岛双发服装有限公司借出设备明细"}, append(append([]string{}, templateHeader...), "多余列")); err != nil {
		t.Fatalf("标题行与多余尾部列不应影响 A–K 校验: %v", err)
	}
}

// TestParseRealFilePassesTemplateCheck 真实总账文件必须仍可解析（模板校验不误伤既有通道）。
func TestParseRealFilePassesTemplateCheck(t *testing.T) {
	res := parseRealFile(t)
	if res.TotalD == 0 {
		t.Fatal("真实文件解析结果异常：台账数量合计为 0")
	}
}
