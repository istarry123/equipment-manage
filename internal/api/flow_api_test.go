package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"equipment/internal/models"
	"equipment/internal/service"
)

// postJSON 便捷 POST。
func postJSON(t *testing.T, r http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestDoFlowHistoricalDate API 层：occurred_at 透传 → borrow_date/current_since 生效；未来日期 400。
func TestDoFlowHistoricalDate(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test-version")
	cid := mkCategory(t, db, "裁剪设备")
	b, err := service.CreateBorrower(db, "泰和", "", "")
	if err != nil {
		t.Fatal(err)
	}
	eq, err := service.CreateEquipment(db, service.CreateEquipmentInput{
		EquipmentNo: strPtr("6061"), Name: "环形割刀", Model: "EBK-SA", CategoryID: &cid, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 历史借出
	w := postJSON(t, r, "/api/equipment/"+uStr(eq.ID)+"/flow", map[string]any{
		"action": models.ActionBorrow, "operator": "张工", "borrower_id": b.ID,
		"occurred_at": "2026-05-10 09:30:00",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("历史借出失败: %d body=%s", w.Code, w.Body.String())
	}
	var rec models.BorrowRecord
	if err := db.Where("equipment_id = ?", eq.ID).First(&rec).Error; err != nil {
		t.Fatal(err)
	}
	if got := rec.BorrowDate.Format("2006-01-02"); got != "2026-05-10" {
		t.Fatalf("borrow_date 应为 2026-05-10，实际 %s", got)
	}

	// 日期格式错误 → 400
	w2 := postJSON(t, r, "/api/equipment/"+uStr(eq.ID)+"/flow", map[string]any{
		"action": models.ActionBorrow, "operator": "张工", "borrower_id": b.ID,
		"occurred_at": "not-a-date",
	})
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("坏日期应 400，实际 %d body=%s", w2.Code, w2.Body.String())
	}

	// 未来日期 → 400（service ErrOccurredFuture 映射）
	eq2, err := service.CreateEquipment(db, service.CreateEquipmentInput{
		EquipmentNo: strPtr("6062"), Name: "拉布机", Model: "CM-01", CategoryID: &cid, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	w3 := postJSON(t, r, "/api/equipment/"+uStr(eq2.ID)+"/flow", map[string]any{
		"action": models.ActionBorrow, "operator": "张工", "borrower_id": b.ID,
		"occurred_at": "2099-01-01",
	})
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("未来日期应 400，实际 %d body=%s", w3.Code, w3.Body.String())
	}

	// 历史归还（外借单归还接口传 actual_return_date）：先正常借出 6062 再历史归还
	w4 := postJSON(t, r, "/api/equipment/"+uStr(eq2.ID)+"/flow", map[string]any{
		"action": models.ActionBorrow, "operator": "张工", "borrower_id": b.ID,
	})
	if w4.Code != http.StatusOK {
		t.Fatalf("借出 6062 失败: %d %s", w4.Code, w4.Body.String())
	}
	var rec6062 models.BorrowRecord
	if err := db.Where("equipment_id = ?", eq2.ID).First(&rec6062).Error; err != nil {
		t.Fatal(err)
	}
	w5 := postJSON(t, r, "/api/borrows/"+uStr(rec6062.ID)+"/return", map[string]any{
		"operator": "张工", "actual_return_date": "2026-06-01",
	})
	if w5.Code != http.StatusOK {
		t.Fatalf("历史归还失败: %d body=%s", w5.Code, w5.Body.String())
	}
	if err := db.First(&rec6062, rec6062.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !rec6062.ActualReturnDate.Valid || rec6062.ActualReturnDate.Time.Format("2006-01-02") != "2026-06-01" {
		t.Fatalf("实际归还日期应为 2026-06-01，实际 %v", rec6062.ActualReturnDate)
	}
}
