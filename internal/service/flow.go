package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// 流转相关业务错误。
var (
	ErrInvalidTransition = errors.New("当前状态不允许该操作")
	ErrUnknownAction     = errors.New("未知的流转动作")
	ErrTeamRequired      = errors.New("请选择目标班组")
	ErrTeamNotActive     = errors.New("目标班组已停用或不存在")
	ErrTeamSame          = errors.New("目标班组不能与原班组相同")
	ErrBorrowerRequired  = errors.New("请选择外借方")
	ErrBorrowerInactive  = errors.New("外借方已停用或不存在")
	ErrNoOpenBorrow      = errors.New("该设备没有未归还的外借单，无法归还")
	ErrScrapReason       = errors.New("报废必须填写原因")
	ErrNoScrapAfter      = errors.New("设备已报废（终态），不可再流转")
)

// FlowRequest 一次流转的入参（字段已类型化，由 API 层解析）。
type FlowRequest struct {
	Action             string
	Operator           string
	ToTeamID           *uint
	BorrowerID         *uint
	ExpectedReturnDate *time.Time
	Remark             string
}

// transitionPlan 一次流转的目标状态与联动计划。
type transitionPlan struct {
	status         string
	toTeamID       *uint
	toTeamName     string
	toBorrowerID   *uint
	borrowerName   string
	createBorrow   bool // 建外借单
	returnBorrow   bool // 归还外借单
	expectedReturn *time.Time
	remark         string
}

// Transition 执行一次流转（决策 4 白名单 + 铁律 4）：
// 同一事务内 = 更新 equipment 当前状态 + 追加 transaction（含名称快照）+ 联动 borrow_record。
func Transition(db *gorm.DB, id uint, in FlowRequest) (*models.Equipment, error) {
	if err := validateOperator(in.Operator); err != nil {
		return nil, err
	}
	var eq models.Equipment
	if err := db.First(&eq, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if eq.Status == models.StatusScrapped {
		return nil, ErrNoScrapAfter
	}
	from := eq.Status
	fromTeamID := eq.CurrentTeamID
	fromTeamName := teamName(db, eq.CurrentTeamID)

	plan := transitionPlan{remark: in.Remark}
	switch in.Action {
	case models.ActionOutToTeam: // 出库给班组
		if from != models.StatusInStock {
			return nil, wrapErr(in.Action, from)
		}
		t, err := fetchActiveTeam(db, in.ToTeamID)
		if err != nil {
			return nil, err
		}
		plan.status = models.StatusInTeam
		plan.toTeamID, plan.toTeamName = &t.ID, t.Name

	case models.ActionReturnFromTeam: // 班组归还入库
		if from != models.StatusInTeam {
			return nil, wrapErr(in.Action, from)
		}
		plan.status = models.StatusInStock

	case models.ActionHandover: // 班组转交
		if from != models.StatusInTeam {
			return nil, wrapErr(in.Action, from)
		}
		if in.ToTeamID == nil {
			return nil, ErrTeamRequired
		}
		if eq.CurrentTeamID != nil && *eq.CurrentTeamID == *in.ToTeamID {
			return nil, ErrTeamSame
		}
		t, err := fetchActiveTeam(db, in.ToTeamID)
		if err != nil {
			return nil, err
		}
		plan.status = models.StatusInTeam
		plan.toTeamID, plan.toTeamName = &t.ID, t.Name

	case models.ActionBorrow: // 外借（仓库/班组 → 外借方）
		if from != models.StatusInStock && from != models.StatusInTeam {
			return nil, wrapErr(in.Action, from)
		}
		b, err := fetchActiveBorrower(db, in.BorrowerID)
		if err != nil {
			return nil, err
		}
		plan.status = models.StatusBorrowed
		plan.toBorrowerID, plan.borrowerName = &b.ID, b.Name
		plan.createBorrow = true
		plan.expectedReturn = in.ExpectedReturnDate

	case models.ActionReturnBorrow: // 外借归还 → 仓库
		if from != models.StatusBorrowed {
			return nil, wrapErr(in.Action, from)
		}
		plan.status = models.StatusInStock
		plan.returnBorrow = true

	case models.ActionToMaintenance: // 送修
		if from != models.StatusInStock && from != models.StatusInTeam {
			return nil, wrapErr(in.Action, from)
		}
		plan.status = models.StatusMaintenance

	case models.ActionFromMaintenance: // 维修完成 → 仓库
		if from != models.StatusMaintenance {
			return nil, wrapErr(in.Action, from)
		}
		plan.status = models.StatusInStock

	case models.ActionScrap: // 报废（终态）
		if strings.TrimSpace(in.Remark) == "" {
			return nil, ErrScrapReason
		}
		plan.status = models.StatusScrapped

	default:
		return nil, ErrUnknownAction
	}

	now := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		// 外借单联动
		var borrowRecID *uint
		if plan.createBorrow {
			rec := models.BorrowRecord{
				EquipmentID: eq.ID,
				BorrowerID:  *plan.toBorrowerID,
				BorrowDate:  models.FromTime(now),
				Status:      models.BorrowOutstanding,
				CreatedAt:   models.FromTime(now),
				UpdatedAt:   models.FromTime(now),
			}
			if plan.expectedReturn != nil {
				rec.ExpectedReturnDate = models.ValidTime(*plan.expectedReturn)
			}
			if err := tx.Create(&rec).Error; err != nil {
				return err
			}
			borrowRecID = &rec.ID
		}
		if plan.returnBorrow {
			rec, err := openBorrowRecord(tx, eq)
			if err != nil {
				return err
			}
			rec.ActualReturnDate = models.ValidTime(now)
			rec.Status = models.BorrowReturned
			rec.UpdatedAt = models.FromTime(now)
			if err := tx.Save(&rec).Error; err != nil {
				return err
			}
			plan.borrowerName = borrowerName(tx, &rec.BorrowerID)
			plan.toBorrowerID = &rec.BorrowerID
			borrowRecID = &rec.ID
		}

		// 更新当前状态快照（决策 8 current_since）
		curBorrower := plan.toBorrowerID
		curBorrowRec := borrowRecID
		if plan.returnBorrow { // 归还：清除外借占位
			curBorrower = nil
			curBorrowRec = nil
		}
		eq.Status = plan.status
		eq.CurrentTeamID = nil // 除出库/转交外均清除
		if plan.toTeamID != nil {
			eq.CurrentTeamID = plan.toTeamID
		}
		eq.CurrentBorrowerID = curBorrower
		eq.CurrentBorrowRecordID = curBorrowRec
		eq.CurrentSince = models.ValidTime(now)
		eq.UpdatedAt = models.FromTime(now)
		if err := tx.Save(&eq).Error; err != nil {
			return err
		}

		// 历史记录（含名称快照，决策 14）
		return tx.Create(&models.Transaction{
			EquipmentID:    eq.ID,
			Action:         in.Action,
			FromStatus:     from,
			ToStatus:       plan.status,
			FromTeamID:     fromTeamID,
			ToTeamID:       plan.toTeamID,
			FromTeamName:   fromTeamName,
			ToTeamName:     plan.toTeamName,
			BorrowerID:     plan.toBorrowerID,
			BorrowerName:   plan.borrowerName,
			BorrowRecordID: borrowRecID,
			OccurredAt:     models.FromTime(now),
			Operator:       in.Operator,
			Remark:         strings.TrimSpace(plan.remark),
			CreatedAt:      models.FromTime(now),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &eq, nil
}

// 打开设备当前未归还的外借单（优先 current_borrow_record_id，其次最早未归还单）。
func openBorrowRecord(db *gorm.DB, eq models.Equipment) (*models.BorrowRecord, error) {
	var rec models.BorrowRecord
	if eq.CurrentBorrowRecordID != nil {
		if err := db.First(&rec, *eq.CurrentBorrowRecordID).Error; err == nil &&
			rec.Status == models.BorrowOutstanding {
			return &rec, nil
		}
	}
	err := db.Where("equipment_id = ? AND status = ?", eq.ID, models.BorrowOutstanding).
		Order("id ASC").First(&rec).Error
	if err != nil {
		return nil, ErrNoOpenBorrow
	}
	return &rec, nil
}

func fetchActiveTeam(db *gorm.DB, id *uint) (*models.Team, error) {
	if id == nil {
		return nil, ErrTeamRequired
	}
	var t models.Team
	if err := db.First(&t, *id).Error; err != nil || !t.IsActive {
		return nil, ErrTeamNotActive
	}
	return &t, nil
}

func fetchActiveBorrower(db *gorm.DB, id *uint) (*models.Borrower, error) {
	if id == nil {
		return nil, ErrBorrowerRequired
	}
	var b models.Borrower
	if err := db.First(&b, *id).Error; err != nil || !b.IsActive {
		return nil, ErrBorrowerInactive
	}
	return &b, nil
}

func teamName(db *gorm.DB, id *uint) string {
	if id == nil {
		return ""
	}
	var t models.Team
	if err := db.First(&t, *id).Error; err != nil {
		return ""
	}
	return t.Name
}

func borrowerName(db *gorm.DB, id *uint) string {
	if id == nil {
		return ""
	}
	var b models.Borrower
	if err := db.First(&b, *id).Error; err != nil {
		return ""
	}
	return b.Name
}

func wrapErr(action, from string) error {
	return fmt.Errorf("%w（动作 %s，当前状态 %s）", ErrInvalidTransition, action, from)
}

// ListTransactions 某设备完整流转历史（倒序，永久保存）。
func ListTransactions(db *gorm.DB, equipmentID uint, limit, offset int) ([]models.Transaction, int64, error) {
	var total int64
	if err := db.Model(&models.Transaction{}).Where("equipment_id = ?", equipmentID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.Transaction
	err := db.Where("equipment_id = ?", equipmentID).
		Order("occurred_at DESC, id DESC").
		Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}
