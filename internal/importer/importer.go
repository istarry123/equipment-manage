package importer

import (
	"errors"
	"fmt"
	"sort"
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
	Categories   []string `json:"categories"`
	InternalCode string   `json:"internal_code"` // 生成的 EQ 内部码区间
	Time         string   `json:"time"`
}

// ErrDBNotEmpty 表示数据库已存在设备数据（禁止重复全量导入）。
var ErrDBNotEmpty = errors.New("数据库已存在设备数据：本功能仅支持首次全量导入；如需重导请走“备份→清空→导入”流程（危险操作需二次确认）")

// ErrBatchImported 表示同一文件（source hash 相同）已导入过（§二十六：幂等）。
var ErrBatchImported = errors.New("该文件/批次已经导入过（文件指纹相同）；如确需重导请先备份并清空数据后重新导入")

// Import 将解析结果写入数据库（单事务，决策 18 §四十三）。
// 流程：批次幂等检查 → 空库守卫 → 单事务{类别、import_batch、设备行(来源键)、IMPORT_INIT、
// batch 计数、equipment_seq 全量重算、audit}。
func Import(db *gorm.DB, res *ParseResult) (*ImportReport, error) {
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
		for _, g := range okGroups {
			catName := g.Category
			if catName == "" {
				catName = categoryOther
			}
			catID := catIDs[catName]
			sourceRemark := fmt.Sprintf("来源: %s（行 R%d-R%d）", res.Filename, g.RowFrom, g.RowTo)

			// 有编号设备：一条一设备（决策 18：重复编号=同号多台真机，逐 token 展开）
			for i, no := range g.Numbers {
				eq := models.Equipment{
					EquipmentNo:   ptr(no),
					InternalCode:  nextCode(),
					Name:          g.Name,
					Model:         g.Model,
					CategoryID:    &catID,
					Status:        statusInStock,
					CurrentSince:  models.ValidTime(now),
					ImportBatchID: &batch.ID,
					SourceKey:     fmt.Sprintf("R%d-R%d#N%d", g.RowFrom, g.RowTo, i+1), // 来源定位（§四十）
					CreatedAt:     models.FromTime(now),
					UpdatedAt:     models.FromTime(now),
				}
				if firstCode == "" {
					firstCode = eq.InternalCode
				}
				if err := tx.Create(&eq).Error; err != nil {
					return fmt.Errorf("写入设备 %s(%s) 失败: %w", g.Name, no, err)
				}
				imported++
				if err := writeInit(tx, eq.ID, sourceRemark, now); err != nil {
					return err
				}
			}
			// 无编号设备（含描述文本当编号）：逐台展开（决策 18 §十）
			remark := g.DescRemark
			for i := 0; i < g.Unnumbered; i++ {
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
					SourceKey:     fmt.Sprintf("R%d-R%d#U%d", g.RowFrom, g.RowTo, i+1),
					CreatedAt:     models.FromTime(now),
					UpdatedAt:     models.FromTime(now),
				}
				if firstCode == "" {
					firstCode = eq.InternalCode
				}
				if err := tx.Create(&eq).Error; err != nil {
					return fmt.Errorf("写入无编号设备 %s 失败: %w", g.Name, err)
				}
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

		// 4) audit 留痕
		return tx.Create(&models.AuditLog{
			Action:    "IMPORT",
			Target:    res.Filename,
			Detail:    fmt.Sprintf("批次#%d 导入 %d 台（台账源 %d），跳过 %d，分组 %d，Block %d，指纹 %s", batch.ID, imported, res.TotalD, report.Skipped, len(okGroups), blockGroups, res.SourceHash),
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
