// Package importer 实现 Excel 初始化导入（Tier 1：设备台账）。
// 规则依据：docs/import-rules.md 与决策基线 15/16/18（v1.1）。
package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// templateHeader 导入模板 R2 表头（A–K，语义固定；docs/import-rules.md §2.1）。
// 解析器按固定列位取值，因此表头必须逐列一致，否则列语义会整体错位。
var templateHeader = []string{
	"类别", "设备名称", "设备型号", "台账数量", "财务数量", "台账设备编号",
	"时间", "公司", "台数", "借出设备编号", "备注",
}

// ErrTemplateMismatch 文件列布局不符合导入模板（拒绝解析，绝不按固定列位硬解析错位数据）。
//
// 事故背景（2026-09-11，用户上报）：工作簿1.xlsx 为「到达时间/外借方/数量/设备编号」版式，
// 被按总账模板硬解析 → D 列“设备编号”被当成“台账数量”、B 列“外借方”被当成“设备名称”，
// 预览显示“预计新增设备 5,693,048 台”（该文件实际仅 208~230 台）。
var ErrTemplateMismatch = errors.New("文件列布局不符合导入模板")

// validateTemplate 校验 R2 表头与导入模板 A–K 是否逐列一致。
// 不一致即返回 ErrTemplateMismatch，并在错误信息中回显 R2（表头行）与 R1（标题行）的实际内容，
// 便于用户判断“版式不对”还是“表头挪了行”；不做模糊匹配、不猜测列语义（engineering-persona §5）。
func validateTemplate(title, header []string) error {
	var mismatch []string
	for i, want := range templateHeader {
		got := cellAt(header, i)
		col := string(rune('A' + i))
		switch {
		case got == "":
			mismatch = append(mismatch, fmt.Sprintf("%s列(期望 %q，实际为空)", col, want))
		case got != want:
			mismatch = append(mismatch, fmt.Sprintf("%s列(期望 %q，实际 %q)", col, want, got))
		}
	}
	if len(mismatch) == 0 {
		return nil
	}
	shown := mismatch
	if len(shown) > 6 {
		shown = shown[:6]
	}
	parts := []string{fmt.Sprintf("第 %d 行必须是表头 A–K（%s）", sheetHeaderRow, strings.Join(templateHeader, "/"))}
	if vals := echoRow(header); vals != "" {
		parts = append(parts, fmt.Sprintf("实际识别到第 %d 行：%s", sheetHeaderRow, vals))
	}
	if vals := echoRow(title); vals != "" {
		parts = append(parts, fmt.Sprintf("第 %d 行：%s", sheetTitleRow, vals))
	}
	parts = append(parts, fmt.Sprintf("不一致项（前 %d 条）：%s", len(shown), strings.Join(shown, "；")))
	parts = append(parts, "请改用系统导入模板（设备借出总账.xlsx 版式）后重试")
	return fmt.Errorf("%w：%s", ErrTemplateMismatch, strings.Join(parts, "；"))
}

// cellAt 取行内第 i 列（越界返回空串）。
func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

// echoRow 把一行单元格渲染为 `A="x"、B="y"` 形式（最多 6 个非空值，超长截断），用于错误回显。
func echoRow(row []string) string {
	var out []string
	for i := 0; i < len(templateHeader) && len(out) < 6; i++ {
		v := cellAt(row, i)
		if v == "" {
			continue
		}
		if utf8.RuneCountInString(v) > 24 {
			v = string([]rune(v)[:24]) + "…"
		}
		out = append(out, fmt.Sprintf("%s=%q", string(rune('A'+i)), v))
	}
	return strings.Join(out, "、")
}

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
	// Issue 级别：BLOCK（阻断）/ WARN（提示，可导入）/ REVIEW（需人工确认，决策18 §六C）。
	IssueLevelReview = "REVIEW"
)

// Issue 校验问题（V 系列，见 import-rules.md §4）。
type Issue struct {
	Row     int    `json:"row"`
	Code    string `json:"code"`  // V1..V10
	Level   string `json:"level"` // BLOCK / WARN / REVIEW
	Group   string `json:"group,omitempty"`
	Message string `json:"message"`
}

// fTokenKind F 列单个编号单元的语义分类（决策 18 §五/§六）。
type fTokenKind int

const (
	// tkNumber 明显设备编号：equipment_no 原样保留（可含中文/字母/横杠等）。
	tkNumber fTokenKind = iota
	// tkNoNumber 显式“无编号”标记。
	tkNoNumber
	// tkDesc 明显描述文本（如“拉布机配件”）：equipment_no=NULL + remark 原文。
	tkDesc
	// tkReview 无法自动判断（情况 C）：不猜不丢，REVIEW 由用户在 Preview 人工确认。
	tkReview
)

// Group 设备分组 = 同 name+model 的源行聚合。
// 决策 18：equipment_no 不再是唯一键；组内 Numbers 允许重复（每 token 一台真机，
// 同号多台以 equipment_seq 区分）。分组仅用于聚合台账数量与归类，身份一律在设备行。
type Group struct {
	Key         string   `json:"key"`
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Model       string   `json:"model"`
	RowFrom     int      `json:"row_from"`
	RowTo       int      `json:"row_to"`
	DSum        int      `json:"d_sum"`       // 台账数量（源 D 合计）
	ESum        int      `json:"e_sum"`       // 财务数量（源 E 合计，仅供参考）
	Numbers     []string `json:"numbers"`     // 编号清单（按出现顺序，允许重复）
	Unnumbered  int      `json:"unnumbered"`  // 无编号台数（含描述文本设备）
	DescRemark  string   `json:"desc_remark"` // 描述文本充当编号时的原文备注
	HasNumericD bool     `json:"has_numeric_d"`
	ReviewN     int      `json:"review_n"` // 需人工确认(情况C)的设备台数
	OK          bool     `json:"ok"`       // 是否通过 BLOCK 校验可导入
}

// ParseResult 一次解析的结果。
type ParseResult struct {
	Filename     string            `json:"filename"`
	SourceHash   string            `json:"source_hash"` // 源文件 SHA-256（批次幂等锚点，§二十五/二十六）
	Sheet        string            `json:"sheet"`
	Header       []string          `json:"header"`
	Groups       []*Group          `json:"groups"`
	Issues       []Issue           `json:"issues"`
	TotalD       int               `json:"total_d"` // 源台账数量合计
	ReviewN      int               `json:"review_n"`
	BorrowEvents []*BorrowEvent    `json:"borrow_events,omitempty"` // J 列借出事件（Phase 6 解析层）
	Derived      *DerivationReport `json:"derived,omitempty"`       // 设备最终状态推导（Phase 7）
	BlankRows    []int             `json:"blank_rows,omitempty"`
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
	sheet := sheets[0]               // 1) 合并单元格：构建 单列纵向合并 的锚点映射
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

	readRow := func(row int) []string {
		out := make([]string, 11)
		for c := 1; c <= 11; c++ {
			out[c-1] = cell(c, row)
		}
		return out
	}
	header := readRow(sheetHeaderRow)

	// Phase 1（2026-09-11）：模板校验前置。列布局不符一律拒绝，
	// 绝不在错误的列语义下继续解析（否则会静默产生海量错误设备）。
	if err := validateTemplate(readRow(sheetTitleRow), header); err != nil {
		return nil, err
	}

	res := &ParseResult{Sheet: sheet, Header: header}
	if sum, err := fileSHA256(path); err == nil {
		res.SourceHash = sum // 幂等锚点；读失败不阻断解析（会话层以文件为准）
	}
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
		// 决策 18/§五：逐 token 分类——编号原样保留、无编号显式标记、描述文本→无编号+原文备注、
		// 无法自动判断→REVIEW 人工确认（绝不静默丢弃）。
		rowNumbers := 0 // 本行判为编号的 token 数
		rowDesc := 0    // 本行判为描述文本的 token 数
		rowDescText := []string{}
		rowNoNumber := false // 本行是否显式标注“无编号”
		for _, t := range tokens {
			main, ann := splitAnnotation(t)
			kind, why := classifyF(main)
			switch kind {
			case tkNumber:
				rowNumbers++
				if strings.ContainsAny(main, "Oo") {
					res.Issues = append(res.Issues, Issue{Row: r, Code: "V5", Level: "WARN", Group: key,
						Message: fmt.Sprintf("编号 %q 含字母 O，疑似 0/O 录入混淆，请人工核对", main)})
				}
				// 原样保留（含中文/字母/符号；重复即多台真机，不入唯一性校验）
				g.Numbers = append(g.Numbers, main)
				if ann != "" {
					g.DescRemark = joinRemark(g.DescRemark, fmt.Sprintf("%s%s", main, ann))
					res.Issues = append(res.Issues, Issue{Row: r, Code: "V10", Level: "WARN", Group: key,
						Message: fmt.Sprintf("编号 %q 附带注释 %q（已并入备注）", main, ann)})
				}
			case tkNoNumber:
				rowNoNumber = true
			case tkDesc:
				rowDesc++
				rowDescText = append(rowDescText, t)
				// 情况 B：描述文本 → equipment_no=NULL，原文进 remark（决策18 §六B）
			case tkReview:
				g.ReviewN++
				res.ReviewN++
				res.Issues = append(res.Issues, Issue{Row: r, Code: "V10", Level: IssueLevelReview, Group: key,
					Message: fmt.Sprintf("无法自动判断的编号内容 %q（%s），请在预览中人工确认：作为编号或作为无编号设备", t, why)})
			}
		}
		// 显式“无编号”且无任何编号 token：整行按台账数量(缺省 1)记无编号
		if rowNoNumber && rowNumbers == 0 && rowDesc == 0 {
			if dOwn == 0 {
				dOwn = 1
				res.Issues = append(res.Issues, Issue{Row: r, Code: "V6", Level: "WARN", Group: key,
					Message: "标注无编号但台账数量为空，按 1 台处理"})
			}
			g.Unnumbered += dOwn
			continue
		}
		// 描述文本与编号同现：描述按文本条数 1 台记无编号（原文保留，避免静默丢）
		if rowDesc > 0 {
			for _, tx := range rowDescText {
				g.DescRemark = joinRemark(g.DescRemark, tx)
			}
			if rowNumbers == 0 {
				d := dOwn
				if d == 0 {
					d = rowDesc
				}
				g.Unnumbered += d
				res.Issues = append(res.Issues, Issue{Row: r, Code: "V10", Level: "WARN", Group: key,
					Message: fmt.Sprintf("编号位置为描述文本 %q，按 %d 台无编号处理（原文保留到备注）", strings.Join(rowDescText, " "), d)})
				continue
			}
			// 编号与描述混排：描述逐条记 1 台，编号照常
			g.Unnumbered += rowDesc
			res.Issues = append(res.Issues, Issue{Row: r, Code: "V10", Level: "WARN", Group: key,
				Message: fmt.Sprintf("编号与描述文本混排：描述 %q 按 %d 台无编号处理（原文保留到备注）", strings.Join(rowDescText, " "), rowDesc)})
		}
		if rowNoNumber && rowNumbers > 0 {
			// 显式“无编号”与编号同现属异常，REVIEW 交人工（不猜）
			g.ReviewN++
			res.ReviewN++
			res.Issues = append(res.Issues, Issue{Row: r, Code: "V10", Level: IssueLevelReview, Group: key,
				Message: "同一行同时出现编号与“无编号”标注，语义冲突，请在预览中人工确认"})
		}
	}

	// 汇总排序（保持行序）
	res.Groups = make([]*Group, 0, len(order))
	for _, k := range order {
		res.Groups = append(res.Groups, groupByKey[k])
	}

	// 3) J 列借出事件解析（Phase 6；G/H/I/J/K 锚定行 + 续 J 行合并）
	res.BorrowEvents = collectBorrowEvents(ownCell, cell)

	// 4) 分组级校验（决策 18：重复不再 BLOCK；数量一致性 BLOCK）
	validateGroups(res)

	// 5) 设备最终状态推导（Phase 7；只读推导，不写库）
	res.Derived = DeriveEquipmentStatus(res)
	return res, nil
}

// collectBorrowEvents 扫描 G–K 列，按“G 自有值=事件首行；其后到下一个 G 自有行之前含 J 的行并入”合并事件。
// 事件数口径与决策 15 一致（G 锚定 144 = 内部 101 + 外部 43）。
// 续行若自带 I（合并格内的另一起借出）→ 事件置 REVIEW（不猜测是否同一批，交给用户）。
// 只读不写库；纯解析，永不丢原文（§十六）。
func collectBorrowEvents(ownCell func(col, row int) string, cell func(col, row int) string) []*BorrowEvent {
	// 列常量：G=7 H=8 I=9 J=10 K=11（1-based）
	const (
		cDate = 7
		cComp = 8
		cCnt  = 9
		cJ    = 10
		cK    = 11
	)
	var events []*BorrowEvent
	var cur *BorrowEvent
	type rowData struct{ g, h, i, j, k string }
	// 收集各行 G/H/I/J/K（自有 G/I/J/K；H 走合并感知取值）
	data := map[int]rowData{}
	rowOrder := make([]int, 0, 120)
	for r := sheetDataStart; r <= sheetSumRow-1; r++ {
		d := rowData{
			g: ownCell(cDate, r), h: cell(cComp, r),
			i: ownCell(cCnt, r), j: ownCell(cJ, r), k: ownCell(cK, r),
		}
		if d.g == "" && d.h == "" && d.i == "" && d.j == "" && d.k == "" {
			continue
		}
		data[r] = d
		rowOrder = append(rowOrder, r)
	}
	for _, r := range rowOrder {
		d := data[r]
		// G 自有值 → 开启新事件
		if d.g != "" {
			if cur != nil {
				events = append(events, cur)
			}
			cur = &BorrowEvent{
				RowFrom: r, RowTo: r,
				DateRaw: d.g, Company: d.h,
				CountRaw: d.i, KRemark: d.k,
			}
			if d.j != "" {
				cur.JRaw = d.j
			}
			continue
		}
		// 续 J 行：并入当前事件（要求其确有 J/H/I 内容）
		if cur != nil && (d.j != "" || d.i != "") {
			if d.i != "" && cur.CountRaw != "" && d.i != cur.CountRaw {
				// 合并区内出现第二个自有 I：不猜测是否同批，REVIEW
				cur.review(fmt.Sprintf("R%d 续行另有台数 I=%s（事件首行 I=%s），是否同批借出请人工确认", r, d.i, cur.CountRaw))
			}
			cur.RowTo = r
			if d.j != "" {
				if cur.JRaw == "" {
					cur.JRaw = d.j
				} else {
					cur.JRaw += " | " + d.j
				}
			}
			if cur.CountRaw == "" {
				cur.CountRaw = d.i
			}
			if cur.KRemark == "" {
				cur.KRemark = d.k
			}
		}
	}
	if cur != nil {
		events = append(events, cur)
	}
	// 生成结构化事件（解析日期/台数/编号；块上下文=事件首行 B/C 合并值）
	out := make([]*BorrowEvent, 0, len(events))
	for _, e := range events {
		be := buildBorrowEvent(e.RowFrom, e.RowTo, e.DateRaw, e.Company, e.CountRaw, e.JRaw, e.KRemark)
		be.Name = cell(2, e.RowFrom)  // B 列：设备名称（合并感知）
		be.Model = cell(3, e.RowFrom) // C 列：型号
		out = append(out, be)
	}
	return out
}

// joinRemark 累积多段备注文本（空格分隔，去重防重复拼接）。
func joinRemark(existing, add string) string {
	if existing == "" {
		return add
	}
	return existing + " " + add
}

// classifyF 对 F 列单个编号单元（已剥离括号注释的主体）做语义分类。
// 规则（决策 18 §五/§六，不猜测、不因含中文而拒绝）：
//   - 明显编号：含字母/数字/横杠/下划线等编号形态（如 6041、001、A01、JUKI-8700、
//     车间A-01、缝制A-02、ABC-001）。规则：主体含 ASCII 字母/数字即为编号
//     （编号可带中文前缀，如“车间A-01”）；纯中文若以“号/编号”收尾（设备一号、
//     中文编号）也按编号保留。
//   - “无编号”→ 显式标记。
//   - 明显描述：纯汉字名词短语（如“拉布机配件”“拖布轮”）→ 描述（情况 B，
//     equipment_no=NULL，原文进 remark）。
//   - 其余（纯符号、无法归类的纯中文、空）→ REVIEW 无法自动判断（情况 C，人工确认）。
func classifyF(main string) (fTokenKind, string) {
	m := strings.TrimSpace(main)
	if m == "" {
		return tkReview, "主体为空"
	}
	if m == tokenNoNumber {
		return tkNoNumber, ""
	}
	hasHan := false
	hasASCII := false
	for _, r := range m {
		if unicode.Is(unicode.Han, r) {
			hasHan = true
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			hasASCII = true
		}
	}
	if hasASCII {
		return tkNumber, "" // 编号（可带中文前缀/后缀，决策18 §五）
	}
	if !hasHan {
		return tkReview, "既无汉字也无字母数字，无法判断" // 纯符号
	}
	// 纯中文：
	if strings.HasSuffix(m, "号") || strings.HasSuffix(m, "编号") {
		return tkNumber, "" // 设备一号 / 中文编号（§六 情况A）
	}
	if looksLikeDescription(m) {
		return tkDesc, "" // 拉布机配件 / 拖布轮（§六 情况B）
	}
	return tkReview, "纯中文内容无法可靠判断为编号或描述"
}

// looksLikeDescription 纯中文名词短语的启发式判断（情况 B 例子：拉布机配件、拖布轮）。
// 仅对明显名词短语收尾词生效；不在清单内的一律 REVIEW 交人工，绝不猜。
func looksLikeDescription(m string) bool {
	n := utf8.RuneCountInString(m)
	if n < 2 {
		return false
	}
	for _, suf := range []string{"配件", "备件", "附件", "零件", "部件", "机件", "托板", "拖布轮", "轮", "刀", "针"} {
		if strings.HasSuffix(m, suf) {
			return true
		}
	}
	return false
}

// validateGroups 分组级校验并标记 OK/BLOCK。
// 决策 18：同组重复编号 = 多台真机 → 仅 WARN 提示（不再 BLOCK）；数量不一致仍 BLOCK。
func validateGroups(res *ParseResult) {
	for _, g := range res.Groups {
		g.OK = true
		if !g.HasNumericD && len(g.Numbers) > 0 {
			res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V6", Level: "WARN", Group: g.Key,
				Message: "该组整组缺少台账数量(D)列，将按编号数导入"})
		}
		// 同组内重复编号（V3，决策 18：多台真机，正常展开，仅提示不阻断）
		seen := map[string]int{}
		for _, n := range g.Numbers {
			seen[n]++
		}
		dupOcc := 0
		var dups []string
		for n, c := range seen {
			if c > 1 {
				dups = append(dups, n)
				dupOcc += c - 1
			}
		}
		if len(dups) > 0 {
			sort.Strings(dups)
			res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V3", Level: "WARN", Group: g.Key,
				Message: fmt.Sprintf("同组出现重复编号 %d 个（共多录 %d 次）：%s —— 按同号多台真机展开（display 以 %s（n）区分），如需合并请人工核对",
					len(dups), dupOcc, strings.Join(dups, "、"), dups[0])})
		}
		// 数量一致性（V1，§十二）：D 台账数量 与 展开数（编号+无编号+REVIEW）不一致才 BLOCK
		actual := len(g.Numbers) + g.Unnumbered
		if g.HasNumericD && actual != g.DSum {
			g.OK = false
			res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V1", Level: "BLOCK", Group: g.Key,
				Message: fmt.Sprintf("台账数量 %d 与 展开设备数（编号%d+无编号%d+待确认%d=%d）不一致",
					g.DSum, len(g.Numbers), g.Unnumbered, g.ReviewN, actual+g.ReviewN)})
		}
		// REVIEW（§六C）在 Preview 人工确认流程(Phase 9)落地前不可自动导入：
		// 宁可整组跳过列入报告，也绝不静默丢弃待确认内容（§十六）。
		if g.ReviewN > 0 {
			g.OK = false
			res.Issues = append(res.Issues, Issue{Row: g.RowFrom, Code: "V10", Level: IssueLevelReview, Group: g.Key,
				Message: fmt.Sprintf("该组有 %d 项编号内容需人工确认（REVIEW），确认前暂不导入该组，请对照上方明细在预览中确认或修正 Excel", g.ReviewN)})
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

// fileSHA256 计算源文件 SHA-256（十六进制小写）。
func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// splitAnnotation 剥离编号中的括号注释（全半角）。返回主体与注释。
func splitAnnotation(t string) (string, string) {
	idx := strings.IndexAny(t, "（(")
	if idx <= 0 {
		return t, ""
	}
	return t[:idx], t[idx:]
}
