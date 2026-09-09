package service

import (
	"fmt"
	"testing"

	"equipment/internal/models"
)

func noP(s string) *string { return &s }

func TestDisplayNoUnit(t *testing.T) {
	cases := []struct {
		no          *string
		name, model string
		seq         int
		grp         int64
		want        string
	}{
		{noP("6041"), "平缝机", "M1", 1, 1, "6041"},
		{noP("6041"), "平缝机", "M1", 2, 4, "6041（2）"},
		{nil, "缝制熨斗", "JUKI DDL-8700", 1, 1, "JUKI DDL-8700（1）"},
		{nil, "缝制熨斗", "", 2, 2, "缝制熨斗（2）"},
		{nil, "", "", 3, 1, "未编号设备（3）"},
	}
	for _, c := range cases {
		if got := DisplayNo(c.no, c.name, c.model, c.seq, c.grp); got != c.want {
			t.Fatalf("DisplayNo(%v,%s,%s,%d,%d)=%q want %q", c.no, c.name, c.model, c.seq, c.grp, got, c.want)
		}
	}
}

func TestRenumberAllSeqGroups(t *testing.T) {
	db := openDB(t)
	cid := catID(t, db, "类")
	// 同号 3 台
	for i := 0; i < 3; i++ {
		_, err := CreateEquipment(db, CreateEquipmentInput{EquipmentNo: noP("6041"), Name: "平缝机", Model: "M1", CategoryID: cid, Operator: "a"})
		if err != nil {
			t.Fatal(err)
		}
	}
	// 无编号同型号 2 台
	for i := 0; i < 2; i++ {
		_, err := CreateEquipment(db, CreateEquipmentInput{Name: "缝制熨斗", Model: "T-3NS", CategoryID: cid, Operator: "a"})
		if err != nil {
			t.Fatal(err)
		}
	}
	// 名称型号均空（未编号设备 回退组）
	for i := 0; i < 2; i++ {
		now := models.Now()
		if err := db.Create(&models.Equipment{
			InternalCode: fmt.Sprintf("EQ-%06d", 6+i),
			Name: "", Model: "", Status: models.StatusInStock,
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 打乱一个组的 seq 再全量重算（模拟旧库漂移）
	db.Model(&models.Equipment{}).Where("equipment_no = ?", "6041").Update("equipment_seq", 99) //nolint:errcheck
	if err := RenumberAllSeq(db); err != nil {
		t.Fatalf("RenumberAllSeq: %v", err)
	}
	check := func(where string, want []int) {
		var got []int
		db.Model(&models.Equipment{}).Where(where).Order("id ASC").Pluck("equipment_seq", &got)
		if len(got) != len(want) {
			t.Fatalf("%s seq 长度 %v want %v", where, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s seq=%v want %v", where, got, want)
			}
		}
	}
	check("equipment_no = '6041'", []int{1, 2, 3})
	check("equipment_no IS NULL AND model='T-3NS'", []int{1, 2})
	check("equipment_no IS NULL AND model='' AND name=''", []int{1, 2})
	// 幂等：再跑一次不变
	if err := RenumberAllSeq(db); err != nil {
		t.Fatal(err)
	}
	check("equipment_no = '6041'", []int{1, 2, 3})
}
