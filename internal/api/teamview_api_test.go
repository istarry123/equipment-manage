package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"equipment/internal/models"
	"equipment/internal/service"

	"gorm.io/gorm"
)

func mkCategory(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()
	now := models.Now()
	c := models.Category{Name: name, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func uStr(v uint) string { return strconv.FormatUint(uint64(v), 10) }

// TestTeamEquipmentAPI 班组设备视图 API 契约 + 参数路径。
func TestTeamEquipmentAPI(t *testing.T) {
	db := newTestDB(t)
	r := New(db, "test")

	sew := mkCategory(t, db, "缝纫设备")
	a, _ := service.CreateTeam(db, "A班", "")

	// 2 台班组设备（6061/6062 → A班）
	for _, no := range []string{"6061", "6062"} {
		eq, err := service.CreateEquipment(db, service.CreateEquipmentInput{
			EquipmentNo: strPtr(no), Name: "平缝机", Model: "M1", CategoryID: &sew, Operator: "张工",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Transition(db, eq.ID, service.FlowRequest{
			Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &a.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// 1 台在库（未分配）
	if _, err := service.CreateEquipment(db, service.CreateEquipmentInput{
		EquipmentNo: strPtr("6069"), Name: "锁边机", Model: "S1", CategoryID: &sew, Operator: "张工",
	}); err != nil {
		t.Fatal(err)
	}

	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w := get("/api/teams/equipment")
	if w.Code != http.StatusOK {
		t.Fatalf("班组设备接口失败: %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Stats struct {
			TeamCount       int64 `json:"team_count"`
			TotalEquipment  int64 `json:"total_equipment"`
			InTeamEquipment int64 `json:"in_team_equipment"`
		} `json:"stats"`
		Teams []struct {
			Name       string `json:"name"`
			Total      int    `json:"total"`
			Categories []struct {
				Category string `json:"category"`
				Count    int    `json:"count"`
			} `json:"categories"`
		} `json:"teams"`
		Unassigned struct {
			Count int `json:"count"`
		} `json:"unassigned"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Stats.TotalEquipment != 3 || body.Stats.InTeamEquipment != 2 || body.Stats.TeamCount != 1 {
		t.Fatalf("统计异常: %+v", body.Stats)
	}
	found := false
	for _, tm := range body.Teams {
		if tm.Name == "A班" && tm.Total == 2 && len(tm.Categories) == 1 && tm.Categories[0].Count == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("A班分组异常: %+v", body.Teams)
	}
	if body.Unassigned.Count != 1 {
		t.Fatalf("未分配应为 1: %+v", body.Unassigned)
	}

	// 参数路径：按班组 / 未分配 / 关键字
	for _, p := range []string{
		"/api/teams/equipment?team=" + uStr(a.ID),
		"/api/teams/equipment?team=unassigned",
		"/api/teams/equipment?q=6061",
	} {
		if w := get(p); w.Code != http.StatusOK {
			t.Fatalf("参数路径失败 %s: %d %s", p, w.Code, w.Body.String())
		}
	}
}
