package service

import (
	"errors"
	"testing"

	"equipment/internal/models"
)

func TestDeleteTeamGuarded(t *testing.T) {
	db := openDB(t)
	cat := *catID(t, db, "裁剪设备")
	// 未引用：可删除（建错清除）
	tmp, _ := CreateTeam(db, "临时组", "")
	if err := DeleteTeam(db, tmp.ID); err != nil {
		t.Fatalf("未引用班组应可删除: %v", err)
	}
	var cnt int64
	db.Model(&models.Team{}).Where("id = ?", tmp.ID).Count(&cnt)
	if cnt != 0 {
		t.Fatal("班组未真正删除")
	}

	a, _ := CreateTeam(db, "A班", "")
	b, _ := CreateTeam(db, "B班", "")
	// 设备当前占用 A → 不可删
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6061"), Name: "平缝机", Model: "M1", CategoryID: &cat, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &a.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteTeam(db, a.ID); !errors.Is(err, ErrTeamInUse) {
		t.Fatalf("占用班组应拒绝: %v", err)
	}
	// A → B 转交后：A 被历史引用仍不可删；B 当前占用不可删
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionHandover, Operator: "张工", ToTeamID: &b.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteTeam(db, a.ID); !errors.Is(err, ErrTeamReferenced) {
		t.Fatalf("历史引用班组应拒绝: %v", err)
	}
	if err := DeleteTeam(db, b.ID); !errors.Is(err, ErrTeamInUse) {
		t.Fatalf("当前占用 B 应拒绝: %v", err)
	}
	// 设备归还后 B 仍被历史引用 → 仍不可删（历史可追溯性优先）
	// （RETURN_FROM_TEAM 引用 to_team=B）
	if _, err := Transition(db, eq.ID, FlowRequest{Action: models.ActionReturnFromTeam, Operator: "张工"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteTeam(db, b.ID); !errors.Is(err, ErrTeamReferenced) {
		t.Fatalf("历史引用 B 应拒绝: %v", err)
	}
}

func TestDeleteCategoryGuarded(t *testing.T) {
	db := openDB(t)
	unused := *catID(t, db, "备用类别")
	used := *catID(t, db, "在用类别")
	if err := DeleteCategory(db, unused); err != nil {
		t.Fatalf("未使用类别应可删除: %v", err)
	}
	if _, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: no("6061"), Name: "平缝机", CategoryID: &used, Operator: "张工",
	}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCategory(db, used); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("使用中类别应拒绝: %v", err)
	}
}
