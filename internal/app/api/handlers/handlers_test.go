package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	apiapp "rss-platform/internal/app/api"
	"rss-platform/internal/app/api/handlers"
	"rss-platform/internal/app/api/middleware"
	"rss-platform/internal/service"
)

type digestServiceStub struct{}

func (digestServiceStub) LatestDigest(context.Context) (service.DigestView, error) {
	return service.DigestView{Title: "今日 AI 日报"}, nil
}

type dossierReaderStub struct {
	items     []service.DossierListItem
	detail    service.DossierDetail
	listErr   error
	detailErr error
}

func (s dossierReaderStub) ListDossiers(context.Context, service.DossierListFilter) ([]service.DossierListItem, error) {
	return s.items, s.listErr
}

func (s dossierReaderStub) GetDossier(context.Context, string) (service.DossierDetail, error) {
	return s.detail, s.detailErr
}

type articleServiceStub struct{}

func (articleServiceStub) ListArticles(context.Context) ([]service.ArticleView, error) {
	return []service.ArticleView{{
		ID:              "art-1",
		Title:           "Model News",
		TitleTranslated: "模型新闻",
		CoreSummary:     "核心观点",
	}}, nil
}

type profileServiceStub struct{}

func (profileServiceStub) ActiveProfile(_ context.Context, profileType string) (service.ProfileView, error) {
	return service.ProfileView{ProfileType: profileType, Name: "default-" + profileType, Payload: map[string]any{}}, nil
}

type jobTriggerStub struct {
	calls   []time.Time
	results []service.JobTriggerResult
}

func (s *jobTriggerStub) TriggerDailyDigest(_ context.Context, now time.Time) (service.JobTriggerResult, error) {
	s.calls = append(s.calls, now)

	if len(s.results) == 0 {
		return service.JobTriggerResult{DigestDate: now.Format("2006-01-02"), Status: "accepted"}, nil
	}

	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

func TestLatestDigestRouteReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterDigestRoutes(router.Group("/api/v1"), digestServiceStub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/digests/latest", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["title"] != "今日 AI 日报" {
		t.Fatalf("want title %q got %#v", "今日 AI 日报", body["title"])
	}
	if _, ok := body["sections"]; ok {
		t.Fatalf("did not expect sections in response: %#v", body["sections"])
	}
}

func TestRegisterDossierRoutesReturnsListAndDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterDossierRoutes(router.Group("/api/v1"), dossierReaderStub{
		items: []service.DossierListItem{{
			ID:              "dos-1",
			TitleTranslated: "模型新闻",
			PublishState:    "suggested",
		}},
		detail: service.DossierDetail{
			ID:              "dos-1",
			TitleTranslated: "模型新闻",
			PublishState:    "suggested",
		},
	})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/dossiers", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", listRec.Code)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/dossiers/dos-1", nil)
	detailRec := httptest.NewRecorder()
	router.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", detailRec.Code)
	}
}

func TestRegisterDossierRoutesRejectsLimitAboveOneHundred(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterDossierRoutes(router.Group("/api/v1"), dossierReaderStub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dossiers?limit=101", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestRegisterDossierRoutesReturnsNotFoundForMissingDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterDossierRoutes(router.Group("/api/v1"), dossierReaderStub{detailErr: gorm.ErrRecordNotFound})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dossiers/missing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rec.Code)
	}
}

func TestArticleRouteWithoutReaderReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterArticleRoutes(router.Group("/api/v1"), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d", rec.Code)
	}
}

func TestDigestRouteWithoutReaderReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterDigestRoutes(router.Group("/api/v1"), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/digests/latest", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d", rec.Code)
	}
}

func TestProfileRouteWithoutReaderReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterProfileRoutes(router.Group("/api/v1"), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profiles/ai/active", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d", rec.Code)
	}
}

func TestJobRouteReturnsSkippedWhenDigestAlreadyQueued(t *testing.T) {
	gin.SetMode(gin.TestMode)
	trigger := &jobTriggerStub{results: []service.JobTriggerResult{{DigestDate: "2026-04-11", Status: "skipped"}}}
	router := gin.New()
	handlers.RegisterJobRoutes(router.Group("/api/v1"), trigger)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/daily-digest", bytes.NewBufferString(`{"trigger_at":"2026-04-11T07:00:00+08:00"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"status":"skipped"`)) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestRequireAPIKeyRejectsMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(middleware.RequireAPIKey("secret"))
	protected.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
	if rec.Body.String() != "{\"error\":\"invalid api key\"}" {
		t.Fatalf("want invalid api key body got %s", rec.Body.String())
	}
}

func TestNewRouterWithoutJobTriggerReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := apiapp.NewRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/daily-digest", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d", rec.Code)
	}
}

func TestRouterRegistersVersionedRoutesAndProtectsJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	trigger := &jobTriggerStub{}
	router := apiapp.NewRouter(
		apiapp.WithDigestReader(digestServiceStub{}),
		apiapp.WithDossierReader(dossierReaderStub{items: []service.DossierListItem{{ID: "dos-1", TitleTranslated: "模型新闻"}}, detail: service.DossierDetail{ID: "dos-1", TitleTranslated: "模型新闻"}}),
		apiapp.WithArticleReader(articleServiceStub{}),
		apiapp.WithProfileReader(profileServiceStub{}),
		apiapp.WithJobTrigger(trigger),
		apiapp.WithAPIKey("secret"),
	)

	latestReq := httptest.NewRequest(http.MethodGet, "/api/v1/digests/latest", nil)
	latestRec := httptest.NewRecorder()
	router.ServeHTTP(latestRec, latestReq)
	if latestRec.Code != http.StatusOK {
		t.Fatalf("want digest route 200 got %d", latestRec.Code)
	}

	dossierReq := httptest.NewRequest(http.MethodGet, "/api/v1/dossiers", nil)
	dossierRec := httptest.NewRecorder()
	router.ServeHTTP(dossierRec, dossierReq)
	if dossierRec.Code != http.StatusOK {
		t.Fatalf("want dossier route 200 got %d", dossierRec.Code)
	}

	unauthorizedReq := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/daily-digest", bytes.NewBufferString(`{}`))
	unauthorizedReq.Header.Set("Content-Type", "application/json")
	unauthorizedRec := httptest.NewRecorder()
	router.ServeHTTP(unauthorizedRec, unauthorizedReq)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("want unauthorized 401 got %d", unauthorizedRec.Code)
	}

	authorizedReq := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/daily-digest", bytes.NewBufferString(`{"trigger_at":"2026-04-10T07:00:00+08:00"}`))
	authorizedReq.Header.Set("Content-Type", "application/json")
	authorizedReq.Header.Set("X-API-Key", "secret")
	authorizedRec := httptest.NewRecorder()
	router.ServeHTTP(authorizedRec, authorizedReq)

	if authorizedRec.Code != http.StatusAccepted {
		t.Fatalf("want job route 202 got %d", authorizedRec.Code)
	}
	if !bytes.Contains(authorizedRec.Body.Bytes(), []byte(`"status":"accepted"`)) {
		t.Fatalf("want accepted body got %s", authorizedRec.Body.String())
	}
	if len(trigger.calls) != 1 {
		t.Fatalf("want 1 job trigger call got %d", len(trigger.calls))
	}
	if got := trigger.calls[0].Format(time.RFC3339); got != "2026-04-10T07:00:00+08:00" {
		t.Fatalf("want trigger time 2026-04-10T07:00:00+08:00 got %s", got)
	}
}
