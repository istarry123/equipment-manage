package api

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// TestSPAFallbackDeepRoutes 前端在 v1.4 Phase 2 改为 BrowserRouter，
// 因此每个前端路由的「直接访问 / F5 刷新」都必须由后端 NoRoute 回退到 index.html，
// 否则深链接会 404。此处逐条覆盖 web/src/routes.tsx 登记的全部路径。
func TestSPAFallbackDeepRoutes(t *testing.T) {
	r := newTestEngine(t)
	paths := []string{
		"/", "/dashboard", "/equipment", "/teamview", "/flow", "/borrow",
		"/team", "/import", "/import-detail", "/backup", "/settings",
		"/not-a-registered-path",
	}
	for _, p := range paths {
		w := doGet(t, r, p)
		if w.Code != http.StatusOK {
			t.Errorf("%s 期望 200（SPA 回退）, got %d", p, w.Code)
			continue
		}
		if body := w.Body.String(); !strings.Contains(body, `<div id="root">`) {
			t.Errorf("%s 未回退到 index.html: %s", p, body[:min(len(body), 120)])
		}
	}
}

// TestAPINotSwallowedBySPAFallback /api 下的未知路径必须仍返回统一 JSON 404，
// 不能被 SPA 回退吞成 HTML（否则前端会把 HTML 当 JSON 解析，报错难排查）。
func TestAPINotSwallowedBySPAFallback(t *testing.T) {
	r := newTestEngine(t)
	for _, p := range []string{"/api/no-such", "/api/equipment/999999/detail"} {
		w := doGet(t, r, p)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s 期望 404, got %d", p, w.Code)
		}
		if strings.Contains(w.Body.String(), `<div id="root">`) {
			t.Errorf("%s 被 SPA 回退吞掉", p)
		}
	}
}

var assetRefPattern = regexp.MustCompile(`(?:src|href)="\./assets/([^"]+)"`)

// TestStaticAssetsServedFromEmbed index.html 引用的每个静态资源都必须能从内嵌 FS 命中。
// 资源名带构建哈希，故从 index.html 动态提取，避免硬编码文件名。
func TestStaticAssetsServedFromEmbed(t *testing.T) {
	r := newTestEngine(t)
	index := doGet(t, r, "/").Body.String()
	refs := assetRefPattern.FindAllStringSubmatch(index, -1)
	if len(refs) == 0 {
		t.Fatalf("index.html 未引用任何 ./assets/ 资源（构建产物异常）: %s", index)
	}
	for _, m := range refs {
		p := "/assets/" + m[1]
		w := doGet(t, r, p)
		if w.Code != http.StatusOK {
			t.Errorf("%s 期望 200, got %d", p, w.Code)
			continue
		}
		if w.Body.Len() == 0 {
			t.Errorf("%s 命中但内容为空", p)
		}
	}
}
