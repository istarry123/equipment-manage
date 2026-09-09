package service

import (
	"errors"
	"fmt"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// 清空重导（决策 18 ⑤；Phase 10）：生产导入 = 备份 → 清空业务数据 → 全量重导。
// 数据安全铁律：清空为危险操作，调用方必须先二次确认并先行备份；清空留痕 audit。
// 只清“业务数据”（equipment / flow_record / borrow_record / import_batch），
// 保留字典（category/team/borrower）、settings、audit_log（历史留痕永不清除）。

// ErrConfirmRequired 清空重导必须显式确认。
var ErrConfirmRequired = errors.New("清空是危险操作：需先备份并二次确认（confirm=true）")

// ResetResult 清空结果（被删行数与备份名）。
type ResetResult struct {
	BackupName       string `json:"backup_name"`
	EquipmentDeleted int64  `json:"equipment_deleted"`
	FlowDeleted      int64  `json:"flow_deleted"`
	BorrowDeleted    int64  `json:"borrow_deleted"`
	BatchDeleted     int64  `json:"batch_deleted"`
}

// CountEquipment 当前设备数（供确认提示）。
func CountEquipment(db *gorm.DB) (int64, error) {
	var n int64
	err := db.Model(&models.Equipment{}).Count(&n).Error
	return n, err
}

// ClearImportData 单事务清空业务数据（调用方负责备份与 audit）。
func ClearImportData(db *gorm.DB) (*ResetResult, error) {
	res := &ResetResult{}
	err := db.Transaction(func(tx *gorm.DB) error {
		var n int64
		// 外键顺序：borrow_record / flow_record 引用 equipment → 先删子表
		if err := tx.Model(&models.BorrowRecord{}).Count(&n).Error; err != nil {
			return err
		}
		res.BorrowDeleted = n
		if err := tx.Where("1=1").Delete(&models.BorrowRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Transaction{}).Count(&n).Error; err != nil {
			return err
		}
		res.FlowDeleted = n
		if err := tx.Where("1=1").Delete(&models.Transaction{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Equipment{}).Count(&n).Error; err != nil {
			return err
		}
		res.EquipmentDeleted = n
		if err := tx.Where("1=1").Delete(&models.Equipment{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.ImportBatch{}).Count(&n).Error; err != nil {
			return err
		}
		res.BatchDeleted = n
		return tx.Where("1=1").Delete(&models.ImportBatch{}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("清空业务数据失败: %w", err)
	}
	return res, nil
}
