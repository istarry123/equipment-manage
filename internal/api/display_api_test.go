package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"equipment/internal/service"
)

// TestEquipmentDisplayNoAPI 台账接口返回 display_no（重复编号 6041（1..4）、无编号 T-3NS（1））。
func TestEquipmentDisplayNoAPI(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test")
	sew := mkCategory(t, db, "缝纫设备")
	// 同号 4 台
	for i := 0; i < 4; i++ {
		eq, err := service.CreateEquipment(db, service.CreateEquipmentInput{
			EquipmentNo: strPtr("6041"), Name: "平缝机", Model: "M1", CategoryID: &sew, Operator: "a",
		})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			firstID = eq.ID
		}
	}
	// 无编号同型号 2 台
	for i := 0; i < 2; i++ {
		if _, err := service.CreateEquipment(db, service.CreateEquipmentInput{
			Name: "缝制熨斗", Model: "T-3NS", CategoryID: &sew, Operator: "a",
		}); err != nil {
			t.Fatal(err)
		}
	}
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	w := get("/api/equipment?q=6041&limit=10")
	if w.Code != http.StatusOK {
		t.Fatalf("列表失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			ID           uint   `json:"id"`
			EquipmentSeq int    `json:"equipment_seq"`
			DisplayNo    string `json:"display_no"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 4 {
		t.Fatalf("应 4 台, got %d", len(resp.Items))
	}
	for i, it := range resp.Items {
		want := []string{"6041（1）", "6041（2）", "6041（3）", "6041（4）"}[i]
		if it.DisplayNo != want || it.EquipmentSeq != i+1 {
			t.Fatalf("display 异常: idx=%d seq=%d display=%q", i, it.EquipmentSeq, it.DisplayNo)
		}
	}
	// 详情
	w = get("/api/equipment/" + uStr(firstID))
	var d struct {
		DisplayNo string `json:"display_no"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || d.DisplayNo != "6041（1）" {
		t.Fatalf("详情 display 异常: %q err=%v", d.DisplayNo, err)
	}
	// 无编号
	w = get("/api/equipment?q=T-3NS&limit=10")
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 || resp.Items[0].DisplayNo != "T-3NS（1）" || resp.Items[1].DisplayNo != "T-3NS（2）" {
		t.Fatalf("无编号 display 异常: %+v", resp.Items)
	}
}

var firstID uint
