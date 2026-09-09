package service

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"equipment/internal/database"
	"equipment/internal/models"

	"gorm.io/gorm"
)

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_", "\u0000", "_").Replace(t.Name())
	db, err := database.Open("mem://svc-" + name)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(sqlDB); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

func catID(t *testing.T, db *gorm.DB, name string) *uint {
	t.Helper()
	now := models.Now()
	c := models.Category{Name: name, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return &c.ID
}

func no(v string) *string { return &v }

func TestCreateEquipmentFlow(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "测试类")
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6061"), Name: "环形割刀", Model: "EBK-SA", CategoryID: cid,
		Remark: "测试", Operator: "张三",
	})
	if err != nil {
		t.Fatalf("新增失败: %v", err)
	}
	if eq.InternalCode != "EQ-000001" {
		t.Fatalf("内部码应为 EQ-000001: %s", eq.InternalCode)
	}
	if eq.Status != models.StatusInStock || !eq.CurrentSince.Valid {
		t.Fatalf("初始状态异常: %s since=%v", eq.Status, eq.CurrentSince)
	}
	// 初始流转记录
	var txn int64
	db.Model(&models.Transaction{}).Where("equipment_id = ? AND action = ?", eq.ID, models.ActionImportInit).Count(&txn)
	if txn != 1 {
		t.Fatalf("应写入 1 条初始流转, got %d", txn)
	}
}

// TestCreateDuplicateNo 决策 18：同 (no,name,model) 重复编号 = 多台真机，应放行并分配 seq 1..n。
func TestCreateDuplicateNo(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "测试类")
	in := CreateEquipmentInput{EquipmentNo: no("6041"), Name: "平缝机", Model: "M1", CategoryID: cid, Operator: "张三"}
	eqs := make([]*models.Equipment, 0, 4)
	for i := 0; i < 4; i++ {
		eq, err := CreateEquipment(db, in)
		if err != nil {
			t.Fatalf("同号第 %d 台应允许创建: %v", i+1, err)
		}
		eqs = append(eqs, eq)
	}
	for i, eq := range eqs {
		if eq.EquipmentSeq != i+1 {
			t.Fatalf("seq 应为 %d, got %d", i+1, eq.EquipmentSeq)
		}
	}
	// 不同名称可复用同号（各自独立分组）
	in2 := in
	in2.Name = "马连机"
	if _, err := CreateEquipment(db, in2); err != nil {
		t.Fatalf("不同设备应允许同号: %v", err)
	}
}

func TestCorrectEquipmentRequiresReason(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "类")
	eq, err := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: no("错号"), Name: "x", CategoryID: cid, Operator: "a"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Correct(db, eq.ID, CorrectInput{EquipmentNo: no("6061"), Operator: "张三"})
	if !errors.Is(err, ErrReason) {
		t.Fatalf("无原因应拒绝: %v", err)
	}
}

func TestCorrectEquipmentWritesAudit(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "类")
	eq, err := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: no("错号"), Name: "x", Model: "m", CategoryID: cid, Operator: "a"})
	if err != nil {
		t.Fatal(err)
	}
	fixed, err := Correct(db, eq.ID, CorrectInput{EquipmentNo: no("6061"), Reason: "导入时录错编号", Operator: "张三"})
	if err != nil {
		t.Fatalf("更正失败: %v", err)
	}
	if fixed.EquipmentNo == nil || *fixed.EquipmentNo != "6061" {
		t.Fatalf("编号未更正: %v", fixed.EquipmentNo)
	}
	var audit models.AuditLog
	if err := db.Where("action = ? AND target = ?", "CORRECT", "equipment:"+uintStr(eq.ID)).First(&audit).Error; err != nil {
		t.Fatalf("缺少 audit 记录: %v", err)
	}
	if audit.Operator != "张三" || audit.Reason != "导入时录错编号" {
		t.Fatalf("audit 内容异常: %+v", audit)
	}
	// 决策 18：同组再更正一台为同号 → 允许，并触发组内 seq 重排为 1..n
	eq2, err := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: no("777"), Name: "x", Model: "m", CategoryID: cid, Operator: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Correct(db, eq2.ID, CorrectInput{EquipmentNo: no("6061"), Reason: "合并为同号真机", Operator: "b"}); err != nil {
		t.Fatalf("同号多台应允许更正: %v", err)
	}
	var seqs []int
	db.Model(&models.Equipment{}).
		Where("equipment_no = ? AND name = ? AND model = ?", "6061", "x", "m").
		Order("id ASC").Pluck("equipment_seq", &seqs)
	if len(seqs) != 2 || seqs[0] != 1 || seqs[1] != 2 {
		t.Fatalf("同组 seq 应重排为 [1 2], got %v", seqs)
	}
}

func TestUnnumberedDuplicatesAllowed(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "类")
	for i := 0; i < 3; i++ {
		if _, err := CreateEquipment(db, CreateEquipmentInput{Name: "缝制熨斗", Model: "T-3NS", CategoryID: cid, Operator: "a"}); err != nil {
			t.Fatalf("无编号重复创建应允许: %v", err)
		}
	}
	var c int64
	db.Model(&models.Equipment{}).Where("name = ? AND model = ? AND equipment_no IS NULL", "缝制熨斗", "T-3NS").Count(&c)
	if c != 3 {
		t.Fatalf("应 3 台无编号, got %d", c)
	}
}

func uintStr(v uint) string { return strconv.FormatUint(uint64(v), 10) }
