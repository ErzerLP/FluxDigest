package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
