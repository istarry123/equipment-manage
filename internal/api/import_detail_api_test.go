package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"equipment/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// Phase 3（2026-09-11）：外借明细通道 API 测试（解析 + 匹配预览，只读）。

// writeDetailTestXLSX 生成外借明细测试文件（R1 表头、R2 起数据）。
func writeDetailTestXLSX(t *testing.T, headers []string, cells map[string]string) string {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Sheet1"
	for i, h := range headers {
		axis, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(sheet, axis, h); err != nil {
			t.Fatal(err)
		}
	}
	for axis, v := range cells {
		if err := f.SetCellStr(sheet, axis, v); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "detail.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func uploadFile(t *testing.T, r *gin.Engine, url string, path string) *httptest.ResponseRecorder {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestParseDetailMatchAPI 上传外借明细 → 解析 + 匹配预览（同号多台给出 预分配N 候选）。
func TestParseDetailMatchAPI(t *testing.T) {
	db := newTestDB(t)
	now := models.Now()
	mk := func(no, name, model string, seq int) {
		n := no
		eq := models.Equipment{
			EquipmentNo: &n, EquipmentSeq: seq,
			InternalCode: "EQ-" + no + "-" + model, Name: name, Model: model,
			Status: models.StatusInStock, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
	}
	mk("11231", "双针平车（重机）", "LH-3568", 1)
	mk("11231", "平车", "DDL-9000B", 1)
	if err := db.Create(&models.Borrower{Name: "泰和", IsActive: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	r := New(db, "test-version")

	path := writeDetailTestXLSX(t,
		[]string{"到达时间", "外借方", "数量", "设备编号"},
		map[string]string{
			"A2": "2015.4.12", "B2": "泰和", "C2": "1", "D2": "11231",
			"A3": "2016.1.5", "B3": "新公司Y", "C3": "1", "D3": "6040",
		})
	rec := uploadFile(t, r, "/api/import/borrow-detail/parse", path)
	if rec.Code != http.StatusOK {
		t.Fatalf("parse 应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		ParseID string `json:"parse_id"`
		Summary struct {
			Blocks        int    `json:"blocks"`
			TotalCount    int    `json:"total_count"`
			Numbered      int    `json:"numbered"`
			Devices       int    `json:"devices"`
			Unique        int    `json:"unique"`
			Ambiguous     int    `json:"ambiguous"`
			Missing       int    `json:"missing"`
			Borrowers     int    `json:"borrowers"`
			BorrowersNew  int    `json:"borrowers_new"`
			MinBorrowDate string `json:"min_borrow_date"`
			MaxBorrowDate string `json:"max_borrow_date"`
		} `json:"summary"`
		Preview struct {
			Items []struct {
				EquipmentNo string `json:"equipment_no"`
				Status      string `json:"status"`
				Candidates  []struct {
					Label     string `json:"label"`
					Name      string `json:"name"`
					Model     string `json:"model"`
					DisplayNo string `json:"display_no"`
				} `json:"candidates"`
			} `json:"items"`
			Ambiguous []struct {
				EquipmentNo string `json:"equipment_no"`
				Candidates  []struct {
					Label string `json:"label"`
				} `json:"candidates"`
			} `json:"ambiguous"`
			Missing []struct {
				EquipmentNo string `json:"equipment_no"`
			} `json:"missing"`
			Borrowers []struct {
				Name   string `json:"name"`
				Exists bool   `json:"exists"`
			} `json:"borrowers"`
		} `json:"preview"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ParseID == "" {
		t.Fatal("应返回 parse_id 供后续写入复用")
	}
	if body.Summary.Blocks != 2 || body.Summary.TotalCount != 2 || body.Summary.Numbered != 2 {
		t.Fatalf("摘要异常: %+v", body.Summary)
	}
	if body.Summary.Ambiguous != 1 || body.Summary.Missing != 1 || body.Summary.Unique != 0 {
		t.Fatalf("分桶异常: %+v", body.Summary)
	}
	if body.Summary.MinBorrowDate != "2015-04-12" || body.Summary.MaxBorrowDate != "2016-01-05" {
		t.Fatalf("外借日期区间异常: %s ~ %s", body.Summary.MinBorrowDate, body.Summary.MaxBorrowDate)
	}
	if body.Summary.BorrowersNew != 1 {
		t.Fatalf("应新建 1 个外借方（新公司Y），实际 %+v", body.Summary)
	}
	if len(body.Preview.Ambiguous) != 1 || len(body.Preview.Ambiguous[0].Candidates) != 2 {
		t.Fatalf("歧义清单应含 1 条 2 候选: %+v", body.Preview.Ambiguous)
	}
	if body.Preview.Ambiguous[0].Candidates[0].Label != "预分配1" ||
		body.Preview.Ambiguous[0].Candidates[1].Label != "预分配2" {
		t.Fatalf("候选应带 预分配N 标签: %+v", body.Preview.Ambiguous[0].Candidates)
	}
	if len(body.Preview.Missing) != 1 || body.Preview.Missing[0].EquipmentNo != "6040" {
		t.Fatalf("未匹配清单异常: %+v", body.Preview.Missing)
	}
	if len(body.Preview.Borrowers) != 2 {
		t.Fatalf("外借方计划应为 2 条: %+v", body.Preview.Borrowers)
	}
}

// TestParseDetailRejectsLedgerFile 误把总账文件传到外借明细通道 → 400 且提示改用台账通道。
func TestParseDetailRejectsLedgerFile(t *testing.T) {
	r := newTestEngine(t)
	path := filepath.Join(repoRootForImportTest, "设备借出总账.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Skip("设备借出总账.xlsx 不在仓库根目录，跳过")
	}
	rec := uploadFile(t, r, "/api/import/borrow-detail/parse", path)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("应 400，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("台账")) {
		t.Fatalf("错误信息应提示改用台账通道: %s", rec.Body.String())
	}
}

// TestRunDetailImportAPI 端到端补录：parse → run（单事务）→ 状态/外借单/流转落库；重复补录被拒。
func TestRunDetailImportAPI(t *testing.T) {
	db := newTestDB(t)
	now := models.Now()
	n1, n2, n3 := "1001", "2002", "2002"
	for i, no := range []string{n1, n2, n3} {
		eq := models.Equipment{
			EquipmentNo: &no, EquipmentSeq: 1, InternalCode: "EQ-00020" + string(rune('0'+i)),
			Name: "平车", Model: "DDL-9000B", Status: models.StatusInStock, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&eq).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.Borrower{Name: "泰和", IsActive: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	r := New(db, "test-version")
	path := writeDetailTestXLSX(t, []string{"到达时间", "外借方", "数量", "设备编号"}, map[string]string{
		"A2": "2020.5.1", "B2": "泰和", "C2": "1", "D2": "1001",
		"A3": "2020.5.2", "B3": "泰和", "C3": "1", "D3": "2002",
	})
	rec := uploadFile(t, r, "/api/import/borrow-detail/parse", path)
	if rec.Code != http.StatusOK {
		t.Fatalf("parse 应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		ParseID string `json:"parse_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}

	// 未确认 → 400（危险操作需二次确认）
	bodyNoConfirm, _ := json.Marshal(map[string]any{"parse_id": parsed.ParseID})
	reqBad := httptest.NewRequest(http.MethodPost, "/api/import/borrow-detail/run", bytes.NewReader(bodyNoConfirm))
	reqBad.Header.Set("Content-Type", "application/json")
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("未确认应 400，实际 %d: %s", recBad.Code, recBad.Body.String())
	}

	// 确认补录 → 200
	body, _ := json.Marshal(map[string]any{"parse_id": parsed.ParseID, "confirm": true})
	req := httptest.NewRequest(http.MethodPost, "/api/import/borrow-detail/run", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("run 应 200，实际 %d: %s", rec2.Code, rec2.Body.String())
	}
	var runResp struct {
		Report struct {
			Borrowed    int `json:"borrowed"`
			Created     int `json:"created"`
			Skipped     int `json:"skipped"`
			HistoryOnly int `json:"history_only"`
			Labeled     int `json:"labeled"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &runResp); err != nil {
		t.Fatal(err)
	}
	if runResp.Report.Borrowed != 2 || runResp.Report.Created != 0 || runResp.Report.Skipped != 0 ||
		runResp.Report.HistoryOnly != 0 || runResp.Report.Labeled != 2 {
		t.Fatalf("补录报告异常: %+v", runResp.Report)
	}
	var borrowed int64
	db.Model(&models.Equipment{}).Where("status = ?", models.StatusBorrowed).Count(&borrowed)
	var recs int64
	db.Model(&models.BorrowRecord{}).Where("status = ?", models.BorrowOutstanding).Count(&recs)
	if borrowed != 2 || recs != 2 {
		t.Fatalf("应置外借 2 台并建 2 张外借单，实际 %d/%d", borrowed, recs)
	}
	var flow int64
	db.Model(&models.Transaction{}).Where("action = ?", models.ActionBorrow).Count(&flow)
	if flow != 2 {
		t.Fatalf("应写 2 条 BORROW 流转，实际 %d", flow)
	}

	// 同文件重复补录 → 409（幂等）
	req3 := httptest.NewRequest(http.MethodPost, "/api/import/borrow-detail/run", bytes.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusConflict {
		t.Fatalf("重复补录应 409，实际 %d: %s", rec3.Code, rec3.Body.String())
	}
}
