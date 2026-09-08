package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"equipment/internal/database"
	"equipment/internal/models"

	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := database.Open("mem://api-" + name)
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

// TestListEquipmentKeyword 关键字查询应正常返回（回归：LIKE + join 组合）。
func TestListEquipmentKeyword(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test-version")
	now := models.Now()
	eq := models.Equipment{
		EquipmentNo:  strPtr("6061"),
		InternalCode: "EQ-000001",
		Name:         "环形割刀",
		Model:        "EBK-SA",
		Status:       "IN_STOCK",
		CurrentSince: models.ValidTime(now.Time),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&eq).Error; err != nil {
		t.Fatalf("seed 失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/equipment?q=6061&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("keyword 查询失败: %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Fatalf("期望命中 1 台，实际 %d", resp.Total)
	}

	// 空关键字也正常
	req2 := httptest.NewRequest(http.MethodGet, "/api/equipment?limit=10", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("空关键字查询失败: %d body=%s", w2.Code, w2.Body.String())
	}
}

func strPtr(s string) *string { return &s }
