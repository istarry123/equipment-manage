package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"equipment/internal/models"
	"equipment/internal/service"
)

// TestDeleteDictAPI DELETE /api/teams/:id 与 /api/categories/:id 的受控删除契约。
func TestDeleteDictAPI(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test")

	cat := mkCategory(t, db, "在用类别")
	unusedCat := mkCategory(t, db, "备用类别")
	usedTeam, _ := service.CreateTeam(db, "在用组", "")
	freeTeam, _ := service.CreateTeam(db, "空组", "")

	// 设备占用类别与班组
	eq, err := service.CreateEquipment(db, service.CreateEquipmentInput{
		EquipmentNo: strPtr("6061"), Name: "平缝机", Model: "M1", CategoryID: &cat, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Transition(db, eq.ID, service.FlowRequest{
		Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &usedTeam.ID,
	}); err != nil {
		t.Fatal(err)
	}

	del := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 被引用 → 409
	if w := del(http.MethodDelete, "/api/teams/"+uStr(usedTeam.ID)); w.Code != http.StatusConflict {
		t.Fatalf("占用班组删除应 409: %d %s", w.Code, w.Body.String())
	}
	if w := del(http.MethodDelete, "/api/categories/"+uStr(cat)); w.Code != http.StatusConflict {
		t.Fatalf("占用类别删除应 409: %d %s", w.Code, w.Body.String())
	}
	// 未引用 → 200 且记录消失
	if w := del(http.MethodDelete, "/api/teams/"+uStr(freeTeam.ID)); w.Code != http.StatusOK {
		t.Fatalf("空班组删除应 200: %d %s", w.Code, w.Body.String())
	}
	if w := del(http.MethodDelete, "/api/categories/"+uStr(unusedCat)); w.Code != http.StatusOK {
		t.Fatalf("空类别删除应 200: %d %s", w.Code, w.Body.String())
	}
	var tc, cc int64
	db.Table("team").Where("id = ?", freeTeam.ID).Count(&tc)
	db.Table("category").Where("id = ?", unusedCat).Count(&cc)
	if tc != 0 || cc != 0 {
		t.Fatal("删除未生效")
	}
	// 不存在 → 404
	if w := del(http.MethodDelete, "/api/teams/999999"); w.Code != http.StatusNotFound {
		t.Fatalf("不存在删除应 404: %d", w.Code)
	}
}
