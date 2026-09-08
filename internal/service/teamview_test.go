package service

import (
	"testing"

	"equipment/internal/models"

	"gorm.io/gorm"
)

type tvFixtures struct {
	sewID, cutID, ironID uint
	teamA, teamB         uint
}

// seedTeamViewFixture 建：A/B 班组 + 缝纫/裁剪/熨烫类别。
func seedTeamViewFixture(t *testing.T, db *gorm.DB) tvFixtures {
	t.Helper()
	a, err := CreateTeam(db, "A班", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateTeam(db, "B班", "")
	if err != nil {
		t.Fatal(err)
	}
	return tvFixtures{
		sewID:  *catID(t, db, "缝纫设备"),
		cutID:  *catID(t, db, "裁剪设备"),
		ironID: *catID(t, db, "熨烫设备"),
		teamA:  a.ID, teamB: b.ID,
	}
}

// makeInTeam 新建编号设备并出库到班组，返回设备 id。
func makeInTeam(t *testing.T, db *gorm.DB, no, name, model string, cat, team uint) uint {
	t.Helper()
	eq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: noPtr(no), Name: name, Model: model, CategoryID: &cat, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionOutToTeam, Operator: "张工", ToTeamID: &team,
	}); err != nil {
		t.Fatal(err)
	}
	return eq.ID
}

func noPtr(s string) *string { return &s }

func findTeam(t *testing.T, teams []TeamGroup, name string) *TeamGroup {
	t.Helper()
	for i := range teams {
		if teams[i].Name == name {
			return &teams[i]
		}
	}
	t.Fatalf("未找到班组分组 %s", name)
	return nil
}

func catDevices(g *TeamGroup, cat string) []string {
	for i := range g.Categories {
		if g.Categories[i].Category == cat {
			out := make([]string, 0, len(g.Categories[i].Devices))
			for _, d := range g.Categories[i].Devices {
				if d.EquipmentNo != nil {
					out = append(out, *d.EquipmentNo)
				}
			}
			return out
		}
	}
	return nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Case1-5：班组查询/类别筛选/关键字/未分配/外借不计入班组。
func TestTeamViewBasicsAndFilters(t *testing.T) {
	db := openDB(t)
	fx := seedTeamViewFixture(t, db)
	br, _ := CreateBorrower(db, "泰和", "", "")

	makeInTeam(t, db, "6061", "平缝机", "M1", fx.sewID, fx.teamA)
	makeInTeam(t, db, "6062", "平缝机", "M1", fx.sewID, fx.teamA)
	makeInTeam(t, db, "7012", "裁剪机", "C1", fx.cutID, fx.teamA)
	makeInTeam(t, db, "8021", "熨台", "I1", fx.ironID, fx.teamA)

	// 未分配：在库设备
	if _, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: noPtr("6069"), Name: "锁边机", Model: "S1", CategoryID: &fx.sewID, Operator: "张工",
	}); err != nil {
		t.Fatal(err)
	}
	// 外借设备（外借时清空 current_team_id）
	borEq, err := CreateEquipment(db, CreateEquipmentInput{
		EquipmentNo: noPtr("8001"), Name: "烫台", Model: "T1", CategoryID: &fx.ironID, Operator: "张工",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, borEq.ID, FlowRequest{
		Action: models.ActionBorrow, Operator: "张工", BorrowerID: &br.ID,
	}); err != nil {
		t.Fatal(err)
	}

	view, err := TeamEquipmentView(db, TeamViewOptions{})
	if err != nil {
		t.Fatalf("TeamEquipmentView: %v", err)
	}
	// 统计与 Dashboard 同口径
	if view.Stats.TotalEquipment != 6 || view.Stats.InTeamEquipment != 4 ||
		view.Stats.TeamCount != 2 || view.Stats.UnassignedEquipment != 2 {
		t.Fatalf("统计异常: %+v", view.Stats)
	}
	a := findTeam(t, view.Teams, "A班")
	if a.Total != 4 {
		t.Fatalf("A班应为 4 台, got %d", a.Total)
	}
	if got := catDevices(a, "缝纫设备"); len(got) != 2 || got[0] != "6061" || got[1] != "6062" {
		t.Fatalf("A班缝纫设备异常: %v", got)
	}
	if got := catDevices(a, "裁剪设备"); len(got) != 1 || got[0] != "7012" {
		t.Fatalf("A班裁剪设备异常: %v", got)
	}
	if got := catDevices(a, "熨烫设备"); len(got) != 1 || got[0] != "8021" {
		t.Fatalf("A班熨烫设备异常: %v", got)
	}
	// 未分配：在库 + 外借（均 current_team_id IS NULL）
	if view.Unassigned.Count != 2 {
		t.Fatalf("未分配应为 2 台, got %d", view.Unassigned.Count)
	}
	foundStock, foundBorrow := false, false
	for _, d := range view.Unassigned.Devices {
		if d.EquipmentNo != nil {
			switch *d.EquipmentNo {
			case "6069":
				foundStock = d.Status == models.StatusInStock
			case "8001":
				foundBorrow = d.Status == models.StatusBorrowed
			}
		}
	}
	if !foundStock || !foundBorrow {
		t.Fatalf("未分配应含 在库6069 与外借8001")
	}

	// Case2：按类别筛选 → A班只剩缝纫、未分配只留缝纫
	v2, _ := TeamEquipmentView(db, TeamViewOptions{CategoryID: &fx.sewID})
	if got := catDevices(findTeam(t, v2.Teams, "A班"), "缝纫设备"); len(got) != 2 {
		t.Fatalf("类别筛选 A 班缝纫应为 2: %v", got)
	}
	if len(v2.Teams[0].Categories) != 1 || v2.Unassigned.Count != 1 {
		t.Fatalf("类别筛选后分组异常: cats=%d unassigned=%d", len(v2.Teams[0].Categories), v2.Unassigned.Count)
	}

	// Case3：搜索 6061 → 命中 1 行且在 A班缝纫
	v3, _ := TeamEquipmentView(db, TeamViewOptions{Keyword: "6061"})
	if v3.TotalRows != 1 {
		t.Fatalf("搜索 6061 应 1 行, got %d", v3.TotalRows)
	}
	if got := catDevices(findTeam(t, v3.Teams, "A班"), "缝纫设备"); len(got) != 1 || got[0] != "6061" {
		t.Fatalf("搜索定位异常: %v", got)
	}

	// Case5：外借设备不误入班组
	for _, tg := range view.Teams {
		for _, cg := range tg.Categories {
			for _, d := range cg.Devices {
				if d.EquipmentNo != nil && *d.EquipmentNo == "8001" {
					t.Fatalf("外借设备 8001 不应出现在班组分组")
				}
			}
		}
	}
}

// Case6/7：A班→B班 流转后视图迁移；transaction 不受视图影响。
func TestTeamViewHandoverAndHistoryUntouched(t *testing.T) {
	db := openDB(t)
	fx := seedTeamViewFixture(t, db)
	makeInTeam(t, db, "6061", "平缝机", "M1", fx.sewID, fx.teamA)

	countTx := func() int64 {
		var n int64
		db.Model(&models.Transaction{}).Count(&n)
		return n
	}
	before := countTx()

	// A班 显示 6061
	v1, _ := TeamEquipmentView(db, TeamViewOptions{})
	if got := catDevices(findTeam(t, v1.Teams, "A班"), "缝纫设备"); !containsStr(got, "6061") {
		t.Fatalf("A班应含 6061: %v", got)
	}

	// A班 → B班
	var eq models.Equipment
	if err := db.Where("equipment_no = ?", "6061").First(&eq).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := Transition(db, eq.ID, FlowRequest{
		Action: models.ActionHandover, Operator: "张工", ToTeamID: &fx.teamB,
	}); err != nil {
		t.Fatal(err)
	}

	// 视图迁移（Case6）
	v2, _ := TeamEquipmentView(db, TeamViewOptions{})
	a2 := findTeam(t, v2.Teams, "A班")
	if containsStr(catDevices(a2, "缝纫设备"), "6061") {
		t.Fatal("A班不应再包含 6061")
	}
	b2 := findTeam(t, v2.Teams, "B班")
	if !containsStr(catDevices(b2, "缝纫设备"), "6061") {
		t.Fatal("B班应包含 6061")
	}
	// A班(空) 与 B班(1) 数量正确（无筛选含空班组）
	if a2.Total != 0 || b2.Total != 1 {
		t.Fatalf("A/B 数量异常: A=%d B=%d", a2.Total, b2.Total)
	}

	// Case7：历史完整且视图调用不产生任何记录
	if n := countTx(); n != before+1 {
		t.Fatalf("流转后应 +1 条记录: before=%d now=%d", before, n)
	}
	if _, err := TeamEquipmentView(db, TeamViewOptions{}); err != nil {
		t.Fatal(err)
	}
	if n := countTx(); n != before+1 {
		t.Fatalf("视图调用不应产生 transaction: %d", n)
	}
	var hv models.Transaction
	if err := db.Where("action = ? AND equipment_id = ?", models.ActionHandover, eq.ID).
		Order("id DESC").First(&hv).Error; err != nil {
		t.Fatalf("转交历史缺失: %v", err)
	}
	if hv.FromTeamName != "A班" || hv.ToTeamName != "B班" {
		t.Fatalf("转交名称快照异常: %s -> %s", hv.FromTeamName, hv.ToTeamName)
	}
}
