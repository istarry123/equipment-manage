package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"equipment/internal/models"

	"gorm.io/gorm"
)

// flowExportSeed 造一台外借设备 + 一台在库设备，返回外借方 ID。
func flowExportSeed(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	now := models.Now()
	br := models.Borrower{Name: "莒县双发", IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&br).Error; err != nil {
		t.Fatal(err)
	}
	eq1 := models.Equipment{EquipmentNo: strPtr("6041"), InternalCode: "EQ-000001", Name: "缝纫机", Model: "DDL-8700", Status: models.StatusBorrowed, CurrentBorrowerID: &br.ID, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&eq1).Error; err != nil {
		t.Fatal(err)
	}
	rec := models.BorrowRecord{EquipmentID: eq1.ID, BorrowerID: br.ID, BorrowDate: now, Status: models.BorrowOutstanding, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	eq1.CurrentBorrowRecordID = &rec.ID
	if err := db.Save(&eq1).Error; err != nil {
		t.Fatal(err)
	}
	eq2 := models.Equipment{EquipmentNo: strPtr("6061"), InternalCode: "EQ-000002", Name: "环形割刀", Model: "EBK-SA", Status: models.StatusInStock, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&eq2).Error; err != nil {
		t.Fatal(err)
	}
	return br.ID
}

// TestExportFlowDownload 下载应 200、Content-Type/Disposition 正确、正文为 xlsx。
func TestExportFlowDownload(t *testing.T) {
	db := newTestDB(t)
	flowExportSeed(t, db)
	r := New(db, "test-version")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/export/flow", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d body=%s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "spreadsheetml") {
		t.Fatalf("Content-Type 异常: %s", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, "filename*=UTF-8''") {
		t.Fatalf("Content-Disposition 异常: %s", cd)
	}
	// xlsx 魔数 PK
	if len(w.Body.Bytes()) < 4 || string(w.Body.Bytes()[:2]) != "PK" {
		t.Fatalf("正文应 xlsx（PK 开头）: %q", w.Body.String()[:min(len(w.Body.String()), 20)])
	}
}

// TestExportFlowFilterByStatus 状态筛选应生效并写审计。
func TestExportFlowFilterByStatus(t *testing.T) {
	db := newTestDB(t)
	flowExportSeed(t, db)
	r := New(db, "test-version")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/export/flow?status=BORROWED", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d", w.Code)
	}

	// 审计留痕
	var cnt int64
	db.Model(&models.AuditLog{}).Where("action = ?", "EXPORT_EQUIPMENT_FLOW").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("应写 1 条导出审计, got %d", cnt)
	}
	var al models.AuditLog
	db.Where("action = ?", "EXPORT_EQUIPMENT_FLOW").First(&al)
	if !strings.Contains(al.Detail, "status=BORROWED") {
		t.Fatalf("审计详情应含筛选条件: %s", al.Detail)
	}
}

// TestExportFlowEmptyFilter 空结果仍 200（不 500）。
func TestExportFlowEmptyFilter(t *testing.T) {
	db := newTestDB(t)
	flowExportSeed(t, db)
	r := New(db, "test-version")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/export/flow?status=SCRAPPED", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("空筛选应 200, got %d body=%s", w.Code, w.Body.String())
	}
}
