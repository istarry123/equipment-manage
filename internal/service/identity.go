package service

import (
	"fmt"
	"strings"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// 设备身份与展示（决策 18）：
//   - equipment.id 唯一身份；equipment_no 仅是标签（可重复/空/中文符号）；
//   - equipment_seq：同组序号（非身份）。分组键：
//       有编号 = equipment_no + name + model；
//       无编号 = model（空则 name，再空则 未编号设备）。

const unnumberedFallback = "未编号设备"

// SeqGroupLabel 返回分组标签（用于分组与展示前缀）及是否“有编号”。
func SeqGroupLabel(no *string, name, model string) (string, bool) {
	if no != nil && strings.TrimSpace(*no) != "" {
		return strings.TrimSpace(*no), true
	}
	if m := strings.TrimSpace(model); m != "" {
		return m, false
	}
	if n := strings.TrimSpace(name); n != "" {
		return n, false
	}
	return unnumberedFallback, false
}

// countSeqGroup 组内设备数（不含报废则含：报废设备也保留序号归属，避免组号漂移）。
func countSeqGroup(db *gorm.DB, no *string, name, model string) (int64, error) {
	label, numbered := SeqGroupLabel(no, name, model)
	if numbered {
		var c int64
		err := db.Model(&models.Equipment{}).
			Where("equipment_no = ? AND name = ? AND model = ?", label, name, model).
			Count(&c).Error
		return c, err
	}
	var c int64
	err := db.Model(&models.Equipment{}).
		Where("equipment_no IS NULL AND COALESCE(NULLIF(model,''), NULLIF(name,''), ?) = ?",
			unnumberedFallback, label).
		Count(&c).Error
	return c, err
}

// nextSeqInGroup 返回该组下一个可用序号（用于手动新增/导入建档时赋值）。
func nextSeqInGroup(db *gorm.DB, no *string, name, model string) (int, error) {
	c, err := countSeqGroup(db, no, name, model)
	if err != nil {
		return 0, err
	}
	return int(c) + 1, nil
}

// renumberSeqGroup 重新为某组按 id 顺序分配 1..n（编辑/更正名称型号编号后保持不变量）。
func renumberSeqGroup(db *gorm.DB, no *string, name, model string) error {
	label, numbered := SeqGroupLabel(no, name, model)
	var ids []uint
	var err error
	if numbered {
		err = db.Model(&models.Equipment{}).
			Where("equipment_no = ? AND name = ? AND model = ?", label, name, model).
			Order("id ASC").Pluck("id", &ids).Error
	} else {
		err = db.Model(&models.Equipment{}).
			Where("equipment_no IS NULL AND COALESCE(NULLIF(model,''), NULLIF(name,''), ?) = ?",
				unnumberedFallback, label).
			Order("id ASC").Pluck("id", &ids).Error
	}
	if err != nil {
		return err
	}
	for i, id := range ids {
		if err := db.Model(&models.Equipment{}).Where("id = ?", id).
			UpdateColumn("equipment_seq", i+1).Error; err != nil {
			return err
		}
	}
	return nil
}

// RenumberAllSeq 全量重算 equipment_seq（启动后幂等执行；也供手工修复）。
// 有编号按 (no,name,model)，无编号按 model→name→未编号设备；组内按 id 升序 1..n。
func RenumberAllSeq(db *gorm.DB) error {
	type row struct {
		ID          uint
		EquipmentNo *string
		Name        string
		Model       string
	}
	var rows []row
	if err := db.Model(&models.Equipment{}).Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}
	type grp struct {
		ids []uint
	}
	order := []string{}
	groups := map[string]*grp{}
	keyFor := func(r row) string {
		label, numbered := SeqGroupLabel(r.EquipmentNo, r.Name, r.Model)
		if numbered {
			return "N|" + label + "\x00" + r.Name + "\x00" + r.Model
		}
		return "U|" + label
	}
	for _, r := range rows {
		k := keyFor(r)
		g, ok := groups[k]
		if !ok {
			g = &grp{}
			groups[k] = g
			order = append(order, k)
		}
		g.ids = append(g.ids, r.ID) // rows 已按 id 升序
	}
	for _, k := range order {
		g := groups[k]
		for i, id := range g.ids {
			if err := db.Model(&models.Equipment{}).Where("id = ?", id).
				UpdateColumn("equipment_seq", i+1).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// DisplayNo 生成展示编号（后端统一计算下发，前端禁止自行拼接）：
//   - 有编号：同组(groupCount)多台 → 6041（n）；单台 → 6041；
//   - 无编号：恒带序号 → JUKI DDL-8700（1）、设备名（1）、未编号设备（1）。
func DisplayNo(no *string, name, model string, seq int, groupCount int64) string {
	label, numbered := SeqGroupLabel(no, name, model)
	if !numbered {
		return fmt.Sprintf("%s（%d）", label, seq)
	}
	if groupCount > 1 {
		return fmt.Sprintf("%s（%d）", label, seq)
	}
	return label
}
