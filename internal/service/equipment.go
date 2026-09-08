// Package service 承载业务规则（状态机/唯一性等校验与事务），HTTP Handler 不直接写业务。
package service

import (
	"errors"
	"fmt"
	"strings"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// 公共业务错误（便于 API 映射 HTTP 语义）。
var (
	ErrNotFound  = errors.New("记录不存在")
	ErrDuplicate = errors.New("同名称同型号下该编号已存在")
	ErrEmptyName = errors.New("设备名称不能为空")
	ErrOperator  = errors.New("操作人不能为空")
	ErrReason    = errors.New("受限更正必须填写原因")
)

// CreateEquipmentInput 新增设备入参。
type CreateEquipmentInput struct {
	EquipmentNo *string `json:"equipment_no"` // 可空：无编号设备（内部码承载身份，决策 6）
	Name        string  `json:"name"`
	Model       string  `json:"model"`
	CategoryID  *uint   `json:"category_id"`
	Remark      string  `json:"remark"`
	Operator    string  `json:"operator"` // 操作人（必填）
}

// CreateEquipment 手动新增设备：默认在库 + 初始流转记录（与导入口径一致）。
func CreateEquipment(db *gorm.DB, in CreateEquipmentInput) (*models.Equipment, error) {
	if err := validateOperator(in.Operator); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrEmptyName
	}
	model := strings.TrimSpace(in.Model)
	var no *string
	if in.EquipmentNo != nil && strings.TrimSpace(*in.EquipmentNo) != "" {
		v := strings.TrimSpace(*in.EquipmentNo)
		no = &v
	}
	if err := ensureCategory(db, in.CategoryID); err != nil {
		return nil, err
	}
	if err := checkNoUnique(db, name, model, no, 0); err != nil {
		return nil, err
	}

	now := models.Now()
	code, err := NextInternalCode(db)
	if err != nil {
		return nil, err
	}
	eq := models.Equipment{
		EquipmentNo:  no,
		InternalCode: code,
		Name:         name,
		Model:        model,
		CategoryID:   in.CategoryID,
		Status:       models.StatusInStock,
		CurrentSince: models.ValidTime(now.Time),
		Remark:       strings.TrimSpace(in.Remark),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&eq).Error; err != nil {
			return err
		}
		return tx.Create(&models.Transaction{
			EquipmentID: eq.ID,
			Action:      models.ActionImportInit,
			FromStatus:  "-",
			ToStatus:    models.StatusInStock,
			OccurredAt:  now,
			Operator:    in.Operator,
			Remark:      "手动新增设备",
			CreatedAt:   now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &eq, nil
}

// EditBasicInput 基本信息编辑（不含编号；编号只能走受限更正，决策 13）。
type EditBasicInput struct {
	Name       string
	Model      string
	CategoryID *uint
	Remark     string
}

// EditBasic 编辑设备基本信息（名称/型号/类别/备注）。
func EditBasic(db *gorm.DB, id uint, in EditBasicInput) (*models.Equipment, error) {
	var eq models.Equipment
	if err := db.First(&eq, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if err := ensureCategory(db, in.CategoryID); err != nil {
		return nil, err
	}
	eq.Name = name
	eq.Model = strings.TrimSpace(in.Model)
	eq.CategoryID = in.CategoryID
	eq.Remark = strings.TrimSpace(in.Remark)
	eq.UpdatedAt = models.Now()
	if err := db.Save(&eq).Error; err != nil {
		return nil, err
	}
	return &eq, nil
}

// CorrectInput 受限更正（决策 13）：更正录错的编号等，须填原因并记 audit。
type CorrectInput struct {
	EquipmentNo *string `json:"equipment_no"` // nil/空串 = 清为无编号
	Name        *string `json:"name"`         // 可一并更正
	Model       *string `json:"model"`
	CategoryID  *uint   `json:"category_id"`
	Remark      *string `json:"remark"`
	Reason      string  `json:"reason"`   // 必填
	Operator    string  `json:"operator"` // 必填
}

// Correct 受限更正：允许修正 equipment_no（其余字段也可顺带修正）。
func Correct(db *gorm.DB, id uint, in CorrectInput) (*models.Equipment, error) {
	if err := validateOperator(in.Operator); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrReason
	}
	var eq models.Equipment
	if err := db.First(&eq, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	old := fmt.Sprintf("编号=%v 名称=%s 型号=%s", eqNo(eq.EquipmentNo), eq.Name, eq.Model)

	var no *string
	if in.EquipmentNo != nil && strings.TrimSpace(*in.EquipmentNo) != "" {
		v := strings.TrimSpace(*in.EquipmentNo)
		no = &v
	}
	if err := checkNoUnique(db, eq.Name, eq.Model, no, eq.ID); err != nil {
		return nil, err
	}

	now := models.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		eq.EquipmentNo = no
		if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
			eq.Name = strings.TrimSpace(*in.Name)
		}
		if in.Model != nil {
			eq.Model = strings.TrimSpace(*in.Model)
		}
		if in.CategoryID != nil {
			if err := ensureCategory(tx, in.CategoryID); err != nil {
				return err
			}
			eq.CategoryID = in.CategoryID
		}
		if in.Remark != nil {
			eq.Remark = strings.TrimSpace(*in.Remark)
		}
		eq.UpdatedAt = now
		if err := tx.Save(&eq).Error; err != nil {
			return err
		}
		detail := fmt.Sprintf("更正前: %s | 更正后: 编号=%v 名称=%s 型号=%s", old, eqNo(eq.EquipmentNo), eq.Name, eq.Model)
		return tx.Create(&models.AuditLog{
			Action:    "CORRECT",
			Target:    fmt.Sprintf("equipment:%d", eq.ID),
			Detail:    detail,
			Reason:    strings.TrimSpace(in.Reason),
			Operator:  in.Operator,
			CreatedAt: now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &eq, nil
}

// NextInternalCode 生成下一个 EQ 内部码。
func NextInternalCode(db *gorm.DB) (string, error) {
	var max int
	if err := db.Model(&models.Equipment{}).
		Select("COALESCE(MAX(CAST(SUBSTR(internal_code, 4) AS INTEGER)),0)").
		Scan(&max).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("EQ-%06d", max+1), nil
}

// checkNoUnique 同 name+model 下 equipment_no 唯一（决策 16；排除自身 id）。
func checkNoUnique(db *gorm.DB, name, model string, no *string, excludeID uint) error {
	if no == nil {
		return nil
	}
	q := db.Model(&models.Equipment{}).
		Where("name = ? AND model = ? AND equipment_no = ?", name, model, *no)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var c int64
	if err := q.Count(&c).Error; err != nil {
		return err
	}
	if c > 0 {
		return ErrDuplicate
	}
	return nil
}

func ensureCategory(db *gorm.DB, id *uint) error {
	if id == nil {
		return nil
	}
	var c models.Category
	if err := db.First(&c, *id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("所选类别不存在")
		}
		return err
	}
	return nil
}

func validateOperator(op string) error {
	if strings.TrimSpace(op) == "" {
		return ErrOperator
	}
	return nil
}

func eqNo(p *string) string {
	if p == nil {
		return "(无)"
	}
	return *p
}
