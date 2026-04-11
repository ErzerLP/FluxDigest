package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/app/api/handlers"
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/service"
)

type adminStatusReaderStub struct {
	view service.AdminStatusView
	err  error
}

func (s adminStatusReaderStub) GetStatus(_ context.Context) (service.AdminStatusView, error) {
	if s.err != nil {
		return service.AdminStatusView{}, s.err
	}
	return s.view, nil
}

type adminLLMUpdaterStub struct {
	version profile.Version
	err     error
}

func (s adminLLMUpdaterStub) UpdateLLM(_ context.Context, _ service.UpdateLLMConfigInput) (profile.Version, error) {
	if s.err != nil {
		return profile.Version{}, s.err
	}
	return s.version, nil
}

type adminJobReaderStub struct {
	items []service.JobRunRecord
	err   error
}

func (s adminJobReaderStub) ListLatest(_ context.Context, _ service.JobRunListFilter) ([]service.JobRunRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func TestAdminStatusRouteReturnsDashboardJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		Status: adminStatusReaderStub{view: service.AdminStatusView{
			Runtime: service.RuntimeStatusView{
				LatestDigestDate: "2026-04-11",
				LatestJobStatus:  "succeeded",
			},
		}},
	})

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
	if runtimeBody["latest_job_status"] != "succeeded" {
		t.Fatalf("want latest_job_status succeeded got %#v", runtimeBody["latest_job_status"])
	}
}

func TestAdminUpdateLLMRouteReturnsProfileVersionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		LLMUpdater: adminLLMUpdaterStub{version: profile.Version{
			ID:          "ver-1",
			ProfileType: profile.TypeLLM,
			Name:        "admin-llm",
			Version:     3,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://llm.local/v1"}`),
		}},
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/configs/llm",
		bytes.NewBufferString(`{"base_url":"https://llm.local/v1","model":"gpt-4.1","api_key":{"mode":"keep"}}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	if bytes.Contains(rec.Body.Bytes(), []byte(`PayloadJSON`)) {
		t.Fatalf("did not expect PayloadJSON in response: %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`payload_json`)) {
		t.Fatalf("did not expect payload_json in response: %s", rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["profile_type"] != profile.TypeLLM {
		t.Fatalf("want profile_type %q got %#v", profile.TypeLLM, body["profile_type"])
	}
	if _, ok := body["PayloadJSON"]; ok {
		t.Fatalf("did not expect PayloadJSON key in response: %#v", body)
	}
	if _, ok := body["payload_json"]; ok {
		t.Fatalf("did not expect payload_json key in response: %#v", body)
	}
}

func TestAdminJobsRouteRejectsNonPositiveLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		Jobs: adminJobReaderStub{},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/jobs?limit=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
}
