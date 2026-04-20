package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/app/api/handlers"
	"rss-platform/internal/service"
)

type adminStatusStub struct{}

func (adminStatusStub) GetStatus(context.Context) (service.AdminStatusView, error) {
	return service.AdminStatusView{
		Runtime: service.RuntimeStatusView{
			LatestDigestDate: "2026-04-11",
			LatestJobStatus:  "succeeded",
		},
	}, nil
}

func TestDefaultRouterConfigDoesNotInjectPlaceholderReaders(t *testing.T) {
	cfg := defaultRouterConfig()

	if cfg.articleReader != nil {
		t.Fatal("want nil articleReader by default")
	}
	if cfg.digestReader != nil {
		t.Fatal("want nil digestReader by default")
	}
	if cfg.profileReader != nil {
		t.Fatal("want nil profileReader by default")
	}
	if cfg.metrics == nil {
		t.Fatal("want metrics initialized")
	}
}

func TestRouterExposesHealthz(t *testing.T) {
	router := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("want body %s got %s", `{"status":"ok"}`, rec.Body.String())
	}
}

func TestRouterRegistersAdminStatusRoute(t *testing.T) {
	router := NewRouter(WithAdminDeps(handlers.AdminDeps{Status: adminStatusStub{}}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	runtimeBody, ok := body["runtime"].(map[string]any)
	if !ok {
		t.Fatalf("want runtime object got %#v", body["runtime"])
	}
	if runtimeBody["latest_digest_date"] != "2026-04-11" {
		t.Fatalf("want latest_digest_date 2026-04-11 got %#v", runtimeBody["latest_digest_date"])
	}
}

func TestRouterServesStaticIndexForNonAPIRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><body>FluxDigest UI</body></html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	router := NewRouter(WithStaticDir(dir))

	req := httptest.NewRequest(http.MethodGet, "/configs/llm", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("FluxDigest UI")) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestRouterDoesNotServeStaticIndexForAPIBaseRoute(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rec.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("FluxDigest UI")) {
		t.Fatalf("unexpected html fallback %s", rec.Body.String())
	}
}

func TestRouterDoesNotServeStaticIndexForHealthzTrailingSlash(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("want non-html fallback status got %d", rec.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("FluxDigest UI")) {
		t.Fatalf("unexpected html fallback %s", rec.Body.String())
	}
}

func TestRouterDoesNotServeStaticIndexForUnknownNonGETRoute(t *testing.T) {
	router := newStaticRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/configs/llm", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rec.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("FluxDigest UI")) {
		t.Fatalf("unexpected html fallback %s", rec.Body.String())
	}
}

func newStaticRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><body>FluxDigest UI</body></html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	return NewRouter(WithStaticDir(dir))
}
