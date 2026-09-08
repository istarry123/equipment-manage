// Package importer 实现 Excel 初始化导入（Tier 1：设备台账）。
// 规则依据：docs/import-rules.md 与决策基线 15/16。
package importer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// 列常量（0-based 对应 A..K）。
const (
	colCategory = 0  // A
	colName     = 1  // B
	colModel    = 2  // C
	colD        = 3  // D 台账数量
	colE        = 4  // E 财务数量
	colF        = 5  // F 台账设备编号
	colG        = 6  // G 时间
	colH        = 7  // H 公司
	colI        = 8  // I 台数
	colJ        = 9  // J 借出设备编号
	colK        = 10 // K 备注
)

const (
	sheetTitleRow    = 1
	sheetHeaderRow   = 2
	sheetDataStart   = 5
	sheetSumRow      = 467
	categoryOther    = "其他"
	tokenNoNumber    = "无编号"
	operatorImport   = "系统导入"
	statusInStock    = "IN_STOCK"
	actionImportInit = "IMPORT_INIT"
)

// Issue 校验问题（V 系列，见 import-rules.md §4）。
type Issue struct {
	Row     int    `json:"row"`
	Code    string `json:"code"`  // V1..V10
	Level   string `json:"level"` // BLOCK / WARN
	Group   string `json:"group,omitempty"`
	Message string `json:"message"`
}

// Group 设备分组 = 同 name+model（编号唯一粒度，决策 16）。
type Group struct {
	Key         string   `json:"key"`
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Model       string   `json:"model"`
	RowFrom     int      `json:"row_from"`
	RowTo       int      `json:"row_to"`
	DSum        int      `json:"d_sum"`       // 台账数量（源 D 合计）
	ESum        int      `json:"e_sum"`       // 财务数量（源 E 合计，仅供参考）
	Numbers     []string `json:"numbers"`     // 编号清单（按出现顺序）
	Unnumbered  int      `json:"unnumbered"`  // 无编号台数
	DescRemark  string   `json:"desc_remark"` // 描述文本充当编号时的原文备注
	HasNumericD bool     `json:"has_numeric_d"`
	OK          bool     `json:"ok"` // 是否通过 BLOCK 校验可导入
}

// ParseResult 一次解析的结果。
type ParseResult struct {
	Filename  string   `json:"filename"`
	Sheet     string   `json:"sheet"`
	Header    []string `json:"header"`
	Groups    []*Group `json:"groups"`
	Issues    []Issue  `json:"issues"`
	TotalD    int      `json:"total_d"` // 源台账数量合计
	BlankRows []int    `json:"blank_rows,omitempty"`
}

// block 表示按 B（名称）锚点划分的连续行区间（用于跨行回填 name/model 的行归属）。
type rowCol struct {
	col int
	row int
}

// Parse 只读解析 xlsx（不修改原文件），产出分组与问题清单。
func Parse(path string) (*ParseResult, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("打开 Excel 失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("Excel 中没有工作表")
	}
	sheet := sheets[0]

	// 1) 合并单元格：构建 单列纵向合并 的锚点映射
	anchors := map[int]map[int]int{} // col -> (row -> anchorRow)
	merges, err := f.GetMergeCells(sheet)
	if err != nil {
		return nil, fmt.Errorf("读取合并单元格失败: %w", err)
	}
	for _, mc := range merges {
		s1, r1, err := excelize.CellNameToCoordinates(mc.GetStartAxis())
		if err != nil {
			continue
		}
		s2, r2, err := excelize.CellNameToCoordinates(mc.GetEndAxis())
		if err != nil {
			continue
		}
		if s1 == s2 && r2 > r1 && r2 <= sheetSumRow+3 {
			if anchors[s1] == nil {
				anchors[s1] = map[int]int{}
			}
			for r := r1 + 1; r <= r2; r++ {
				anchors[s1][r] = r1
			}
		}
	}

	// 2) 读取单元格
	//   - cell：合并感知（类别/名称/型号等描述字段取锚点行值）
	//   - ownCell：真实自有单元格（数量 D/E 与编号 F 列：合并继承不算，避免重复统计）
	cell := func(col, row int) string {
		r := row
		if a, ok := anchors[col][row]; ok {
			r = a
		}
		if r < 1 || r > sheetSumRow {
			return ""
		}
		name, _ := excelize.CoordinatesToCellName(col, r)
		v, _ := f.GetCellValue(sheet, name)
		return strings.TrimSpace(v)
	}
	ownCell := func(col, row int) string {
		if row < 1 || row > sheetSumRow {
			return ""
		}
		// excelize 会把合并锚点值回填到整个区间：仅当本行是"自有单元格"
		// （不在任何单列纵向合并的覆盖行上）时才认为是真实填写值。
		if _, covered := anchors[col][row]; covered {
			return ""
		}
		name, _ := excelize.CoordinatesToCellName(col, row)
		v, _ := f.GetCellValue(sheet, name)
		return strings.TrimSpace(v)
	}

	header := make([]string, 11)
	for c := 1; c <= 11; c++ {
		header[c-1] = cell(c, sheetHeaderRow)
	}

	res := &ParseResult{Sheet: sheet, Header: header}
	groupByKey := map[string]*Group{}
	order := []string{}

	rowRangeEnd := sheetSumRow - 1 // 数据到 466 行；467 为合计
	for r := sheetDataStart; r <= rowRangeEnd; r++ {
		cat := cell(colCategory+1, r)
		name := cell(colName+1, r)
		model := cell(colModel+1, r)
		dStr := ownCell(colD+1, r)
		eStr := ownCell(colE+1, r)
		fStr := ownCell(colF+1, r)

		isBlank := name == "" && model == "" && dStr == "" && eStr == "" && fStr == "" && cat == ""
		if isBlank {
			continue
		}
		if cat == "合计" || name == "合计" {
			continue
		}

		// 台账数量列（仅本行真实单元格，合并继承的不算）
		dOwn, _ := strconv.Atoi(dStr)
		eOwn, _ := strconv.Atoi(eStr)
		if dOwn > 0 {
			res.TotalD += dOwn
		}

		// 名称/型号为空的行（借出续行、纯借出行等）不产生台账实体
		if name == "" && fStr == "" && dOwn == 0 {
			continue
		}
		if name == "" {
			// B 合并继承为空仍可能来自异常数据；若 D 或 F 存在但无名，生成占位名避免静默丢数据
			name = "(未命名)"
		}

		key := name + "\x00" + model
		g := groupByKey[key]
		if g == nil {
			g = &Group{Key: key, Category: cat, Name: name, Model: model, RowFrom: r, RowTo: r}
			groupByKey[key] = g
			order = append(order, key)
		}
		g.RowTo = r
		if g.Category == "" {
			g.Category = cat
		} else if cat != "" && cat != g.Category {
			// 同名同型号跨类别：仅提示
			res.Issues = append(res.Issues, Issue{Row: r, Code: "V9", Level: "WARN", Group: key,
				Message: fmt.Sprintf("同类目 %q 又出现 %q，以首见类别为准", g.Category, cat)})
		}
		if dOwn > 0 {
			g.DSum += dOwn
			g.HasNumericD = true
		}
		g.ESum += eOwn

		tokens := splitTokens(fStr)
		if len(tokens) == 0 {
			if dOwn > 0 {
				g.Unnumbered += dOwn
				res.Issues = append(res.Issues, Issue{Row: r, Code: "V6", Level: "WARN", Group: key,
					Message: fmt.Sprintf("台账数量 %d 但编号列为空，按无编号处理", dOwn)})
			}
			continue
		}
		if isNoNumberOnly(tokens) {
			if dOwn == 0 {
				dOwn = 1 // 标了无编号却没有数量：按 1 台处理并提示
				res.Issues = append(res.Issues, Issue{Row: r, Code: "V6", Level: "WARN", Group: key,
					Message: "标注无编号但台账数量为空，按 1 台处理"})
			}
			g.Unnumbered += dOwn
			continue
		}
		// 整行均为描述文本（如“拉布机配件”）：按无编号处理，原文入备注（V10 WARN）
		allDesc := true
		for _, t := range tokens {
			m, _ := splitAnnotation(t)
			if !containsCJK(m) {
				allDesc = false
				break
			}
		}
		if allDesc {
			d := dOwn
			if d == 0 {
				d = 1
			}
			g.Unnumbered += d
			g.DescRemark = strings.TrimSpace(g.DescRemark + " " + strings.Join(tokens, " "))
			res.Issues = append(res.Issues, Issue{Row: r, Code: "V10", Level: "WARN", Group: key,
				Message: fmt.Sprintf("编号位置为描述文本 %q，按 %d 台无编号处理（原文保留到备注）", strings.Join(tokens, " "), d)})
			continue
		}
		// 编号行：逐 token 剥离括号注释 / 检出 0-O / 检出混合描述
		for _, t := range tokens {
			main, _ := splitAnnotation(t)
			if containsCJK(main) {
				g.OK = false
				res.Issues = append(res.Issues, Issue{Row: r, Code: "V10", Level: "BLOCK", Group: key,
					Message: fmt.Sprintf("编号与描述混排 %q，需人工修正", t)})
				continue
			}
			if strings.ContainsAny(main, "Oo") {
				res.Issues = append(res.Issues, Issue{Row: r, Code: "V5", Level: "WARN", Group: key,
					Message: fmt.Sprintf("编号 %q 含字母 O，疑似 0/O 录入混淆，请人工核对", main)})
			}
			g.Numbers = append(g.Numbers, main)
		}
	}

	// 汇总排序（保持行序）
	res.Groups = make([]*Group, 0, len(order))
	for _, k := range order {
		res.Groups = append(res.Groups, groupByKey[k])
	}

	// 3) 分组级校验（V1/V3/V4 语义按决策 16：同 name+model 内唯一）
	validateGroups(res)
	return res, nil
}

// validateGroups 分组级校验并标记 OK/BLOCK。
func validateGroups(res *ParseResult) {
	for _, g := range res.Groups {
		g.OK = true
		if !g.HasNumericD && len(g.Numbers) > 0 {
			res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V6", Level: "WARN", Group: g.Key,
				Message: "该组整组缺少台账数量(D)列，将按编号数导入"})
		}
		// 同组内重复编号（V3，决策 16：BLOCK）
		seen := map[string]int{}
		for _, n := range g.Numbers {
			seen[n]++
		}
		var dups []string
		dupOcc := 0
		for n, c := range seen {
			if c > 1 {
				dups = append(dups, n)
				dupOcc += c - 1
			}
		}
		if len(dups) > 0 {
			sort.Strings(dups)
			g.OK = false
			res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V3", Level: "BLOCK", Group: g.Key,
				Message: fmt.Sprintf("同组重复编号 %d 个（多录 %d 次）：%s —— 请人工处理（改号或确认为两台同号机）",
					len(dups), dupOcc, strings.Join(dups, "、"))})
		}
		// 数量一致性（V1）：期望 D 合计 vs 实际 编号数+无编号
		actual := len(g.Numbers) + g.Unnumbered
		if g.HasNumericD && actual != g.DSum {
			// 有重复时已 BLOCK；此处仅对非重复情形提示偏差
			if len(dups) == 0 {
				g.OK = false
				res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V1", Level: "BLOCK", Group: g.Key,
					Message: fmt.Sprintf("台账数量 %d 与 实际编号(%d)+无编号(%d)=%d 不一致", g.DSum, len(g.Numbers), g.Unnumbered, actual)})
			}
		}
		// 财务数量一致性（V2，仅提示）
		if g.HasNumericD && g.ESum != 0 && g.DSum != g.ESum {
			res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V2", Level: "WARN", Group: g.Key,
				Message: fmt.Sprintf("台账数量 %d 与 财务数量 %d 不一致（财务口径，仅提示）", g.DSum, g.ESum)})
		}
	}
}

// splitTokens 按任意空白切分编号清单。
func splitTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) })
}

func isNoNumberOnly(tokens []string) bool {
	if len(tokens) == 1 {
		return tokens[0] == tokenNoNumber
	}
	// 允许 '无编号' 与空混排场景仅剩 token
	for _, t := range tokens {
		if strings.TrimSpace(t) != "" {
			return false
		}
	}
	return false
}

// splitAnnotation 剥离编号中的括号注释（全半角）。返回主体与注释。
func splitAnnotation(t string) (string, string) {
	idx := strings.IndexAny(t, "（(")
	if idx <= 0 {
		return t, ""
	}
	return t[:idx], t[idx:]
}

func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
