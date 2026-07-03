package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaofengguo/web-doc/api/internal/auth"
	"github.com/xiaofengguo/web-doc/api/internal/db"
	"github.com/xiaofengguo/web-doc/api/internal/handler"
	"github.com/xiaofengguo/web-doc/api/internal/model"
	"github.com/xiaofengguo/web-doc/api/internal/storage"
	"github.com/xiaofengguo/web-doc/api/internal/watcher"
)

const testJWTSecret = "test-secret"

// newTestApp 用真实的 registerRoutes 接线搭建 app，确保测试覆盖的就是生产路由的鉴权边界。
func newTestApp(t *testing.T) (*gin.Engine, *handler.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	d, err := db.Open("sqlite", filepath.Join(dir, "webdoc.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	st, err := storage.New(filepath.Join(dir, "docs"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	hub, err := watcher.NewHub(filepath.Join(dir, "docs"))
	if err != nil {
		t.Fatalf("new hub: %v", err)
	}
	h := handler.New(d, st, hub)
	h.JWTSecret = testJWTSecret

	app := gin.New()
	registerRoutes(app, h)
	return app, h
}

func seedDoc(t *testing.T, h *handler.Handler, id string) {
	t.Helper()
	if err := h.Storage.CreateDoc(id, "<h1>seed</h1>"); err != nil {
		t.Fatalf("create doc dir: %v", err)
	}
	n := model.Node{
		ID:         id,
		Type:       "doc",
		Title:      "Seed Doc",
		EntryFile:  "index.html",
		Visibility: "private",
	}
	if err := h.DB.Create(&n).Error; err != nil {
		t.Fatalf("insert doc: %v", err)
	}
}

func validToken(t *testing.T) string {
	t.Helper()
	tok, err := auth.SignToken(testJWTSecret, auth.Claims{
		Sub: "u1", Username: "tester", Role: "admin",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

func doJSON(app *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	return rw
}

// TestMutatingEndpointsRequireAuth 未登录（无 token / 伪造 token）访问写接口与全量列表必须 401。
func TestMutatingEndpointsRequireAuth(t *testing.T) {
	app, h := newTestApp(t)
	seedDoc(t, h, "doc-1")

	cases := []struct {
		method, path, body string
	}{
		{"GET", "/api/nodes", ""},
		{"POST", "/api/nodes", `{"type":"doc","title":"x"}`},
		{"PATCH", "/api/nodes/doc-1", `{"title":"hacked"}`},
		{"DELETE", "/api/nodes/doc-1", ""},
		{"PATCH", "/api/nodes/reorder/batch", `{"items":[]}`},
		{"POST", "/api/docs/doc-1/html", `{"html":"<h1>hacked</h1>"}`},
		{"POST", "/api/docs/doc-1/zip", ""},
		{"POST", "/api/docs/doc-1/file", `{"path":"index.html","content":"hacked"}`},
		{"POST", "/api/docs/doc-1/share", ""},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			if rw := doJSON(app, tc.method, tc.path, tc.body, ""); rw.Code != http.StatusUnauthorized {
				t.Fatalf("no token: got %d want 401 body=%q", rw.Code, rw.Body.String())
			}
			if rw := doJSON(app, tc.method, tc.path, tc.body, "forged.token.value"); rw.Code != http.StatusUnauthorized {
				t.Fatalf("forged token: got %d want 401 body=%q", rw.Code, rw.Body.String())
			}
		})
	}

	// 未授权请求不得产生副作用：文档内容与记录保持原样
	var n model.Node
	if err := h.DB.First(&n, "id = ?", "doc-1").Error; err != nil {
		t.Fatalf("doc-1 should still exist: %v", err)
	}
	if n.Title != "Seed Doc" {
		t.Fatalf("title mutated: %q", n.Title)
	}
}

// TestMutatingEndpointsWorkWithValidToken 合法登录态可正常写入。
func TestMutatingEndpointsWorkWithValidToken(t *testing.T) {
	app, h := newTestApp(t)
	seedDoc(t, h, "doc-1")
	tok := validToken(t)

	if rw := doJSON(app, "GET", "/api/nodes", "", tok); rw.Code != http.StatusOK {
		t.Fatalf("list nodes: got %d body=%q", rw.Code, rw.Body.String())
	}
	if rw := doJSON(app, "POST", "/api/nodes", `{"type":"doc","title":"new"}`, tok); rw.Code != http.StatusOK {
		t.Fatalf("create node: got %d body=%q", rw.Code, rw.Body.String())
	}
	if rw := doJSON(app, "PATCH", "/api/nodes/doc-1", `{"title":"renamed"}`, tok); rw.Code != http.StatusOK {
		t.Fatalf("update node: got %d body=%q", rw.Code, rw.Body.String())
	}
	if rw := doJSON(app, "POST", "/api/docs/doc-1/file", `{"path":"index.html","content":"<h1>ok</h1>"}`, tok); rw.Code != http.StatusOK {
		t.Fatalf("save file: got %d body=%q", rw.Code, rw.Body.String())
	}
	if rw := doJSON(app, "DELETE", "/api/nodes/doc-1", "", tok); rw.Code != http.StatusOK {
		t.Fatalf("delete node: got %d body=%q", rw.Code, rw.Body.String())
	}
	var n model.Node
	if err := h.DB.First(&n, "id = ?", "doc-1").Error; err == nil {
		t.Fatal("doc-1 should be deleted")
	}
}

// TestShareVisitorEndpointsStayAnonymous 分享访客依赖的只读端点必须保持匿名可访问。
func TestShareVisitorEndpointsStayAnonymous(t *testing.T) {
	app, h := newTestApp(t)
	seedDoc(t, h, "doc-1")
	share := model.Share{
		ID: "s1", DocID: "doc-1", Token: "tok123", CreatedAt: time.Now(),
	}
	if err := h.DB.Create(&share).Error; err != nil {
		t.Fatalf("insert share: %v", err)
	}

	cases := []struct {
		path string
		want int
	}{
		{"/api/auth/public-info", http.StatusOK},
		{"/api/shares/tok123", http.StatusOK},
		{"/api/nodes/doc-1", http.StatusOK},
		{"/api/docs/doc-1/file?path=index.html", http.StatusOK},
		{"/d/doc-1/index.html", http.StatusOK},
		{"/p/tok123/index.html", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rw := doJSON(app, "GET", tc.path, "", "")
			if rw.Code != tc.want {
				t.Fatalf("got %d want %d body=%q", rw.Code, tc.want, rw.Body.String())
			}
		})
	}
}
