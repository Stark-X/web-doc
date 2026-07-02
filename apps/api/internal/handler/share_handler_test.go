package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xiaofengguo/web-doc/api/internal/db"
	"github.com/xiaofengguo/web-doc/api/internal/model"
	"github.com/xiaofengguo/web-doc/api/internal/storage"
)

func TestCreateShareIncludesStaticShareBaseURL(t *testing.T) {
	h, app := newShareTestHandler(t)
	h.ShareBaseURL = "https://share.example.com/p"
	insertShareTestDoc(t, h, "doc-1")

	first := postShare(t, app, "/api/docs/doc-1/share")
	if first.StaticShareBaseURL != "https://share.example.com/p" {
		t.Fatalf("StaticShareBaseURL = %q", first.StaticShareBaseURL)
	}

	second := postShare(t, app, "/api/docs/doc-1/share")
	if second.Token != first.Token {
		t.Fatalf("second token = %q, want reused token %q", second.Token, first.Token)
	}
	if second.StaticShareBaseURL != "https://share.example.com/p" {
		t.Fatalf("reused StaticShareBaseURL = %q", second.StaticShareBaseURL)
	}
}

func TestCreateShareOmitsStaticShareBaseURLWhenUnset(t *testing.T) {
	h, app := newShareTestHandler(t)
	insertShareTestDoc(t, h, "doc-1")

	req := httptest.NewRequest(http.MethodPost, "/api/docs/doc-1/share", nil)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status got %d body=%q", rw.Code, rw.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rw.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["staticShareBaseUrl"]; ok {
		t.Fatalf("staticShareBaseUrl should be omitted when unset: %v", body)
	}
}

type shareTestResponse struct {
	ID                 string `json:"id"`
	DocID              string `json:"docId"`
	Token              string `json:"token"`
	StaticShareBaseURL string `json:"staticShareBaseUrl,omitempty"`
}

func newShareTestHandler(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	d, err := db.Open("sqlite", filepath.Join(t.TempDir(), "webdoc.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	st, err := storage.New(filepath.Join(t.TempDir(), "docs"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	h := New(d, st, nil)

	app := gin.New()
	app.POST("/api/docs/:id/share", h.CreateShare)
	return h, app
}

func insertShareTestDoc(t *testing.T, h *Handler, id string) {
	t.Helper()
	n := model.Node{
		ID:         id,
		Type:       "doc",
		Title:      "Doc",
		EntryFile:  "index.html",
		Visibility: "private",
	}
	if err := h.DB.Create(&n).Error; err != nil {
		t.Fatalf("insert doc: %v", err)
	}
}

func postShare(t *testing.T, app *gin.Engine, path string) shareTestResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rw := httptest.NewRecorder()
	app.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("status got %d body=%q", rw.Code, rw.Body.String())
	}
	var got shareTestResponse
	if err := json.Unmarshal(rw.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Token == "" {
		t.Fatal("expected token")
	}
	return got
}
