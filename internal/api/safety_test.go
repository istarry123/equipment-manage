package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"equipment/internal/models"
)

// 本文件是 v1.4 Phase 5 的**安全不变量**验收物：
// 把「危险操作必须受控」这条铁律（AGENTS.md §3.5）里**尚未被测试锁定**的部分固化为断言。
//
// 已有测试覆盖、此处不重复的部分（仅在报告中引用）：
//   - 历史日期不得晚于今天 → TestDoFlowHistoricalDate / service.TestTransitionFutureRejected
//   - 导入 REVIEW 门禁 422 → TestImportRunReviewGate
//   - 清空重导需 confirm → TestImportResetAndReimport
//   - 报废原因必填、报废为终态（service 层） → service.TestScrapFlow
//   - 班组/类别受控删除 → TestDeleteDictAPI
//   - 受限更正必须填原因 → service.TestCorrectEquipmentRequiresReason

// TestNoEquipmentDeleteRoute 设备禁止物理删除（决策 6）：
// 路由表不得存在任何 /api/equipment 的删除入口 —— 否则「报废收口」的语义会被绕过。
func TestNoEquipmentDeleteRoute(t *testing.T) {
	r := newTestEngine(t)
	cases := []string{
		"/api/equipment/1",
		"/api/equipment",
		"/api/equipment/1/flow",
		"/api/equipment/1/transactions",
	}
	for _, p := range cases {
		req := httptest.NewRequest(http.MethodDelete, p, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("DELETE %s 期望 404（不存在删除入口），实际 %d body=%s", p, w.Code, w.Body.String())
		}
	}
}

// TestEquipmentNumberNotEditableViaPut 设备编号不可直接修改（决策 13）：
// 受限更正通道（/correct，需原因+audit）之外的编辑接口**必须忽略编号字段**。
// 该性质目前由「请求结构体不含 equipment_no」保证；本测试把它锁住，防止日后被无意放开。
func TestEquipmentNumberNotEditableViaPut(t *testing.T) {
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

	// 故意夹带 equipment_no 字段：应被忽略（而非被写入）
	body := `{"name":"环形割刀（改名）","model":"EBK-SA2","equipment_no":"9999","remark":"改基本信息"}`
	req := httptest.NewRequest(http.MethodPut, "/api/equipment/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("编辑基本信息期望 200，实际 %d body=%s", w.Code, w.Body.String())
	}

	var got models.Equipment
	if err := db.First(&got, eq.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.EquipmentNo == nil || *got.EquipmentNo != "6061" {
		t.Errorf("设备编号被直接修改了（决策 13 禁止）：期望 6061，实际 %v", got.EquipmentNo)
	}
	if got.Name != "环形割刀（改名）" || got.Model != "EBK-SA2" {
		t.Errorf("基本信息未按预期更新：name=%q model=%q", got.Name, got.Model)
	}
}

// TestRestoreRequiresConfirm 恢复备份是危险操作（铁律 5）：
// 缺 filename → 400；未带 confirm:true → 400，且**不得**触碰任何文件。
func TestRestoreRequiresConfirm(t *testing.T) {
	r := newTestEngine(t)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/restore", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	if w := post(`{}`); w.Code != http.StatusBadRequest {
		t.Errorf("缺 filename 期望 400，实际 %d body=%s", w.Code, w.Body.String())
	}
	if w := post(`{"filename":"pre-restore-20260101-000000.db"}`); w.Code != http.StatusBadRequest {
		t.Errorf("未确认（confirm 缺省 false）期望 400，实际 %d body=%s", w.Code, w.Body.String())
	}
	if w := post(`{"filename":"pre-restore-20260101-000000.db","confirm":false}`); w.Code != http.StatusBadRequest {
		t.Errorf("confirm:false 期望 400，实际 %d body=%s", w.Code, w.Body.String())
	}

	// 400 响应必须是统一错误体（前端据 error.message 提示；code 为 HTTP 状态码）
	w := post(`{"filename":"x.db","confirm":false}`)
	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("错误体不是统一 JSON: %s", w.Body.String())
	}
	if resp.Error.Code != http.StatusBadRequest {
		t.Errorf("错误体 code 期望 400，实际 %d（body=%s）", resp.Error.Code, w.Body.String())
	}
	if resp.Error.Message == "" {
		t.Errorf("错误体缺少可读提示: %s", w.Body.String())
	}
}

// TestScrappedIsTerminalViaAPI 报废为终态（决策 4）：已报废设备不可再流转，
// 且**不得发生任何状态写入**（失败请求必须是无副作用的）。
func TestScrappedIsTerminalViaAPI(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test-version")
	now := models.Now()
	eq := models.Equipment{
		EquipmentNo:  strPtr("7001"),
		InternalCode: "EQ-000002",
		Name:         "报废样机",
		Model:        "X-1",
		Status:       "SCRAPPED",
		CurrentSince: models.ValidTime(now.Time),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&eq).Error; err != nil {
		t.Fatalf("seed 失败: %v", err)
	}
	var txnBefore int64
	db.Model(&models.Transaction{}).Where("equipment_id = ?", eq.ID).Count(&txnBefore)

	// 尝试从报废状态发起流转（多种动作都应被拒绝）
	actions := []string{"OUT_TO_TEAM", "TO_MAINTENANCE", "SCRAP", "BORROW"}
	for _, a := range actions {
		body := `{"action":"` + a + `","operator":"张工","remark":"测试"}`
		req := httptest.NewRequest(http.MethodPost, "/api/equipment/1/flow", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("报废后动作 %s 期望 400（终态拒绝），实际 %d body=%s", a, w.Code, w.Body.String())
		}
	}

	var after models.Equipment
	if err := db.First(&after, eq.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != "SCRAPPED" {
		t.Errorf("报废设备状态被改动：期望 SCRAPPED，实际 %s", after.Status)
	}
	var txnAfter int64
	db.Model(&models.Transaction{}).Where("equipment_id = ?", eq.ID).Count(&txnAfter)
	if txnAfter != txnBefore {
		t.Errorf("被拒绝的流转竟然写入了历史记录：前 %d 条 → 后 %d 条", txnBefore, txnAfter)
	}
}
