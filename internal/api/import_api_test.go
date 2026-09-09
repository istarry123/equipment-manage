package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"equipment/internal/database"
	"equipment/internal/models"
	"equipment/internal/service"
)

const repoRootForImportTest = "../.."

// TestImportRunReviewGate 端到端：REVIEW 未全部确认 → 拒绝导入；全部确认后成功（真实文件）。
func TestImportRunReviewGate(t *testing.T) {
	r := newTestEngine(t) // 空内存库
	path := filepath.Join(repoRootForImportTest, "设备借出总账.xlsx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 Excel 失败: %v", err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "设备借出总账.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/import/parse", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("parse 失败: %d %s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		ParseID string `json:"parse_id"`
		Summary struct {
			ReviewItems  int `json:"review_items"`
			Suspected    int `json:"suspected"`
			BorrowEvents int `json:"borrow_events"`
		} `json:"summary"`
		Preview struct {
			Reviews []struct {
				Key string `json:"key"`
			} `json:"reviews"`
			Suspected []struct {
				SourceKey string `json:"source_key"`
			} `json:"suspected"`
		} `json:"preview"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Summary.BorrowEvents != 144 {
		t.Fatalf("历史借出事件应为 144，实际 %d", parsed.Summary.BorrowEvents)
	}
	if parsed.Summary.ReviewItems == 0 {
		t.Log("提示：真实文件未产生 REVIEW 项（则门禁测试意义有限）")
	}

	// 1) 未确认任何 REVIEW → 拒绝（422）
	runBody, _ := json.Marshal(map[string]any{"parse_id": parsed.ParseID})
	req2 := httptest.NewRequest(http.MethodPost, "/api/import/run", bytes.NewReader(runBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if parsed.Summary.ReviewItems > 0 && rec2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("REVIEW 未确认应 422，实际 %d body=%s", rec2.Code, rec2.Body.String())
	}
	if parsed.Summary.ReviewItems == 0 && rec2.Code == http.StatusUnprocessableEntity {
		t.Fatalf("无 REVIEW 项却返回 422: %s", rec2.Body.String())
	}

	// 2) 全部确认 → 导入成功（真实文件约 144 REVIEW items；执行可能较重但单机可接受）
	var ack []string
	for _, rv := range parsed.Preview.Reviews {
		ack = append(ack, rv.Key)
	}
	var sus []string
	for _, s := range parsed.Preview.Suspected {
		sus = append(sus, s.SourceKey)
	}
	runBody2, _ := json.Marshal(map[string]any{
		"parse_id": parsed.ParseID, "confirmed_suspects": sus,
		"acknowledged_reviews": ack,
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/import/run", bytes.NewReader(runBody2))
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("确认后导入失败: %d %s", rec3.Code, rec3.Body.String())
	}
	var report struct {
		Report struct {
			Imported    int `json:"imported"`
			BorrowedNow int `json:"borrowed_now"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec3.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Report.Imported != 2147 {
		t.Fatalf("应导入 2147 台，实际 %d", report.Report.Imported)
	}
	if report.Report.BorrowedNow == 0 {
		t.Log("提示：勾选疑似后 borrowed_now=0（真实文件外部疑似候选可能为空或全勾选失败）")
	}
	// 对账（§四十七）：run 后会话保留（只删临时文件），应能对账且 PASS
	recBody, _ := json.Marshal(map[string]any{"parse_id": parsed.ParseID})
	req4 := httptest.NewRequest(http.MethodPost, "/api/import/reconcile", bytes.NewReader(recBody))
	req4.Header.Set("Content-Type", "application/json")
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("对账失败: %d %s", rec4.Code, rec4.Body.String())
	}
	var recon struct {
		Reconcile struct {
			Pass        bool `json:"pass"`
			ExcelOKDevs int  `json:"excel_ok_devs"`
			DBBatchDevs int  `json:"db_batch_devs"`
			Missing     int  `json:"missing"`
			Extra       int  `json:"extra"`
		} `json:"reconcile"`
	}
	if err := json.Unmarshal(rec4.Body.Bytes(), &recon); err != nil {
		t.Fatal(err)
	}
	if !recon.Reconcile.Pass || recon.Reconcile.ExcelOKDevs != 2147 ||
		recon.Reconcile.DBBatchDevs != 2147 ||
		recon.Reconcile.Missing != 0 || recon.Reconcile.Extra != 0 {
		t.Fatalf("对账应 PASS 且 2147==2147: %+v", recon.Reconcile)
	}
}

// TestImportResetAndReimport 清空重导（Phase10 决策18⑤）：非空库 → reset(备份+清空) → 全量重导。
func TestImportResetAndReimport(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "equipment.db")
	db, err := database.Open(dbFile)
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
	r := New(db, "test")
	// 预置一台设备 → 库非空（模拟旧数据）
	cid := mkCategory(t, db, "裁剪设备")
	if _, err := service.CreateEquipment(db, service.CreateEquipmentInput{
		EquipmentNo: strPtr("6061"), Name: "环形割刀", Model: "EBK-SA", CategoryID: &cid, Operator: "a",
	}); err != nil {
		t.Fatal(err)
	}

	// 未确认 → 400
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/import/reset", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := post(`{"confirm":false}`); w.Code != http.StatusBadRequest {
		t.Fatalf("未确认应 400，实际 %d %s", w.Code, w.Body.String())
	}
	// 确认 → 清空（自动备份 + audit）
	if w := post(`{"confirm":true}`); w.Code != http.StatusOK {
		t.Fatalf("reset 失败: %d %s", w.Code, w.Body.String())
	}
	var n int64
	db.Model(&models.Equipment{}).Count(&n)
	if n != 0 {
		t.Fatalf("reset 后设备应为 0，实际 %d", n)
	}
	var audit int64
	db.Model(&models.AuditLog{}).Where("action = ?", "IMPORT_RESET").Count(&audit)
	if audit != 1 {
		t.Fatalf("应写 1 条 IMPORT_RESET audit，实际 %d", audit)
	}
	// 字典保留
	var cat int64
	db.Model(&models.Category{}).Count(&cat)
	if cat != 1 {
		t.Fatalf("reset 应保留类别字典，实际 %d", cat)
	}
	// 全量重导（真实文件）→ 2147 台
	data, err := os.ReadFile(filepath.Join(repoRootForImportTest, "设备借出总账.xlsx"))
	if err != nil {
		t.Fatalf("读取 Excel 失败: %v", err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "设备借出总账.xlsx")
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/import/parse", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("parse 失败: %d %s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		ParseID string `json:"parse_id"`
		Preview struct {
			Reviews []struct {
				Key string `json:"key"`
			} `json:"reviews"`
			Suspected []struct {
				SourceKey string `json:"source_key"`
			} `json:"suspected"`
		} `json:"preview"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	var ack, sus []string
	for _, rv := range parsed.Preview.Reviews {
		ack = append(ack, rv.Key)
	}
	for _, s := range parsed.Preview.Suspected {
		sus = append(sus, s.SourceKey)
	}
	runBody, _ := json.Marshal(map[string]any{
		"parse_id": parsed.ParseID, "confirmed_suspects": sus, "acknowledged_reviews": ack,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/import/run", bytes.NewReader(runBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("重导失败: %d %s", rec2.Code, rec2.Body.String())
	}
	var rep struct {
		Report struct {
			Imported int `json:"imported"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Report.Imported != 2147 {
		t.Fatalf("清空重导应导入 2147 台，实际 %d", rep.Report.Imported)
	}
	// 清理：关闭连接释放文件锁；删除 reset 在 cwd backup/ 生成的备份（测试隔离）
	if err := sqlDB.Close(); err != nil {
		t.Logf("关闭测试库失败: %v", err)
	}
	cleanTestBackups(t)
}

// cleanTestBackups 移除测试运行期间在 cwd backup/ 新建的备份文件（reset 内部自动备份）。
func cleanTestBackups(t *testing.T) {
	t.Helper()
	dir := filepath.Join(".", "backup")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) < 2*time.Minute {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
