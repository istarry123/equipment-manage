package importer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// J 列借出事件解析（决策 18 §十四~§二十一）。
// 职责边界：本文件只把 G/H/I/J/K 原始单元格解析为结构化 BorrowEvent，
// 不写库、不改设备状态（状态推导在 Phase 7，Preview/Review 在 Phase 9，清空重导在 Phase 10）。
// 铁律：无法可靠解析 → needs_review + 保留原始文本，绝不静默丢弃（§十六）。

// BorrowNumber 事件中借出的“一台”。
type BorrowNumber struct {
	No          string     `json:"no"`                    // 编号原文（空串=无编号/注释类）
	Unnumbered  bool       `json:"unnumbered"`            // 无编号设备
	Remark      string     `json:"remark,omitempty"`      // 括号注释原文（带拖布轮/入南库…）
	Raw         string     `json:"raw"`                   // 该台的原始片段（永不丢）
	ReturnDate  *time.Time `json:"return_date,omitempty"` // 可可靠解析的归还日期（入南库/回库线索）
	NeedsReview bool       `json:"needs_review"`          // 需人工确认
	ReviewWhy   string     `json:"review_why,omitempty"`
}

// BorrowEvent 一个历史借出事件（G 锚定行 + 后续续 J 行合并）。
// Name/Model：事件所在设备块（B/C 合并感知；供与 F 资产按块匹配，Phase 7）。
type BorrowEvent struct {
	RowFrom     int            `json:"row_from"`
	RowTo       int            `json:"row_to"`
	DateRaw     string         `json:"date_raw"`
	Date        *time.Time     `json:"date,omitempty"` // 缺日/异常 → nil（DateReview=true）
	DateReview  bool           `json:"date_review"`
	Company     string         `json:"company"`
	Name        string         `json:"name,omitempty"` // 事件所在设备块名称
	Model       string         `json:"model,omitempty"`
	CountRaw    string         `json:"count_raw"`
	Count       int            `json:"count"` // 解析台数（0=空/无法解析）
	JRaw        string         `json:"j_raw"` // J 全段原文（跨行以 | 连接）
	KRemark     string         `json:"k_remark,omitempty"`
	Numbers     []BorrowNumber `json:"numbers"`
	NeedsReview bool           `json:"needs_review"`
	ReviewWhys  []string       `json:"review_whys,omitempty"`
}

// review 记录一个确认原因并置位。
func (e *BorrowEvent) review(why string) {
	e.NeedsReview = true
	e.ReviewWhys = append(e.ReviewWhys, why)
}

var (
	dateDotted  = regexp.MustCompile(`^(\d{4})\.(\d{1,2})(?:\.(\d{1,2}))?$`)
	dateInText  = regexp.MustCompile(`(\d{4})[./年-](\d{1,2})(?:[./月-](\d{1,2}))?日?`)
	returnWords = []string{"入南库", "回库", "入库", "归还", "回收", "返库", "回仓"}
	// pairParen 匹配一组不含括号的内容（用于逐层剥离配对括号）
	pairParen   = regexp.MustCompile(`[\(（][^\(\)（）]*[\)）]`)
	orphanParen = regexp.MustCompile(`[\(\)（）]`)
)

// parseGDate 解析 G 列时间（YYYY.M.D，可缺日）。缺日/异常 → nil+review（不猜补日，W-11）。
func parseGDate(raw string) (t *time.Time, review bool) {
	m := dateDotted.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return nil, true
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	if y < 2000 || mo < 1 || mo > 12 {
		return nil, true
	}
	if m[3] == "" {
		return nil, true // 缺日（2018.4）→ REVIEW
	}
	d, _ := strconv.Atoi(m[3])
	if d < 1 || d > 31 {
		return nil, true
	}
	tt := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
	return &tt, false
}

// parseJCell J 单元格文本 → 借出设备清单（BorrowCellParser）。
// 算法：先把配对括号整组剥离并替换为占位标记（组内容存 remarks），剩文按空白切词，
// 编号入列表、标记按序回贴为前一台的注释；残留“孤括号”或无法归类 → NeedsReview，
// 原文始终保留在 Raw（§十六：绝不静默丢）。
func parseJCell(raw string) []BorrowNumber {
	flat, groups, odd := stripPairs(strings.TrimSpace(raw))
	var out []BorrowNumber
	lastNum := -1 // 最近一台编号在 out 的下标（供注释回贴）
	words := splitBorrowWords(flat)
	for _, w := range words {
		if w == "" {
			continue
		}
		if idx, isMark := parenMark(w); isMark {
			// 回贴注释
			remark := groups[idx]
			if lastNum >= 0 {
				out[lastNum].Remark = joinBorrowRemark(out[lastNum].Remark, remark)
			} else {
				// 无前置编号的孤立注释 → 保留原文 REVIEW
				out = append(out, BorrowNumber{Raw: remark, NeedsReview: true,
					ReviewWhy: "无前置编号的括号注释，保留原文待人工确认"})
			}
			continue
		}
		if w == tokenNoNumber {
			out = append(out, BorrowNumber{Unnumbered: true, Raw: w})
			lastNum = len(out) - 1
			continue
		}
		if isNumberLike(w) {
			bn := BorrowNumber{No: w, Raw: w}
			if odd {
				bn.NeedsReview = true
				bn.ReviewWhy = "存在未配对的括号或异常结构，编号已尽力提取，请人工复核"
			}
			out = append(out, bn)
			lastNum = len(out) - 1
			continue
		}
		// 无法归类的词（描述等）→ 保留原文 REVIEW
		out = append(out, BorrowNumber{Raw: w, NeedsReview: true,
			ReviewWhy: "无法自动归类的内容，保留原文待人工确认"})
	}
	// 回贴注释后为编号补 ReturnDate（入南库等线索）
	for i := range out {
		out[i].ReturnDate = returnClue(out[i].Remark)
	}
	if len(out) == 0 && strings.TrimSpace(raw) != "" {
		out = append(out, BorrowNumber{Raw: strings.TrimSpace(raw), NeedsReview: true,
			ReviewWhy: "无法自动解析，保留原文待人工确认"})
	}
	return out
}

// stripPairs 反复剥离最内层配对括号（全半角混用）并替换为 \x01N\x01 标记。
// 返回剥离后的平文、各组原文、是否残留孤括号/无法完全配对。
func stripPairs(s string) (flat string, groups []string, odd bool) {
	cur := s
	for {
		loc := pairParen.FindStringIndex(cur)
		if loc == nil {
			break
		}
		content := cur[loc[0]+1 : loc[1]-1] // 去掉首尾括号
		idx := len(groups)
		groups = append(groups, content)
		marker := fmt.Sprintf(" \x01%d\x01 ", idx) // 两侧补空格，避免与前导编号粘成一词
		cur = cur[:loc[0]] + marker + cur[loc[1]:]
	}
	// 残余孤括号：去掉但标记 REVIEW（原文仍整体保留于事件 JRaw）
	if orphanParen.MatchString(cur) {
		odd = true
		cur = orphanParen.ReplaceAllString(cur, " ")
	}
	return cur, groups, odd
}

func parenMark(w string) (int, bool) {
	if len(w) >= 3 && w[0] == '\x01' && w[len(w)-1] == '\x01' {
		if n, err := strconv.Atoi(w[1 : len(w)-1]); err == nil {
			return n, true
		}
	}
	return 0, false
}

func splitBorrowWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\u3000' || r == '|'
	})
}

func joinBorrowRemark(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

// isNumberLike 编号形态（ASCII 字母/数字/横杠/下划线/点等；允许中文前缀编号如 车间A-01）。
func isNumberLike(s string) bool {
	if s == "" {
		return false
	}
	hasAscii := false
	hasHan := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', (r >= 'a' && r <= 'z'), (r >= 'A' && r <= 'Z'):
			hasAscii = true
		case r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '#':
		case r >= '\u4e00' && r <= '\u9fff':
			hasHan = true
		default:
			return false
		}
	}
	if hasHan && !hasAscii {
		return false
	}
	return hasAscii
}

// returnClue 从注释中提取可可靠解析的“入南库/回库”归还日期（§二十一）；不猜日期。
func returnClue(remark string) *time.Time {
	if remark == "" {
		return nil
	}
	isReturn := false
	for _, w := range returnWords {
		if strings.Contains(remark, w) {
			isReturn = true
			break
		}
	}
	if !isReturn {
		return nil
	}
	m := dateInText.FindStringSubmatch(remark)
	if m == nil {
		return nil
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d := 1
	if m[3] != "" {
		d, _ = strconv.Atoi(m[3])
	}
	if y < 2000 || mo < 1 || mo > 12 || d < 1 || d > 31 {
		return nil
	}
	tt := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.Local)
	return &tt
}

// buildBorrowEvent 把一个事件（G 锚定 + 续 J 行）解析为结构化事件。
func buildBorrowEvent(rowFrom, rowTo int, dateRaw, company, countRaw, jRaw, k string) *BorrowEvent {
	e := &BorrowEvent{
		RowFrom: rowFrom, RowTo: rowTo,
		DateRaw: dateRaw, Company: company,
		CountRaw: countRaw, JRaw: jRaw, KRemark: k,
	}
	e.Date, e.DateReview = parseGDate(dateRaw)
	if n, err := strconv.Atoi(strings.TrimSpace(countRaw)); err == nil && n > 0 {
		e.Count = n
	}
	e.Numbers = parseJCell(jRaw)
	for i := range e.Numbers {
		if e.Numbers[i].NeedsReview {
			e.review(fmt.Sprintf("第 %d 台：%s", i+1, e.Numbers[i].ReviewWhy))
		}
	}
	if e.DateReview {
		e.review(fmt.Sprintf("日期原文 %q 无法可靠解析为完整日期（缺日或格式异常），待确认", dateRaw))
	}
	if e.Count > 0 {
		expanded := countBorrowUnits(e.Numbers)
		if expanded > 0 && expanded != e.Count {
			e.review(fmt.Sprintf("台账台数 I=%d 与 J 展开台数 %d 不一致，待人工核对", e.Count, expanded))
		}
	}
	return e
}

// countBorrowUnits 统计展开的设备台数（编号逐台计 1；无编号占位不计，批量按 I 理解）。
func countBorrowUnits(nums []BorrowNumber) int {
	n := 0
	for _, b := range nums {
		if b.Unnumbered || b.No == "" {
			continue
		}
		n++
	}
	return n
}
