package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/app/api/handlers"
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
