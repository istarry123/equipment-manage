package importer

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"equipment/internal/models"
	"equipment/internal/service"

	"gorm.io/gorm"
)

// 外借明细写入 —— v1.3 Phase 4（决策 19）。单事务：新建缺失设备 → 外借方建档 →
// 置 BORROWED + 建外借单 + 写流转记录（历史日期）→ 序号重算 → audit。
//
// 用户口径（2026-09-11）：
//   - 标签「外借N」按外借方顺序生成，只写入外借单/流转记录的备注，**不修改**台账真实名称型号；
//   - 同号多台按候选顺序自动分配（第 1 条→第 1 台），预览可复核、可覆盖；
//   - 未匹配编号用其外借标签新建（名称＝型号＝外借N）。
//
// 安全约束：流转历史只增不改；设备当前已有未归还外借时只补历史、不覆盖现状；
// 结构 BLOCK 的块整体不写入；同一文件指纹只能补录一次（幂等）。

// BorrowDetailOptions 写入选项。
type BorrowDetailOptions struct {
	Choices  map[string]uint // source_key → equipment_id（覆盖自动分配；必须在候选内）
	Skip     map[string]bool // source_key → 跳过该台
	Operator string          // 操作人（默认“系统导入”）
}

// BorrowDetailItemResult 逐台写入结果。
type BorrowDetailItemResult struct {
	SourceKey    string `json:"source_key"`
	Label        string `json:"label,omitempty"` // 外借N
	EquipmentNo  string `json:"equipment_no"`
	EquipmentID  uint   `json:"equipment_id,omitempty"`
	InternalCode string `json:"internal_code,omitempty"`
	Name         string `json:"name,omitempty"`
	Model        string `json:"model,omitempty"`
	Company      string `json:"company"`
	BorrowDate   string `json:"borrow_date"`
	Action       string `json:"action"` // CREATED_BORROW / BORROW_FULL / BORROW_HISTORY_ONLY / SKIPPED / BLOCKED
	Reason       string `json:"reason,omitempty"`
}

// BorrowDetailReport 外借明细补录报告。
type BorrowDetailReport struct {
	Filename         string                    `json:"filename"`
	SourceHash       string                    `json:"source_hash"`
	BatchID          uint                      `json:"batch_id,omitempty"`
	Total            int                       `json:"total"`
	Created          int                       `json:"created"`      // 新建设备台数
	Borrowed         int                       `json:"borrowed"`     // 置 BORROWED 并建外借单台数
	HistoryOnly      int                       `json:"history_only"` // 已有未归还外借 → 只补流转历史
	Skipped          int                       `json:"skipped"`      // 用户跳过台数
	Blocked          int                       `json:"blocked"`      // 因所在块 BLOCK 未写入台数
	Labeled          int                       `json:"labeled"`      // 打「外借N」标签台数
	BorrowersCreated []string                  `json:"borrowers_created"`
	Companies        []string                  `json:"companies"`
	Items            []*BorrowDetailItemResult `json:"items"`
	Issues           []Issue                   `json:"issues"`
	Time             string                    `json:"time"`
}

// ImportBorrowDetail 执行外借明细补录（单事务；不修改设备名称/型号）。
func ImportBorrowDetail(db *gorm.DB, parse *DetailParseResult, match *DetailMatchResult, opts BorrowDetailOptions) (*BorrowDetailReport, error) {
	if parse == nil || match == nil {
		return nil, errors.New("缺少解析或匹配结果")
	}
	operator := strings.TrimSpace(opts.Operator)
	if operator == "" {
		operator = operatorImport
	}
	now := time.Now()
	report := &BorrowDetailReport{
		Filename: parse.Filename, SourceHash: parse.SourceHash,
		Total: len(match.Items), Time: now.Format("2006-01-02 15:04:05"),
	}

	// 批次幂等（同文件指纹已补录过 → 拒绝，绝不重复放大）
	if parse.SourceHash != "" {
		var c int64
		if err := db.Model(&models.ImportBatch{}).
			Where("source_hash = ? AND status = ?", parse.SourceHash, batchStatusDone).Count(&c).Error; err != nil {
			return nil, err
		}
		if c > 0 {
			return nil, ErrBatchImported
		}
	}

	// 结构 BLOCK 的块整体不写入
	blocked := map[int]bool{}
	for _, b := range parse.Blocks {
		if !b.OK {
			blocked[b.Row] = true
		}
	}

	// 内部码序号（沿用台账导入口径：EQ-%06d 递增）
	var maxEQ int
	db.Model(&models.Equipment{}).Select("COALESCE(MAX(CAST(SUBSTR(internal_code, 4) AS INTEGER)),0)").Scan(&maxEQ)
	codeSeq := maxEQ + 1
	nextCode := func() string {
		s := fmt.Sprintf("EQ-%06d", codeSeq)
		codeSeq++
		return s
	}
	borrowerID := map[string]uint{}
	borrowerOrder := []string{}

	err := db.Transaction(func(tx *gorm.DB) error {
		batch := models.ImportBatch{
			SourceName: parse.Filename, SourceHash: parse.SourceHash, Status: batchStatusDone,
			TotalRows: parse.TotalCount, CreatedAt: models.FromTime(now), UpdatedAt: models.FromTime(now),
		}
		if err := tx.Create(&batch).Error; err != nil {
			return fmt.Errorf("写入 import_batch 失败: %w", err)
		}
		report.BatchID = batch.ID

		for _, it := range match.Items {
			res := &BorrowDetailItemResult{
				SourceKey: it.SourceKey, Label: it.Label, EquipmentNo: it.EquipmentNo,
				Company: it.Company, BorrowDate: it.BorrowDate,
			}
			report.Items = append(report.Items, res)

			if blocked[it.BlockRow] {
				res.Action = "BLOCKED"
				res.Reason = "所在块结构校验未通过（BLOCK），未写入"
				report.Blocked++
				continue
			}
			if opts.Skip[it.SourceKey] {
				res.Action = "SKIPPED"
				res.Reason = "用户选择跳过"
				report.Skipped++
				continue
			}
			if it.BorrowDate == "" {
				res.Action = "BLOCKED"
				res.Reason = "缺少可用的到达时间"
				report.Blocked++
				continue
			}

			// 1) 解析目标设备（现有）或按标签新建
			var eq models.Equipment
			createNew := false
			switch {
			case it.Chosen != nil:
				targetID := it.Chosen.EquipmentID
				if override, ok := opts.Choices[it.SourceKey]; ok && override != targetID {
					// 覆盖选择必须落在候选内，绝不接受任意设备 id
					allowed := false
					for _, c := range it.Candidates {
						if c.EquipmentID == override {
							allowed = true
							break
						}
					}
					if !allowed {
						res.Action = "SKIPPED"
						res.Reason = fmt.Sprintf("指定的设备 #%d 不在候选内，已跳过", override)
						report.Skipped++
						report.Issues = append(report.Issues, Issue{Row: it.Row, Code: "V-M2", Level: "WARN", Message: res.Reason})
						continue
					}
					targetID = override
				}
				if err := tx.First(&eq, targetID).Error; err != nil {
					return fmt.Errorf("设备 #%d 不存在: %w", targetID, err)
				}
			default:
				if it.Label == "" { // 文件给了型号、台账又不存在 → 不凭空新建
					res.Action = "SKIPPED"
					res.Reason = "编号不在台账且无外借标签，未新建（需人工确认名称/型号）"
					report.Skipped++
					report.Issues = append(report.Issues, Issue{Row: it.Row, Code: "V-M3", Level: "WARN", Message: res.Reason})
					continue
				}
				createNew = true
				no := it.EquipmentNo
				eq = models.Equipment{
					EquipmentNo: &no, InternalCode: nextCode(), Name: it.Label, Model: it.Label,
					Status: statusInStock, CurrentSince: models.ValidTime(now),
					ImportBatchID: &batch.ID, SourceKey: it.SourceKey,
					Remark:    fmt.Sprintf("外借明细补录新建（来源 %s R%d）", parse.Filename, it.Row),
					CreatedAt: models.FromTime(now), UpdatedAt: models.FromTime(now),
				}
				if err := tx.Create(&eq).Error; err != nil {
					return fmt.Errorf("新建设备 %s 失败: %w", it.EquipmentNo, err)
				}
				report.Created++
				res.EquipmentID, res.InternalCode = eq.ID, eq.InternalCode
				res.Name, res.Model = eq.Name, eq.Model
				// 初始流转记录（决策 7）：时间取该次外借日期，保证时间线顺序
				if err := tx.Create(&models.Transaction{
					EquipmentID: eq.ID, Action: actionImportInit, FromStatus: "-", ToStatus: statusInStock,
					OccurredAt: models.FromTime(mustDate(it.BorrowDate, now)), Operator: operator,
					Remark:    fmt.Sprintf("外借明细补录新建（来源 R%d，编号 %s）", it.Row, it.EquipmentNo),
					CreatedAt: models.FromTime(now),
				}).Error; err != nil {
					return err
				}
			}
			if !createNew {
				res.EquipmentID, res.InternalCode = eq.ID, eq.InternalCode
				res.Name, res.Model = eq.Name, eq.Model
			}
			if it.Label != "" {
				report.Labeled++
			}

			// 外借方字典（按名称复用/新建）
			bid, ok := borrowerID[it.Company]
			if !ok {
				id, err := findOrCreateBorrower(tx, it.Company)
				if err != nil {
					return err
				}
				borrowerID[it.Company] = id
				borrowerOrder = append(borrowerOrder, it.Company)
				bid = id
			}
			borrowDate := mustDate(it.BorrowDate, now)
			remark := detailRemark(it)

			// 2) 设备当前已有未归还外借 → 只补流转历史，不覆盖现状
			if eq.Status != statusInStock {
				if err := tx.Create(&models.Transaction{
					EquipmentID: eq.ID, Action: models.ActionBorrow,
					FromStatus: eq.Status, ToStatus: eq.Status,
					BorrowerID: &bid, BorrowerName: it.Company,
					OccurredAt: models.FromTime(borrowDate), Operator: operator,
					Remark:    remark + "（该设备当前已有未归还外借，仅补记历史）",
					CreatedAt: models.FromTime(now),
				}).Error; err != nil {
					return err
				}
				res.Action = "BORROW_HISTORY_ONLY"
				res.Reason = fmt.Sprintf("设备当前状态为 %s，未覆盖现状，仅补流转历史", eq.Status)
				report.HistoryOnly++
				report.Issues = append(report.Issues, Issue{Row: it.Row, Code: "V-M1", Level: "WARN", Message: res.Reason})
				continue
			}

			// 3) 外借单 + 置 BORROWED + 流转记录（同一事务，铁律 4）
			rec := models.BorrowRecord{
				EquipmentID: eq.ID, BorrowerID: bid, BorrowDate: models.FromTime(borrowDate),
				Status: models.BorrowOutstanding, Remark: remark,
				CreatedAt: models.FromTime(now), UpdatedAt: models.FromTime(now),
			}
			if err := tx.Create(&rec).Error; err != nil {
				return fmt.Errorf("建立外借单失败: %w", err)
			}
			eq.Status = models.StatusBorrowed
			eq.CurrentBorrowerID = &bid
			eq.CurrentBorrowRecordID = &rec.ID
			eq.CurrentSince = models.ValidTime(borrowDate)
			eq.UpdatedAt = models.FromTime(now)
			if err := tx.Save(&eq).Error; err != nil {
				return fmt.Errorf("更新设备状态失败: %w", err)
			}
			if err := tx.Create(&models.Transaction{
				EquipmentID: eq.ID, Action: models.ActionBorrow,
				FromStatus: statusInStock, ToStatus: models.StatusBorrowed,
				BorrowerID: &bid, BorrowerName: it.Company, BorrowRecordID: &rec.ID,
				OccurredAt: models.FromTime(borrowDate), Operator: operator,
				Remark: remark, CreatedAt: models.FromTime(now),
			}).Error; err != nil {
				return fmt.Errorf("写入流转记录失败: %w", err)
			}
			if createNew {
				res.Action = "CREATED_BORROW"
			} else {
				res.Action = "BORROW_FULL"
			}
			report.Borrowed++
		}

		// 4) 批次计数回填 + 序号重算 + audit 留痕
		if err := tx.Model(&models.ImportBatch{}).Where("id = ?", batch.ID).Updates(map[string]any{
			"imported_count": report.Borrowed,
			"warning_count":  report.HistoryOnly,
			"error_count":    report.Blocked,
			"updated_at":     models.FromTime(now),
		}).Error; err != nil {
			return err
		}
		if err := service.RenumberAllSeq(tx); err != nil {
			return fmt.Errorf("equipment_seq 重算失败: %w", err)
		}
		return tx.Create(&models.AuditLog{
			Action: "IMPORT_BORROW_DETAIL",
			Target: parse.Filename,
			Detail: fmt.Sprintf("外借明细补录：批次#%d 共 %d 台，置外借 %d，新建设备 %d，仅补历史 %d，跳过 %d，块阻断 %d，外借方 %d 个，指纹 %s",
				batch.ID, report.Total, report.Borrowed, report.Created, report.HistoryOnly,
				report.Skipped, report.Blocked, len(borrowerOrder), parse.SourceHash),
			Operator: operator, CreatedAt: models.FromTime(now),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	report.BorrowersCreated = borrowerOrder
	report.Companies = borrowerOrder
	return report, nil
}

// detailRemark 生成外借单/流转记录备注（含 外借N 标签、来源块行与行号、到达时间原文）。
func detailRemark(it *DetailMatchItem) string {
	var b strings.Builder
	b.WriteString("外借明细补录")
	if it.Label != "" {
		fmt.Fprintf(&b, "〔%s〕", it.Label)
	}
	fmt.Fprintf(&b, " 来源 %s R%d", it.BlockRows, it.Row)
	if it.BorrowRaw != "" {
		fmt.Fprintf(&b, "；到达时间原文 %s", it.BorrowRaw)
	}
	return b.String()
}

// mustDate 解析 YYYY-MM-DD；失败回退 fallback（正常路径不会走到）。
func mustDate(s string, fallback time.Time) time.Time {
	if t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.Local); err == nil {
		return t
	}
	return fallback
}
