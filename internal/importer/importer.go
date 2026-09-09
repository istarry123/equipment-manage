package importer

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"equipment/internal/models"
	"equipment/internal/service"

	"gorm.io/gorm"
)

// ImportReport 导入报告（指标对齐主章程：总记录/成功/跳过/重复/异常/无编号）。
type ImportReport struct {
	Filename     string   `json:"filename"`
	SourceHash   string   `json:"source_hash,omitempty"`
	BatchID      uint     `json:"batch_id,omitempty"`
	TotalSourceD int      `json:"total_source_d"` // 源台账数量合计（总记录）
	Groups       int      `json:"groups"`         // 分组数
	Imported     int      `json:"imported"`       // 成功导入设备台数
	Skipped      int      `json:"skipped"`        // 因 BLOCK 跳过的设备台数
	BlockGroups  int      `json:"block_groups"`   // 被 BLOCK 的分组数
	Duplicates   int      `json:"duplicates"`     // 同组重复编号多录次数（按多台真机展开，决策18）
	Warnings     int      `json:"warnings"`       // WARN 级问题数
	Reviews      int      `json:"reviews"`        // REVIEW 级问题数（需人工确认）
	Unnumbered   int      `json:"unnumbered"`     // 导入的无编号台数
	BorrowedNow  int      `json:"borrowed_now"`   // 用户勾选确认“当前仍借出”并置 BORROWED 的台数
	Categories   []string `json:"categories"`
	InternalCode string   `json:"internal_code"` // 生成的 EQ 内部码区间
	Time         string   `json:"time"`
}

// ErrDBNotEmpty 表示数据库已存在设备数据（禁止重复全量导入）。
var ErrDBNotEmpty = errors.New("数据库已存在设备数据：本功能仅支持首次全量导入；如需重导请走“备份→清空→导入”流程（危险操作需二次确认）")

// ErrBatchImported 表示同一文件（source hash 相同）已导入过（§二十六：幂等）。
var ErrBatchImported = errors.New("该文件/批次已经导入过（文件指纹相同）；如确需重导请先备份并清空数据后重新导入")

// ImportOptions 导入附加选项（Phase 9 Preview/Review）。
type ImportOptions struct {
	// ConfirmedSuspects 用户勾选确认“当前仍借出”的疑似在借设备 source_key 列表
	// （仅外部公司、可唯一定位的候选；导入同事务内置 BORROWED 并建 borrow_record+flow，§三十三）。
	ConfirmedSuspects []string
}

// Import 将解析结果写入数据库（单事务，决策 18 §四十三）。
func Import(db *gorm.DB, res *ParseResult) (*ImportReport, error) {
	return ImportWithOptions(db, res, ImportOptions{})
}

// ImportWithOptions 与 Import 相同，并应用用户勾选的疑似在借设备（置 BORROWED）。
func ImportWithOptions(db *gorm.DB, res *ParseResult, opts ImportOptions) (*ImportReport, error) {
	now := time.Now()
	report := &ImportReport{Filename: res.Filename, SourceHash: res.SourceHash,
		Time: now.Format("2006-01-02 15:04:05")}

	// 批次幂等（§二十五/二十六）：同文件指纹已成功导入 → 拒绝，绝不复制数据
	if res.SourceHash != "" {
		var c int64
		if err := db.Model(&models.ImportBatch{}).
			Where("source_hash = ? AND status = ?", res.SourceHash, batchStatusDone).Count(&c).Error; err != nil {
			return nil, err
		}
		if c > 0 {
			return nil, ErrBatchImported
		}
	}

	// 空库守卫（防覆盖，数据安全铁律）
	var existing int64
	if err := db.Model(&models.Equipment{}).Count(&existing).Error; err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrDBNotEmpty
	}

	// 汇总统计
	blockGroups := 0
	dups := 0
	warns := 0
	var okGroups []*Group
	for _, g := range res.Groups {
		if !g.OK {
			blockGroups++
		} else {
			okGroups = append(okGroups, g)
		}
		for _, c := range dupCounts(g.Numbers) {
			dups += c
		}
	}
	for _, is := range res.Issues {
		switch is.Level {
		case "WARN":
			warns++
		case IssueLevelReview:
			report.Reviews++
		}
	}
	report.Groups = len(res.Groups)
	report.TotalSourceD = res.TotalD
	report.BlockGroups = blockGroups
	report.Duplicates = dups
	report.Warnings = warns
	for _, g := range res.Groups {
		if !g.OK {
			report.Skipped += DeviceCount(g)
		}
	}
	report.Categories = distinctCategoryNames(okGroups)

	// 内部码序号
	var maxEQ int
	var lastCode string
	db.Model(&models.Equipment{}).Select("COALESCE(MAX(CAST(SUBSTR(internal_code, 4) AS INTEGER)),0)").Scan(&maxEQ)
	seq := maxEQ + 1
	nextCode := func() string {
		s := fmt.Sprintf("EQ-%06d", seq)
		seq++
		lastCode = s
		return s
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		// 1) 批次记录（先建占位，随后回填计数）
		batch := models.ImportBatch{
			SourceName:    res.Filename,
			SourceHash:    res.SourceHash,
			Status:        batchStatusDone,
			TotalRows:     res.TotalD,
			ImportedCount: 0,
			WarningCount:  warns,
			ErrorCount:    blockGroups,
			CreatedAt:     models.FromTime(now),
			UpdatedAt:     models.FromTime(now),
		}
		if err := tx.Create(&batch).Error; err != nil {
			return fmt.Errorf("写入 import_batch 失败: %w", err)
		}
		report.BatchID = batch.ID

		// 2) 类别
		catIDs := map[string]uint{}
		for _, name := range report.Categories {
			var c models.Category
			err := tx.Where("name = ?", name).First(&c).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c = models.Category{Name: name, Sort: 0}
				if err := tx.Create(&c).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			catIDs[name] = c.ID
		}

		imported := 0
		unnumbered := 0
		firstCode := ""
		eqBySource := map[string]uint{} // source_key -> equipment id（供勾选疑似应用，Phase 9）
		for _, g := range okGroups {
			catName := g.Category
			if catName == "" {
				catName = categoryOther
			}
			catID := catIDs[catName]
			sourceRemark := fmt.Sprintf("来源: %s（行 R%d-R%d）", res.Filename, g.RowFrom, g.RowTo)

			// 有编号设备：一条一设备（决策 18：重复编号=同号多台真机，逐 token 展开）
			for i, no := range g.Numbers {
				sk := fmt.Sprintf("R%d-R%d#N%d", g.RowFrom, g.RowTo, i+1)
				eq := models.Equipment{
					EquipmentNo:   ptr(no),
					InternalCode:  nextCode(),
					Name:          g.Name,
					Model:         g.Model,
					CategoryID:    &catID,
					Status:        statusInStock,
					CurrentSince:  models.ValidTime(now),
					ImportBatchID: &batch.ID,
					SourceKey:     sk, // 来源定位（§四十）
					CreatedAt:     models.FromTime(now),
					UpdatedAt:     models.FromTime(now),
				}
				if firstCode == "" {
					firstCode = eq.InternalCode
				}
				if err := tx.Create(&eq).Error; err != nil {
					return fmt.Errorf("写入设备 %s(%s) 失败: %w", g.Name, no, err)
				}
				eqBySource[sk] = eq.ID
				imported++
				if err := writeInit(tx, eq.ID, sourceRemark, now); err != nil {
					return err
				}
			}
			// 无编号设备（含描述文本当编号）：逐台展开（决策 18 §十）
			remark := g.DescRemark
			for i := 0; i < g.Unnumbered; i++ {
				sk := fmt.Sprintf("R%d-R%d#U%d", g.RowFrom, g.RowTo, i+1)
				eq := models.Equipment{
					EquipmentNo:   nil,
					InternalCode:  nextCode(),
					Name:          g.Name,
					Model:         g.Model,
					CategoryID:    &catID,
					Status:        statusInStock,
					CurrentSince:  models.ValidTime(now),
					Remark:        remark,
					ImportBatchID: &batch.ID,
					SourceKey:     sk,
					CreatedAt:     models.FromTime(now),
					UpdatedAt:     models.FromTime(now),
				}
				if firstCode == "" {
					firstCode = eq.InternalCode
				}
				if err := tx.Create(&eq).Error; err != nil {
					return fmt.Errorf("写入无编号设备 %s 失败: %w", g.Name, err)
				}
				eqBySource[sk] = eq.ID
				imported++
				unnumbered++
				if err := writeInit(tx, eq.ID, sourceRemark, now); err != nil {
					return err
				}
			}
		}
		report.Imported = imported
		report.Unnumbered = unnumbered
		if firstCode != "" {
			report.InternalCode = fmt.Sprintf("%s ~ %s", firstCode, lastCode)
		} else {
			report.InternalCode = "（无，全部被 BLOCK）"
		}

		// 3) 回填批次计数 + 事务内全量重算 equipment_seq（决策18；复用 service 权威分组逻辑）
		if err := tx.Model(&models.ImportBatch{}).Where("id = ?", batch.ID).Updates(map[string]any{
			"imported_count": imported,
			"updated_at":     models.FromTime(now),
		}).Error; err != nil {
			return err
		}
		if err := service.RenumberAllSeq(tx); err != nil {
			return fmt.Errorf("equipment_seq 分配失败: %w", err)
		}

		// 3b) 应用用户勾选“当前仍借出”的疑似在借设备（§三十三；同事务置 BORROWED + borrow_record + flow）
		borrowedNow := 0
		if len(opts.ConfirmedSuspects) > 0 {
			byKey := map[string]*SuspectCandidate{}
			_, candidates, _, _, _ := BuildReviewView(res)
			for _, c := range candidates {
				byKey[c.SourceKey] = c
			}
			for _, sk := range opts.ConfirmedSuspects {
				eqID, okEq := eqBySource[sk]
				if !okEq {
					continue
				}
				can, okCan := byKey[sk]
				if !okCan {
					continue // 只接受 Preview 列出的疑似候选，绝不猜测其他设备
				}
				var eq models.Equipment
				if err := tx.First(&eq, eqID).Error; err != nil {
					return err
				}
				if eq.Status != statusInStock {
					continue
				}
				// 外借方字典：按公司名复用/新建（外部公司；内部单位已被 Preview 排除）
				borrowerID, err := findOrCreateBorrower(tx, can.Company)
				if err != nil {
					return err
				}
				rec := models.BorrowRecord{
					EquipmentID: eq.ID, BorrowerID: borrowerID,
					Status:    models.BorrowOutstanding,
					CreatedAt: models.FromTime(now), UpdatedAt: models.FromTime(now),
				}
				if bd := parseCandidateDate(can.BorrowDate); bd != nil {
					rec.BorrowDate = models.FromTime(*bd)
				} else {
					rec.BorrowDate = models.FromTime(now)
				}
				if err := tx.Create(&rec).Error; err != nil {
					return err
				}
				eq.Status = models.StatusBorrowed
				eq.CurrentBorrowerID = &borrowerID
				eq.CurrentBorrowRecordID = &rec.ID
				eq.CurrentSince = models.ValidTime(rec.BorrowDate.Time)
				eq.UpdatedAt = models.FromTime(now)
				if err := tx.Save(&eq).Error; err != nil {
					return err
				}
				if err := tx.Create(&models.Transaction{
					EquipmentID: eq.ID, Action: models.ActionBorrow,
					FromStatus: statusInStock, ToStatus: models.StatusBorrowed,
					BorrowerID: &borrowerID, BorrowerName: can.Company,
					BorrowRecordID: &rec.ID,
					OccurredAt:     rec.BorrowDate, Operator: operatorImport,
					Remark:    fmt.Sprintf("导入确认当前在借（来源 R%d：%s）", can.Row, can.Company),
					CreatedAt: models.FromTime(now),
				}).Error; err != nil {
					return err
				}
				borrowedNow++
			}
		}
		report.BorrowedNow = borrowedNow

		// 4) audit 留痕
		return tx.Create(&models.AuditLog{
			Action:    "IMPORT",
			Target:    res.Filename,
			Detail:    fmt.Sprintf("批次#%d 导入 %d 台（台账源 %d），跳过 %d，分组 %d，Block %d，当前在借确认 %d，指纹 %s", batch.ID, imported, res.TotalD, report.Skipped, len(okGroups), blockGroups, borrowedNow, res.SourceHash),
			Reason:    "",
			Operator:  operatorImport,
			CreatedAt: models.FromTime(now),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(report.Categories)
	return report, nil
}

// findOrCreateBorrower 按名称查找外借方，不存在则新建（字典自动构建）。
func findOrCreateBorrower(tx *gorm.DB, name string) (uint, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("外借方名称为空，无法建立外借单")
	}
	var b models.Borrower
	if err := tx.Where("name = ?", name).First(&b).Error; err == nil {
		return b.ID, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	now := models.Now()
	b = models.Borrower{Name: name, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&b).Error; err != nil {
		return 0, err
	}
	return b.ID, nil
}

// parseCandidateDate 解析“2006-01-02”候选借出日期。
func parseCandidateDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), time.Local)
	if err != nil {
		return nil
	}
	return &t
}

// batchStatusDone import_batch 完成状态。
const batchStatusDone = "DONE"

// writeInit 为导入设备写初始流转记录（决策 7）。
func writeInit(tx *gorm.DB, equipmentID uint, remark string, now time.Time) error {
	return tx.Create(&models.Transaction{
		EquipmentID: equipmentID,
		Action:      actionImportInit,
		FromStatus:  "-",
		ToStatus:    statusInStock,
		OccurredAt:  models.FromTime(now),
		Operator:    operatorImport,
		Remark:      remark,
		CreatedAt:   models.FromTime(now),
	}).Error
}

// DeviceCount 一个分组的“潜在设备数”（用于报告/统计口径）。
func DeviceCount(g *Group) int {
	if g.HasNumericD && g.DSum > 0 {
		return g.DSum
	}
	return len(g.Numbers) + g.Unnumbered
}

func dupCounts(nums []string) map[string]int {
	seen := map[string]int{}
	out := map[string]int{}
	for _, n := range nums {
		seen[n]++
	}
	for n, c := range seen {
		if c > 1 {
			out[n] = c - 1
		}
	}
	return out
}

func distinctCategoryNames(groups []*Group) []string {
	m := map[string]bool{}
	for _, g := range groups {
		name := g.Category
		if name == "" {
			name = categoryOther
		}
		m[name] = true
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func ptr(s string) *string { return &s }
