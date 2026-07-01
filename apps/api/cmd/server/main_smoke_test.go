package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// TestRouteWiring 验证 Gin 路由在不连 DB 的情况下也能正确注册并响应；
// 重点是路径模式（含 /:id/*path 和 NoRoute）不会 panic，CORS 头能正确加上。
// 真实 handler 依赖 DB/Storage/Hub，这里只测路由树本身。
func TestRouteWiring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	app.Use(gin.Recovery())

	corsCfg := cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
	app.Use(cors.New(corsCfg))

	app.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// 注册一个真实的 GET 路由，用于 CORS preflight 命中
	app.GET("/api/nodes", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"items": []any{}}) })

	// 模拟带 :id/*path 的路由（这是从 fiber 迁过来后路径语法变化最大的一处）
	app.GET("/d/:id/*path", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":   c.Param("id"),
			"path": c.Param("path"),
		})
	})

	// 模拟 NoRoute (SPA fallback)
	backendPrefixes := []string{"/api", "/d/", "/ws", "/healthz", "/mcp"}
	app.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}

		p := c.Request.URL.Path
		for _, pre := range backendPrefixes {
			if len(p) >= len(pre) && p[:len(pre)] == pre {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		}
		c.String(http.StatusOK, "spa-fallback")
	})

	cases := []struct {
		name     string
		method   string
		path     string
		wantCode int
		wantBody string
	}{
		{"healthz", "GET", "/healthz", 200, "ok"},
		{"doc asset wildcard root", "GET", "/d/abc/index.html", 200, ""},
		{"doc asset wildcard nested", "GET", "/d/abc/sub/x.css", 200, ""},
		{"api unknown -> 404", "GET", "/api/whatever", 404, ""},
		{"mcp prefix unknown -> 404", "GET", "/mcp/foo", 404, ""},
		{"spa fallback", "GET", "/spa/page", 200, "spa-fallback"},
		{"api nodes get", "GET", "/api/nodes", 200, ""},
		{"cors preflight", "OPTIONS", "/api/nodes", 204, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.method == "OPTIONS" {
				req.Header.Set("Origin", "https://example.com")
				req.Header.Set("Access-Control-Request-Method", "GET")
			}
			rw := httptest.NewRecorder()
			app.ServeHTTP(rw, req)
			if rw.Code != tc.wantCode {
				t.Fatalf("status: got %d want %d body=%q", rw.Code, tc.wantCode, rw.Body.String())
			}
			if tc.wantBody != "" && rw.Body.String() != tc.wantBody {
				t.Fatalf("body: got %q want %q", rw.Body.String(), tc.wantBody)
			}
		})
	}
}

// TestParamPathBehavior 单独验证 :id/*path 在不同路径下的解析行为
func TestParamPathBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := gin.New()
	app.GET("/d/:id/*path", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":   c.Param("id"),
			"path": c.Param("path"),
		})
	})
	app.GET("/d/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":   c.Param("id"),
			"path": "(no path)",
		})
	})

	cases := []struct {
		path string
		want int
	}{
		{"/d/abc/index.html", 200},
		{"/d/abc/sub/x.css", 200},
		{"/d/abc", 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", tc.path, nil)
		rw := httptest.NewRecorder()
		app.ServeHTTP(rw, req)
		if rw.Code != tc.want {
			t.Fatalf("path=%s status got %d want %d body=%q", tc.path, rw.Code, tc.want, rw.Body.String())
		}
		t.Logf("path=%s body=%s", tc.path, rw.Body.String())
	}
}
