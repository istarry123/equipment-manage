package importer

import (
	"testing"
	"time"

	"equipment/internal/models"
)

// Phase 5（2026-09-11）：外借明细补录对账测试。

// TestReconcileBorrowDetail 补录后对账 PASS；未补录台记为未补录（不算差异）。
func TestReconcileBorrowDetail(t *testing.T) {
	db := openMigratedDB(t, "detail-reco")
	now := models.Now()
	for i, no := range []string{"1001", "1002"} {
		n := no
		eq := models.Equipment{
			EquipmentNo: &n, EquipmentSeq: 1, InternalCode: "EQ-00040" + string(rune('0'+i)),
			Name: "平车", Model: "DDL-9000B", Status: models.StatusInStock, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
	}
	path := writeDetailXLSX(t, detailHeaders(), map[string]string{
		"A2": "2020.5.1", "B2": "甲公司", "C2": "1", "D2": "1001",
		"A3": "2020.5.2", "B3": "乙公司", "C3": "1", "D3": "1002",
	}, nil)
	parse, err := ParseBorrowDetail(path)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	match, err := MatchBorrowDetail(db, parse)
	if err != nil {
		t.Fatalf("匹配失败: %v", err)
	}

	// 补录前：两台均未补录 → 未补录 2，仍 PASS（无差异）
	reco, err := ReconcileBorrowDetail(db, parse, match)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if !reco.Pass || reco.NotWritten != 2 || reco.Written != 0 {
		t.Fatalf("补录前对账应为「未补录 2、PASS」: %+v", reco)
	}

	// 只补甲公司
	if _, err := ImportBorrowDetail(db, parse, match,
		BorrowDetailOptions{Skip: map[string]bool{match.Items[1].SourceKey: true}}); err != nil {
		t.Fatalf("补录失败: %v", err)
	}
	reco, err = ReconcileBorrowDetail(db, parse, match)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if !reco.Pass || reco.Written != 1 || reco.Matched != 1 || reco.NotWritten != 1 || reco.Mismatch != 0 {
		t.Fatalf("分批补录后对账异常: %+v", reco)
	}

	// 人为破坏：改外借日期 → 对账 FAIL 并列出问题
	var rec models.BorrowRecord
	if err := db.Where("remark LIKE ?", "%"+match.Items[0].SourceKey+"%").First(&rec).Error; err != nil {
		t.Fatal(err)
	}
	rec.BorrowDate = models.FromTime(now.Add(24 * time.Hour)) // 与文件 2020-05-01 不符
	if err := db.Save(&rec).Error; err != nil {
		t.Fatal(err)
	}
	reco, err = ReconcileBorrowDetail(db, parse, match)
	if err != nil {
		t.Fatalf("对账失败: %v", err)
	}
	if reco.Pass || reco.Mismatch != 1 {
		t.Fatalf("日期不符应 FAIL 且列出 1 条: %+v", reco)
	}
	if len(reco.Items) == 0 || reco.Items[0].Side != RecoSideMismatch {
		t.Fatalf("应列出不一致明细: %+v", reco.Items)
	}
}
