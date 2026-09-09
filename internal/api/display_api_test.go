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

// TestSearchByDisplayNo 按 display_no 形如 “6041（2）” 搜索应命中对应设备（§三十六）。
func TestSearchByDisplayNo(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test")
	sew := mkCategory(t, db, "缝纫设备")
	var ids []uint
	for i := 0; i < 4; i++ {
		eq, err := service.CreateEquipment(db, service.CreateEquipmentInput{
			EquipmentNo: strPtr("6041"), Name: "平缝机", Model: "M1", CategoryID: &sew, Operator: "a",
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, eq.ID)
	}
	// 搜索 “6041（2）” → 命中全部同号（基础编号匹配）
	req := httptest.NewRequest(http.MethodGet, "/api/equipment?q="+"6041%EF%BC%882%EF%BC%89&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("搜索失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			ID        uint   `json:"id"`
			DisplayNo string `json:"display_no"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 4 {
		t.Fatalf("display_no 搜索应命中同号 4 台，实际 %d", len(resp.Items))
	}
	_ = ids
}

// TestBorrowDisplayNoAPI 外借列表返回 display_no（决策18 §三十四）。
func TestBorrowDisplayNoAPI(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test")
	sew := mkCategory(t, db, "缝纫设备")
	borrower, err := service.CreateBorrower(db, "泰和", "", "")
	if err != nil {
		t.Fatal(err)
	}
	eq, err := service.CreateEquipment(db, service.CreateEquipmentInput{
		EquipmentNo: strPtr("6061"), Name: "平缝机", Model: "M1", CategoryID: &sew, Operator: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Transition(db, eq.ID, service.FlowRequest{
		Action: "BORROW", Operator: "a", BorrowerID: &borrower.ID,
	}); err != nil {
		t.Fatal(err)
	}
	w := doGet(t, r, "/api/borrows?status=OUTSTANDING&limit=10")
	if w.Code != http.StatusOK {
		t.Fatalf("外借列表失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []struct {
			DisplayNo string `json:"display_no"`
			Name      string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].DisplayNo != "6061" {
		t.Fatalf("外借 display_no 异常: %+v", resp.Items)
	}
}

// TestTeamViewDisplayNoAPI 班组视图返回 display_no（无编号按型号（n））。
func TestTeamViewDisplayNoAPI(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test")
	sew := mkCategory(t, db, "缝纫设备")
	for i := 0; i < 2; i++ {
		if _, err := service.CreateEquipment(db, service.CreateEquipmentInput{
			Name: "缝制熨斗", Model: "T-3NS", CategoryID: &sew, Operator: "a",
		}); err != nil {
			t.Fatal(err)
		}
	}
	w := doGet(t, r, "/api/teams/equipment")
	if w.Code != http.StatusOK {
		t.Fatalf("班组视图失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Unassigned struct {
			Devices []struct {
				DisplayNo string `json:"display_no"`
			} `json:"devices"`
		} `json:"unassigned"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, d := range resp.Unassigned.Devices {
		got[d.DisplayNo] = true
	}
	if !got["T-3NS（1）"] || !got["T-3NS（2）"] {
		t.Fatalf("班组视图 display_no 异常: %v", got)
	}
}

var firstID uint
