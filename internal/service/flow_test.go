package service

import (
	"errors"
	"testing"
	"time"

	"equipment/internal/models"
)

func TestFlowFullChain(t *testing.T) {
	db := openDB(t) // 复用 equipment_test.go 的 openDB（mem://svc-<test>）
	cid := catID(t, db, "裁剪设备")

	// 班组（内部单位）
	t1, err := CreateTeam(db, "裁剪一组", "")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := CreateTeam(db, "裁剪二组", "")
	if err != nil {
		t.Fatal(err)
	}
	// 外借方
	b1, err := CreateBorrower(db, "泰和", "", "")
	if err != nil {
		t.Fatal(err)
	}

	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6061"), Name: "环形割刀", Model: "EBK-SA", CategoryID: cid, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	txCount := func() int64 {
		var n int64
		db.Model(&models.Transaction{}).Where("equipment_id = ?", eq.ID).Count(&n)
		return n
	}
	start := txCount()

	// 出库给班组（仓库 → 裁剪一组）
	e, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &t1.ID})
	if err != nil {
		t.Fatalf("出库失败: %v", err)
	}
	if e.Status != models.StatusInTeam || e.CurrentTeamID == nil || *e.CurrentTeamID != t1.ID {
		t.Fatalf("出库状态异常: %+v", e)
	}
	// 非法：班组内设备不能再次出库
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &t2.ID}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("非法出库应拒绝: %v", err)
	}
	// 班组转交 → 裁剪二组
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionHandover, Operator: "张工", ToTeamID: &t2.ID}); err != nil {
		t.Fatalf("转交失败: %v", err)
	}
	// 转交同班组应拒绝
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionHandover, Operator: "张工", ToTeamID: &t2.ID}); !errors.Is(err, ErrTeamSame) {
		t.Fatalf("同组转交应拒绝: %v", err)
	}
	// 班组内设备可直接外借
	e, err = Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "张工", BorrowerID: &b1.ID})
	if err != nil {
		t.Fatalf("外借失败: %v", err)
	}
	if e.Status != models.StatusBorrowed || e.CurrentBorrowerID == nil {
		t.Fatalf("外借状态异常: %+v", e)
	}
	var rec models.BorrowRecord
	if err := db.Where("equipment_id = ? AND status = ?", eq.ID, models.BorrowOutstanding).First(&rec).Error; err != nil {
		t.Fatalf("未生成外借单: %v", err)
	}
	// 白名单：外借中不能直接送修
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionToMaintenance, Operator: "张工"}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("外借中送修应拒绝: %v", err)
	}
	// 外借归还 → 仓库（决策 3）
	e, err = Transition(db, eq.ID, FlowRequest{Action: models.ActionReturnBorrow, Operator: "张工"})
	if err != nil {
		t.Fatalf("归还失败: %v", err)
	}
	if e.Status != models.StatusInStock || e.CurrentBorrowerID != nil || e.CurrentBorrowRecordID != nil {
		t.Fatalf("归还后状态异常: %+v", e)
	}
	if err := db.First(&rec, rec.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rec.Status != models.BorrowReturned || !rec.ActualReturnDate.Valid {
		t.Fatalf("外借单未回填归还: %+v", rec)
	}
	// 维修链路
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionToMaintenance, Operator: "张工"}); err != nil {
		t.Fatalf("送修失败: %v", err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionFromMaintenance, Operator: "张工"}); err != nil {
		t.Fatalf("维修完成失败: %v", err)
	}
	// 报废：无原因拒绝、有原因成功、终态拒绝
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionScrap, Operator: "张工"}); !errors.Is(err, ErrScrapReason) {
		t.Fatalf("报废无原因应拒绝: %v", err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionScrap, Operator: "张工", Remark: "老化报废"}); err != nil {
		t.Fatalf("报废失败: %v", err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &t1.ID}); !errors.Is(err, ErrNoScrapAfter) {
		t.Fatalf("报废后流转应拒绝: %v", err)
	}

	// 每步都有流水；总条数 = 初始1 + 7 次流转
	want := start + 7 // 出库/转交/外借/归还/送修/维修完成/报废
	if got := txCount(); got != want {
		t.Fatalf("流水条数应为 %d, 实际 %d", want, got)
	}

	// 名称快照检查（决策 14）：最近一条转交记录的 to_team_name 应为 裁剪二组
	var handover models.Transaction
	if err := db.Where("equipment_id = ? AND action = ?", eq.ID, models.ActionHandover).
		Order("id DESC").First(&handover).Error; err != nil {
		t.Fatalf("转交记录缺失: %v", err)
	}
	if handover.FromTeamName != "裁剪一组" || handover.ToTeamName != "裁剪二组" {
		t.Fatalf("名称快照异常: %s -> %s", handover.FromTeamName, handover.ToTeamName)
	}
}

func TestFlowBadActions(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "类")
	eq, err := CreateEquipment(db, CreateEquipmentInput{Name: "x", CategoryID: cid, Operator: "a"})
	if err != nil {
		t.Fatal(err)
	}
	// 在库设备不能归还/维修完成/转交
	for _, act := range []string{models.ActionReturnFromTeam, models.ActionFromMaintenance, models.ActionHandover, models.ActionReturnBorrow} {
		if _, err := Transition(db, eq.ID, FlowRequest{Action: act, Operator: "a"}); !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("动作 %s 应拒绝: %v", act, err)
		}
	}
	// 未知动作
	if _, err := Transition(db, eq.ID, FlowRequest{Action: "NOPE", Operator: "a"}); !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("未知动作应拒绝: %v", err)
	}
	// 出库需班组
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionOutToTeam, Operator: "a"}); !errors.Is(err, ErrTeamRequired) {
		t.Fatalf("缺少班组应拒绝: %v", err)
	}
	// 外借需外借方
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionBorrow, Operator: "a"}); !errors.Is(err, ErrBorrowerRequired) {
		t.Fatalf("缺少外借方应拒绝: %v", err)
	}
}

func TestMastersDuplicate(t *testing.T) {
	db := openDB(t)
	if _, err := CreateTeam(db, "裁剪一组", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateTeam(db, "裁剪一组", ""); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("重复班组应拒绝: %v", err)
	}
	if _, err := CreateBorrower(db, "泰和", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateBorrower(db, "泰和", "", ""); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("重复外借方应拒绝: %v", err)
	}
}

// TestFlowHistoricalDates v1.1 §二十二/§二十三：借出/归还支持历史发生日期；
// 未填默认今天；不得晚于当前时间。
func TestFlowHistoricalDates(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "裁剪设备")
	b1, err := CreateBorrower(db, "泰和", "", "")
	if err != nil {
		t.Fatal(err)
	}
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6061"), Name: "环形割刀", Model: "EBK-SA", CategoryID: cid, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 历史借出：2026-05-10 出借
	borrowAt := time.Date(2026, 5, 10, 9, 30, 0, 0, time.Local)
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionBorrow, Operator: "张工", BorrowerID: &b1.ID, OccurredAt: &borrowAt,
	}); err != nil {
		t.Fatalf("历史借出失败: %v", err)
	}
	var rec models.BorrowRecord
	if err := db.Where("equipment_id = ? AND status = ?", eq.ID, models.BorrowOutstanding).First(&rec).Error; err != nil {
		t.Fatalf("未生成外借单: %v", err)
	}
	if got := rec.BorrowDate.Format("2006-01-02 15:04"); got != "2026-05-10 09:30" {
		t.Fatalf("borrow_date 应为 2026-05-10 09:30，实际 %s", got)
	}
	var borrowTxn models.Transaction
	if err := db.Where("equipment_id = ? AND action = ?", eq.ID, models.ActionBorrow).
		Order("id DESC").First(&borrowTxn).Error; err != nil {
		t.Fatal(err)
	}
	if borrowTxn.OccurredAt.Format("2006-01-02") != "2026-05-10" {
		t.Fatalf("流转 occurred_at 应为 2026-05-10，实际 %s", borrowTxn.OccurredAt.Format("2006-01-02"))
	}
	// current_since 口径（决策 8）取发生时间
	var cur models.Equipment
	if err := db.First(&cur, eq.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !cur.CurrentSince.Valid || cur.CurrentSince.Time.Format("2006-01-02") != "2026-05-10" {
		t.Fatalf("current_since 应为历史借出日 2026-05-10，实际 %v", cur.CurrentSince)
	}

	// 未来日期拒绝（§二十二：不得晚于当前时间）——用另一台在库设备验证
	eq2, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6062"), Name: "拉布机", Model: "CM-01", CategoryID: cid, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(24 * time.Hour)
	if _, err := Transition(db, eq2.ID, FlowRequest{
		Action: models.ActionBorrow, Operator: "张工", BorrowerID: &b1.ID, OccurredAt: &future,
	}); !errors.Is(err, ErrOccurredFuture) {
		t.Fatalf("未来日期应拒绝，实际 %v", err)
	}

	// 历史归还：2026-06-01 归还
	returnAt := time.Date(2026, 6, 1, 17, 0, 0, 0, time.Local)
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionReturnBorrow, Operator: "张工", OccurredAt: &returnAt,
	}); err != nil {
		t.Fatalf("历史归还失败: %v", err)
	}
	if err := db.First(&rec, rec.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !rec.ActualReturnDate.Valid || rec.ActualReturnDate.Time.Format("2006-01-02") != "2026-06-01" {
		t.Fatalf("实际归还日期应为 2026-06-01，实际 %v", rec.ActualReturnDate)
	}
	// 无 OccurredAt → 默认今天
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionToMaintenance, Operator: "张工",
	}); err != nil {
		t.Fatalf("送修失败: %v", err)
	}
	var mt models.Transaction
	if err := db.Where("equipment_id = ? AND action = ?", eq.ID, models.ActionToMaintenance).
		Order("id DESC").First(&mt).Error; err != nil {
		t.Fatal(err)
	}
	if got := mt.OccurredAt.Time.Truncate(time.Second); got.Sub(time.Now()).Abs() > time.Minute {
		t.Fatalf("未指定日期时应默认今天，实际 %s", mt.OccurredAt.Format("2006-01-02 15:04:05"))
	}
}
