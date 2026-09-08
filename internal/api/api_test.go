package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"equipment/internal/database"

	"github.com/gin-gonic/gin"
)

// newTestEngine 构造带内存库的 gin 引擎。
func newTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	sqlDB, err := database.SQLDB(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(sqlDB); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return New(db, "test-version")
}

func doGet(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestHealthOK 健康检查应 200 且报告 db=up。
func TestHealthOK(t *testing.T) {
	r := newTestEngine(t)
	w := doGet(t, r, "/api/health")
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Status  string `json:"status"`
		DB      string `json:"db"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.DB != "up" || body.Version != "test-version" {
		t.Fatalf("响应内容异常: %+v", body)
	}
}

// TestNotFoundJSON 未知 /api 路径应返回统一 JSON 错误（而非 HTML 404）。
func TestNotFoundJSON(t *testing.T) {
	r := newTestEngine(t)
	w := doGet(t, r, "/api/no-such")
	if w.Code != http.StatusNotFound {
		t.Fatalf("期望 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "error") {
		t.Fatalf("应为统一错误 JSON: %s", w.Body.String())
	}
}

// TestStaticIndex 根路径应返回前端页面（占位 index.html 存在）。
func TestStaticIndex(t *testing.T) {
	r := newTestEngine(t)
	w := doGet(t, r, "/")
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "设备资产与流转管理系统") {
		t.Fatalf("首页内容异常: %s", w.Body.String()[:min(len(w.Body.String()), 200)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
