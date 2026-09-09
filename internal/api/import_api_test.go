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
}
