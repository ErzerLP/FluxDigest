package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
