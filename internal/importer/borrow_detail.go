package importer

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// 外借明细（到达/借出明细）解析 —— v1.3 Phase 2（决策 19）。
//
// 版式（工作簿1.xlsx 实测）：
//
//	R1 表头：A 到达时间 | B 外借方 | C 数量 | D 设备编号
//	可选：E 设备名称 | F 设备型号（决策 19 消歧方案 A 预留，用于同号多台定位）
//	R2 起为数据；A/B/C 为该块的纵向合并值（块 = C“数量”锚定行起，到下一锚定行前）
//
// 与台账模板（parser.go 的 Parse）是两条互相独立的通道：本文件只解析“外借明细”，
// 不建台账、不写库、不改状态（匹配/预览在 Phase 3，写入在 Phase 4）。
//
// 用户口径（决策 19）：② 台数以 C 列为准；③ 无编号跳过；④ D 列括号内容只保留编号、
// 注释（带拖布轮）原文留痕到备注，不建模为型号字段。
const (
	detailHeaderRow = 1
	detailDataStart = 2

	colDetailDate    = 1 // A 到达时间
	colDetailCompany = 2 // B 外借方
	colDetailCount   = 3 // C 数量（块级，权威口径）
	colDetailNo      = 4 // D 设备编号
	colDetailName    = 5 // E 设备名称（可选）
	colDetailModel   = 6 // F 设备型号（可选）
)

// detailHeader 外借明细模板必需列（A–D）。
var detailHeader = []string{"到达时间", "外借方", "数量", "设备编号"}

// detailOptionalHeader 可选列（出现则必须同名）。
var detailOptionalHeader = map[int]string{5: "设备名称", 6: "设备型号"}

// 外借明细校验问题编码（V-D 系列）。
const (
	codeDetailCount    = "V-D1" // 数量 C 与展开台数不一致
	codeDetailDate     = "V-D2" // 到达时间缺失/无法解析
	codeDetailCompany  = "V-D3" // 外借方为空
	codeDetailUnnumber = "V-D4" // 无编号台数按用户口径跳过（信息）
	codeDetailParen    = "V-D5" // 括号不配对/结构异常，编号已尽力提取
	codeDetailOrphan   = "V-D7" // D 列有内容但没有“数量”锚定的块
	codeDetailMixed    = "V-D8" // 同块内编号与“无编号”混排
	codeDetailTotalRow = "V-D9" // 表末合计行与 C 列实际合计不一致
)

// ErrDetailTemplateMismatch 文件列布局不符合外借明细模板。
var ErrDetailTemplateMismatch = errors.New("文件列布局不符合外借明细模板")

// DetailDevice 明细中一台待补录外借的设备（仅编号可定位的设备入此列；无编号已按口径跳过）。
type DetailDevice struct {
	SourceKey   string `json:"source_key"` // R{from}-R{to}#N{i}（Phase 4 幂等锚点）
	Row         int    `json:"row"`        // 该编号所在源行
	EquipmentNo string `json:"equipment_no"`
	Remark      string `json:"remark,omitempty"` // 括号注释原文（如“带拖布轮”）
	Raw         string `json:"raw"`              // 原始片段（永不丢）
	BlockRow    int    `json:"block_row"`
	BlockRows   string `json:"block_rows"`
}

// BorrowDetailBlock 一个外借明细块（到达时间 + 外借方 + 数量 C）。
type BorrowDetailBlock struct {
	Row        int             `json:"row"`
	RowTo      int             `json:"row_to"`
	BlockRows  string          `json:"block_rows"` // R{from}-R{to}
	ArriveRaw  string          `json:"arrive_raw"`
	ArriveAt   *time.Time      `json:"arrive_at,omitempty"`
	DateReview bool            `json:"date_review"`
	Company    string          `json:"company"`
	Name       string          `json:"name,omitempty"`  // 可选列 E（块级）
	Model      string          `json:"model,omitempty"` // 可选列 F（块级）
	Count      int             `json:"count"`           // C 数量（权威口径）
	Numbers    []string        `json:"numbers"`         // 展开的编号（每 token 一台）
	Devices    []*DetailDevice `json:"devices"`
	RawLines   []string        `json:"raw_lines"`  // D 列原文（逐行，Phase 4 备注留痕用）
	Unnumbered int             `json:"unnumbered"` // 无编号台数（本阶段跳过）
	Remark     string          `json:"remark,omitempty"`
	OK         bool            `json:"ok"` // 结构自洽且可进入 Phase 3 匹配

	unnumberedTok int // 内部：块内“无编号”标记出现次数
}

// DetailParseResult 一次外借明细解析的结果。
type DetailParseResult struct {
	Filename      string               `json:"filename"`
	SourceHash    string               `json:"source_hash"` // 源文件 SHA-256（Phase 4 批次幂等）
	Sheet         string               `json:"sheet"`
	Header        []string             `json:"header"`
	Blocks        []*BorrowDetailBlock `json:"blocks"`
	Devices       []*DetailDevice      `json:"devices"` // 仅可定位编号设备（无编号已跳过）
	TotalCount    int                  `json:"total_count"`
	NumberedCount int                  `json:"numbered_count"`
	SkippedUnnum  int                  `json:"skipped_unnumbered"` // 按口径③跳过
	TotalRowRaw   string               `json:"total_row_raw,omitempty"`
	Issues        []Issue              `json:"issues"`

	// 文件级聚合（供 Phase 3 直接使用）
	Companies  []string `json:"companies"`  // 出现的外借方（按首见顺序）
	Duplicates []string `json:"duplicates"` // 文件内重复出现的编号（同号多台/多次借出提示）
}

// ParseBorrowDetail 只读解析外借明细 xlsx（不修改原文件、不写库）。
func ParseBorrowDetail(path string) (*DetailParseResult, error) {
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

	// 行数上限取真实数据行数（不硬编码固定行号，避免台账模板的 467 行假设外溢）
	allRows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("读取工作表失败: %w", err)
	}
	maxRow := len(allRows)

	// 合并单元格：仅单列纵向合并（A/B/C 块级字段）建锚点映射
	anchors := map[int]map[int]int{}
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
		if s1 == s2 && r2 > r1 && r2 <= maxRow {
			if anchors[s1] == nil {
				anchors[s1] = map[int]int{}
			}
			for r := r1 + 1; r <= r2; r++ {
				anchors[s1][r] = r1
			}
		}
	}

	cell := func(col, row int) string { // 合并感知（块级字段取锚点行值）
		r := row
		if a, ok := anchors[col][row]; ok {
			r = a
		}
		if r < 1 || r > maxRow {
			return ""
		}
		name, _ := excelize.CoordinatesToCellName(col, r)
		v, _ := f.GetCellValue(sheet, name)
		return strings.TrimSpace(v)
	}
	ownCell := func(col, row int) string { // 真实自有单元格（合并继承不算，避免重复计数）
		if row < 1 || row > maxRow {
			return ""
		}
		if _, covered := anchors[col][row]; covered {
			return ""
		}
		name, _ := excelize.CoordinatesToCellName(col, row)
		v, _ := f.GetCellValue(sheet, name)
		return strings.TrimSpace(v)
	}

	header := make([]string, 6)
	for c := 1; c <= 6; c++ {
		header[c-1] = cell(c, detailHeaderRow)
	}
	if err := validateDetailHeader(header); err != nil {
		// 反向提示：误传台账版式 → 引导到台账通道（台账表头在 R2，R1 为公司标题）
		if isLedgerLayout(cell, sheetHeaderRow) {
			return nil, fmt.Errorf("%w；该文件是「设备借出总账（台账）」版式，请改用设备台账导入通道", err)
		}
		return nil, err
	}

	res := &DetailParseResult{Sheet: sheet, Header: header}
	if sum, err := fileSHA256(path); err == nil {
		res.SourceHash = sum
	}

	var cur *BorrowDetailBlock
	blocks := []*BorrowDetailBlock{}
	companySeen := map[string]bool{}

	finalize := func() {
		if cur == nil {
			return
		}
		cur.BlockRows = fmt.Sprintf("R%d-R%d", cur.Row, cur.RowTo)
		cur.fixSourceKeys()
		blocks = append(blocks, cur)
		cur = nil
	}
	startBlock := func(row int, dateRaw, company, name, model, countRaw string) {
		cur = &BorrowDetailBlock{
			Row: row, RowTo: row, ArriveRaw: dateRaw, Company: company,
			Name: name, Model: model, OK: true,
		}
		if company != "" && !companySeen[company] {
			companySeen[company] = true
			res.Companies = append(res.Companies, company)
		}
		if n, err := strconv.Atoi(strings.TrimSpace(countRaw)); err == nil && n > 0 {
			cur.Count = n
		} else if countRaw != "" {
			cur.OK = false
			res.Issues = append(res.Issues, Issue{Row: row, Code: codeDetailCount, Level: "BLOCK",
				Message: fmt.Sprintf("数量(C)原文 %q 不是有效台数，无法确定该块台数", countRaw)})
		}
	}

	for r := detailDataStart; r <= maxRow; r++ {
		a := cell(colDetailDate, r)
		b := cell(colDetailCompany, r)
		cOwn := ownCell(colDetailCount, r)
		dOwn := ownCell(colDetailNo, r)

		if a == "" && b == "" && cOwn == "" && dOwn == "" {
			continue // 空行
		}
		// 表末合计行：只有数量、无到达时间/外借方/编号
		if cOwn != "" && dOwn == "" && a == "" && b == "" {
			if res.TotalRowRaw == "" {
				res.TotalRowRaw = strings.TrimSpace(cOwn)
			}
			continue
		}
		switch {
		case cOwn != "":
			finalize()
			startBlock(r, a, b, cell(colDetailName, r), cell(colDetailModel, r), cOwn)
		case cur == nil:
			if dOwn == "" {
				continue // 只有日期/外借方、无数量也无编号的行：无数据可归属
			}
			// D 有内容却没有任何“数量”锚定块 → 不猜归属
			res.Issues = append(res.Issues, Issue{Row: r, Code: codeDetailOrphan, Level: "BLOCK",
				Message: fmt.Sprintf("第 %d 行有编号内容但没有对应的“数量”块（缺少数量列），无法确定台数归属，已跳过", r)})
			continue
		case dOwn != "" && (a != cur.ArriveRaw || b != cur.Company) && (a != "" || b != ""):
			// 自有到达时间/外借方但缺数量：按新块处理并阻断（不并入上一块）
			finalize()
			startBlock(r, a, b, cell(colDetailName, r), cell(colDetailModel, r), "")
		}
		if cur == nil || dOwn == "" {
			continue
		}
		cur.RowTo = r
		cur.RawLines = append(cur.RawLines, dOwn) // 原文留痕（Phase 4 备注用）
		// 逐 token 解析（复用 J 列解析器：处理括号注释/无编号/编号形态；原文永不丢）
		for _, token := range splitBorrowWords(dOwn) {
			for _, bn := range parseJCell(token) {
				if bn.NeedsReview {
					res.Issues = append(res.Issues, Issue{Row: r, Code: codeDetailParen, Level: "WARN",
						Message: fmt.Sprintf("编号内容 %q：%s（已尽力提取，块级台数校验通过即按提取结果处理）", bn.Raw, bn.ReviewWhy)})
				}
				if bn.Unnumbered {
					cur.unnumberedTok++
					continue
				}
				if bn.No == "" { // 无前置编号的孤立注释
					cur.Remark = joinBorrowRemark(cur.Remark, bn.Raw)
					continue
				}
				dev := &DetailDevice{
					Row: r, EquipmentNo: bn.No, Remark: bn.Remark, Raw: bn.Raw,
					BlockRow: cur.Row, BlockRows: fmt.Sprintf("R%d-R%d", cur.Row, cur.RowTo),
				}
				cur.Numbers = append(cur.Numbers, bn.No)
				cur.Devices = append(cur.Devices, dev)
				if bn.Remark != "" {
					cur.Remark = joinBorrowRemark(cur.Remark, fmt.Sprintf("%s%s", bn.No, bn.Remark))
				}
			}
		}
	}
	finalize()

	// 块级校验 + 汇总
	for _, blk := range blocks {
		blk.ArriveAt, blk.DateReview = parseGDate(blk.ArriveRaw)
		switch {
		case blk.ArriveRaw == "":
			blk.OK = false
			res.Issues = append(res.Issues, Issue{Row: blk.Row, Code: codeDetailDate, Level: "BLOCK",
				Message: "缺少到达时间，无法确定外借日期"})
		case blk.DateReview || blk.ArriveAt == nil:
			blk.OK = false
			res.Issues = append(res.Issues, Issue{Row: blk.Row, Code: codeDetailDate, Level: "BLOCK",
				Message: fmt.Sprintf("到达时间原文 %q 无法解析为完整日期（须 YYYY.M.D）", blk.ArriveRaw)})
		}
		if blk.Company == "" {
			blk.OK = false
			res.Issues = append(res.Issues, Issue{Row: blk.Row, Code: codeDetailCompany, Level: "BLOCK",
				Message: "缺少外借方，无法确定借给谁"})
		}
		numbered := len(blk.Numbers)
		switch {
		case blk.unnumberedTok > 0 && numbered > 0:
			// 编号与“无编号”混排：不猜各自台数，交人工
			blk.OK = false
			res.Issues = append(res.Issues, Issue{Row: blk.Row, Code: codeDetailMixed, Level: IssueLevelReview,
				Message: fmt.Sprintf("同一块内既有编号 %d 台又有“无编号”标注（数量 C=%d），台数归属需人工确认", numbered, blk.Count)})
		case blk.unnumberedTok > 0:
			un := blk.Count - numbered
			if un < 0 {
				un = 0
			}
			blk.Unnumbered = un
			res.SkippedUnnum += un
			res.Issues = append(res.Issues, Issue{Row: blk.Row, Code: codeDetailUnnumber, Level: "WARN",
				Message: fmt.Sprintf("该块 %d 台“无编号”，按既定口径跳过补录（不新建、不外借）", un)})
		case blk.Count > 0 && numbered != blk.Count:
			blk.OK = false
			res.Issues = append(res.Issues, Issue{Row: blk.Row, Code: codeDetailCount, Level: "BLOCK",
				Message: fmt.Sprintf("数量 C=%d 与编号展开台数 %d 不一致（以 C 列为准但无法凭空补齐），请核对源文件", blk.Count, numbered)})
		}
		if blk.Count > 0 {
			res.TotalCount += blk.Count
		}
		res.NumberedCount += numbered
	}
	res.Blocks = blocks
	for _, blk := range blocks {
		res.Devices = append(res.Devices, blk.Devices...)
	}
	// 文件内重复编号（同号多台/多次借出）提示，供 Phase 3 消歧
	seen := map[string]int{}
	for _, d := range res.Devices {
		seen[d.EquipmentNo]++
	}
	for _, d := range res.Devices {
		if seen[d.EquipmentNo] > 1 && !containsStr(res.Duplicates, d.EquipmentNo) {
			res.Duplicates = append(res.Duplicates, d.EquipmentNo)
		}
	}
	if res.TotalRowRaw != "" {
		if n, err := strconv.Atoi(res.TotalRowRaw); err == nil && n != res.TotalCount {
			res.Issues = append(res.Issues, Issue{Row: 0, Code: codeDetailTotalRow, Level: "WARN",
				Message: fmt.Sprintf("表末合计 %d 与数量列(C)实际合计 %d 不一致（以 C 列为准，请在源文件更正合计）", n, res.TotalCount)})
		}
	}
	return res, nil
}

// fixSourceKeys 回填编号设备的来源键（按块内顺序；导入后可与 DB source_key 对账）。
func (b *BorrowDetailBlock) fixSourceKeys() {
	for i, d := range b.Devices {
		d.SourceKey = fmt.Sprintf("%s#N%d", b.BlockRows, i+1)
		d.BlockRows = b.BlockRows
	}
}

// validateDetailHeader 校验 R1 表头：A–D 必需且同名；E/F 可选，出现则必须同名；更后列非空即拒绝。
func validateDetailHeader(header []string) error {
	var mismatch []string
	for i, want := range detailHeader {
		got := cellAt(header, i)
		col := string(rune('A' + i))
		switch {
		case got == "":
			mismatch = append(mismatch, fmt.Sprintf("%s列(期望 %q，实际为空)", col, want))
		case got != want:
			mismatch = append(mismatch, fmt.Sprintf("%s列(期望 %q，实际 %q)", col, want, got))
		}
	}
	for i := len(detailHeader); i < len(header); i++ {
		got := cellAt(header, i)
		if got == "" {
			continue
		}
		want, ok := detailOptionalHeader[i+1]
		if ok && got == want {
			continue
		}
		mismatch = append(mismatch, fmt.Sprintf("%s列(期望 %q 或留空，实际 %q)", string(rune('A'+i)), want, got))
	}
	if len(mismatch) == 0 {
		return nil
	}
	shown := mismatch
	if len(shown) > 6 {
		shown = shown[:6]
	}
	parts := []string{fmt.Sprintf("第 %d 行必须是表头 A–D（%s），可选 E 设备名称 / F 设备型号",
		detailHeaderRow, strings.Join(detailHeader, "/"))}
	if vals := echoRow(header); vals != "" {
		parts = append(parts, fmt.Sprintf("实际识别到第 %d 行：%s", detailHeaderRow, vals))
	}
	parts = append(parts, fmt.Sprintf("不一致项（前 %d 条）：%s", len(shown), strings.Join(shown, "；")))
	parts = append(parts, "请使用外借明细模板（到达时间/外借方/数量/设备编号）后重试")
	return fmt.Errorf("%w：%s", ErrDetailTemplateMismatch, strings.Join(parts, "；"))
}

// isLedgerLayout 判断该工作表是否为台账模板版式（用于跨通道误传的提示）。
func isLedgerLayout(cell func(col, row int) string, headerRow int) bool {
	for i, want := range templateHeader {
		if cell(i+1, headerRow) != want {
			return false
		}
	}
	return true
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
