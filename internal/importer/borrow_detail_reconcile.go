package importer

import (
	"fmt"
	"strings"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// 外借明细补录对账 —— v1.3 Phase 5（决策 19）。只读：核对「已补录部分」是否正确。
//
// 口径：以**外借单/流转记录的备注来源键**（`源 R{from}-R{to}#N{i}`）为准逐台核对，
// 因此不依赖写入时的人工改选；未被本文件补录过的台（分批补录中未选的公司）记为
// 「未补录」，不算失败——PASS 只要求「已补录的台全部一致」。
const (
	RecoSideMatched    = "MATCHED"
	RecoSideNotWritten = "NOT_WRITTEN"
	RecoSideMismatch   = "MISMATCH"
)

// detailRemarkPrefix 补录备注前缀（对账识别用）。
const detailRemarkPrefix = "外借明细补录"

// BorrowDetailRecoItem 逐台对账结果。
type BorrowDetailRecoItem struct {
	SourceKey    string `json:"source_key"`
	Label        string `json:"label,omitempty"`
	EquipmentNo  string `json:"equipment_no"`
	Company      string `json:"company"`
	BorrowDate   string `json:"borrow_date"`
	Side         string `json:"side"`
	Detail       string `json:"detail,omitempty"`
	EquipmentID  uint   `json:"equipment_id,omitempty"`
	InternalCode string `json:"internal_code,omitempty"`
	Status       string `json:"status,omitempty"`
}

// BorrowDetailReconcile 对账报告。
type BorrowDetailReconcile struct {
	Filename   string                  `json:"filename"`
	SourceHash string                  `json:"source_hash"`
	Total      int                     `json:"total"`
	Written    int                     `json:"written"`     // 已补录台数
	Matched    int                     `json:"matched"`     // 已补录且一致
	NotWritten int                     `json:"not_written"` // 尚未补录（分批未选）
	Mismatch   int                     `json:"mismatch"`    // 已补录但不一致
	Items      []*BorrowDetailRecoItem `json:"items"`       // 仅列出未补录/不一致，便于排查
	Pass       bool                    `json:"pass"`
	Message    string                  `json:"message"`
}

// ReconcileBorrowDetail 逐台核对补录结果（只读）。
func ReconcileBorrowDetail(db *gorm.DB, parse *DetailParseResult, match *DetailMatchResult) (*BorrowDetailReconcile, error) {
	if parse == nil || match == nil {
		return nil, fmt.Errorf("缺少解析或匹配结果")
	}
	out := &BorrowDetailReconcile{Filename: parse.Filename, SourceHash: parse.SourceHash, Total: len(match.Items)}

	// 一次性取回本通道写入的外借单与流转记录（避免 N+1）
	var recs []models.BorrowRecord
	if err := db.Where("remark LIKE ?", detailRemarkPrefix+"%").Find(&recs).Error; err != nil {
		return nil, fmt.Errorf("查询外借单失败: %w", err)
	}
	var flows []models.Transaction
	if err := db.Where("remark LIKE ?", detailRemarkPrefix+"%").Find(&flows).Error; err != nil {
		return nil, fmt.Errorf("查询流转记录失败: %w", err)
	}
	// 备注 → 该记录归属的来源键（识别 `源 R2-R3#N1`）
	keyOf := func(remark string) string {
		i := strings.Index(remark, "源 ")
		if i < 0 {
			return ""
		}
		rest := remark[i+len("源 "):]
		end := strings.IndexAny(rest, "；; ")
		if end < 0 {
			return strings.TrimSpace(rest)
		}
		return strings.TrimSpace(rest[:end])
	}
	recByKey := map[string][]models.BorrowRecord{}
	for _, r := range recs {
		if k := keyOf(r.Remark); k != "" {
			recByKey[k] = append(recByKey[k], r)
		}
	}
	flowCount := map[string]int{}
	for _, f := range flows {
		if k := keyOf(f.Remark); k != "" {
			flowCount[k]++
		}
	}

	for _, it := range match.Items {
		row := &BorrowDetailRecoItem{
			SourceKey: it.SourceKey, Label: it.Label, EquipmentNo: it.EquipmentNo,
			Company: it.Company, BorrowDate: it.BorrowDate,
		}
		found := recByKey[it.SourceKey]
		if len(found) == 0 {
			row.Side = RecoSideNotWritten
			row.Detail = "尚未补录（分批补录中未选该外借方时可忽略）"
			out.NotWritten++
			out.Items = append(out.Items, row)
			continue
		}
		out.Written++
		if len(found) > 1 {
			row.Side = RecoSideMismatch
			row.Detail = fmt.Sprintf("同一来源键存在 %d 张外借单（应唯一），请人工核对", len(found))
			row.EquipmentID = found[0].EquipmentID
			out.Mismatch++
			out.Items = append(out.Items, row)
			continue
		}
		rec := found[0]
		row.EquipmentID = rec.EquipmentID
		var eq models.Equipment
		if err := db.First(&eq, rec.EquipmentID).Error; err == nil {
			row.InternalCode, row.Status = eq.InternalCode, eq.Status
		}
		var problems []string
		if rec.BorrowDate.Time.Format("2006-01-02") != it.BorrowDate {
			problems = append(problems, fmt.Sprintf("外借日期 %s ≠ 文件 %s",
				rec.BorrowDate.Time.Format("2006-01-02"), it.BorrowDate))
		}
		if rec.Status != models.BorrowOutstanding {
			problems = append(problems, fmt.Sprintf("外借单状态为 %s（应为 OUTSTANDING）", rec.Status))
		}
		if flowCount[it.SourceKey] == 0 {
			problems = append(problems, "缺少对应的流转记录")
		}
		if len(problems) > 0 {
			row.Side = RecoSideMismatch
			row.Detail = strings.Join(problems, "；")
			out.Mismatch++
			out.Items = append(out.Items, row)
			continue
		}
		row.Side = RecoSideMatched
		row.Detail = "已补录且与文件一致"
		out.Matched++
	}

	out.Pass = out.Mismatch == 0
	if out.Pass {
		out.Message = fmt.Sprintf("对账通过：文件 %d 台，已补录 %d 台（一致 %d），未补录 %d 台（不算差异）",
			out.Total, out.Written, out.Matched, out.NotWritten)
	} else {
		out.Message = fmt.Sprintf("对账失败：已补录 %d 台中 %d 台与文件不一致，请按下方清单核对", out.Written, out.Mismatch)
	}
	return out, nil
}
