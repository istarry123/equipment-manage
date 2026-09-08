package service

import (
	"errors"
	"strings"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// ErrDuplicateName 名称已存在（班组/外借方字典唯一名）。
var ErrDuplicateName = errors.New("名称已存在")

// 受控删除相关业务错误（保护数据可追溯性，决策 17）。
var (
	ErrTeamInUse      = errors.New("仍有设备在该班组（班组使用中），请先流转出库后再删除，或改用「停用」")
	ErrTeamReferenced = errors.New("该班组已被历史流转记录引用，为保留历史不可删除（可改用「停用」）")
	ErrCategoryInUse  = errors.New("仍有设备使用该类别，请先调整设备类别后再删除")
)

// CreateTeam 新增班组/内部单位。
func CreateTeam(db *gorm.DB, name, department string) (*models.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("班组名称不能为空")
	}
	var c int64
	if err := db.Model(&models.Team{}).Where("name = ?", name).Count(&c).Error; err != nil {
		return nil, err
	}
	if c > 0 {
		return nil, ErrDuplicateName
	}
	now := models.Now()
	t := models.Team{Name: name, Department: strings.TrimSpace(department), IsActive: true,
		CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateTeamInput 班组更新（改名/部门/停用）。
type UpdateTeamInput struct {
	Name       string
	Department *string
	IsActive   *bool
}

// UpdateTeam 更新班组。
func UpdateTeam(db *gorm.DB, id uint, in UpdateTeamInput) (*models.Team, error) {
	var t models.Team
	if err := db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name != "" && name != t.Name {
		var c int64
		if err := db.Model(&models.Team{}).Where("name = ? AND id <> ?", name, id).Count(&c).Error; err != nil {
			return nil, err
		}
		if c > 0 {
			return nil, ErrDuplicateName
		}
		t.Name = name
	}
	if in.Department != nil {
		t.Department = strings.TrimSpace(*in.Department)
	}
	if in.IsActive != nil {
		t.IsActive = *in.IsActive
	}
	t.UpdatedAt = models.Now()
	if err := db.Save(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateBorrower 新增外借方（公司）。
func CreateBorrower(db *gorm.DB, name, contact, phone string) (*models.Borrower, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("外借方名称不能为空")
	}
	var c int64
	if err := db.Model(&models.Borrower{}).Where("name = ?", name).Count(&c).Error; err != nil {
		return nil, err
	}
	if c > 0 {
		return nil, ErrDuplicateName
	}
	now := models.Now()
	b := models.Borrower{Name: name, Contact: strings.TrimSpace(contact), Phone: strings.TrimSpace(phone),
		IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// UpdateBorrowerInput 外借方更新。
type UpdateBorrowerInput struct {
	Name     string
	Contact  *string
	Phone    *string
	IsActive *bool
}

// UpdateBorrower 更新外借方。
func UpdateBorrower(db *gorm.DB, id uint, in UpdateBorrowerInput) (*models.Borrower, error) {
	var b models.Borrower
	if err := db.First(&b, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name != "" && name != b.Name {
		var c int64
		if err := db.Model(&models.Borrower{}).Where("name = ? AND id <> ?", name, id).Count(&c).Error; err != nil {
			return nil, err
		}
		if c > 0 {
			return nil, ErrDuplicateName
		}
		b.Name = name
	}
	if in.Contact != nil {
		b.Contact = strings.TrimSpace(*in.Contact)
	}
	if in.Phone != nil {
		b.Phone = strings.TrimSpace(*in.Phone)
	}
	if in.IsActive != nil {
		b.IsActive = *in.IsActive
	}
	b.UpdatedAt = models.Now()
	if err := db.Save(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// DeleteTeam 受控删除班组（决策 17）：
//   - 仍被设备占用（equipment.current_team_id）→ ErrTeamInUse；
//   - 被历史流转记录引用（flow_record.from/to_team_id）→ ErrTeamReferenced（保留历史可追溯）；
//   - 两者均无时才物理删除（可安全删除“建错的空班组”）。
func DeleteTeam(db *gorm.DB, id uint) error {
	var t models.Team
	if err := db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	var inUse int64
	if err := db.Model(&models.Equipment{}).Where("current_team_id = ?", id).Count(&inUse).Error; err != nil {
		return err
	}
	if inUse > 0 {
		return ErrTeamInUse
	}
	var ref int64
	if err := db.Model(&models.Transaction{}).
		Where("from_team_id = ? OR to_team_id = ?", id, id).Count(&ref).Error; err != nil {
		return err
	}
	if ref > 0 {
		return ErrTeamReferenced
	}
	return db.Delete(&models.Team{}, id).Error
}

// DeleteCategory 受控删除类别字典（决策 17）：被设备引用（category_id）→ ErrCategoryInUse。
func DeleteCategory(db *gorm.DB, id uint) error {
	var c models.Category
	if err := db.First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	var inUse int64
	if err := db.Model(&models.Equipment{}).Where("category_id = ?", id).Count(&inUse).Error; err != nil {
		return err
	}
	if inUse > 0 {
		return ErrCategoryInUse
	}
	return db.Delete(&models.Category{}, id).Error
}
