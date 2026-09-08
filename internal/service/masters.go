package service

import (
	"errors"
	"strings"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// ErrDuplicateName 名称已存在（班组/外借方字典唯一名）。
var ErrDuplicateName = errors.New("名称已存在")

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
