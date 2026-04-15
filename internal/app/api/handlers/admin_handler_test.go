package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"rss-platform/internal/app/api/handlers"
	"rss-platform/internal/app/api/middleware"
	"rss-platform/internal/domain/admin"
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

type adminConfigReaderStub struct {
	snapshot service.AdminConfigSnapshot
	err      error
}

func (s adminConfigReaderStub) GetSnapshot(_ context.Context) (service.AdminConfigSnapshot, error) {
	if s.err != nil {
		return service.AdminConfigSnapshot{}, s.err
	}
	return s.snapshot, nil
}

type adminConfigUpdaterStub struct {
	llmVersion      profile.Version
	llmErr          error
	minifluxVersion profile.Version
	minifluxErr     error
	publishVersion  profile.Version
	publishErr      error
	schedulerVersion profile.Version
	schedulerErr     error
	promptsVersion  profile.Version
	promptsErr      error
}

func (s adminConfigUpdaterStub) UpdateLLM(_ context.Context, _ service.UpdateLLMConfigInput) (profile.Version, error) {
	if s.llmErr != nil {
		return profile.Version{}, s.llmErr
	}
	return s.llmVersion, nil
}

func (s adminConfigUpdaterStub) UpdateMiniflux(_ context.Context, _ service.UpdateMinifluxConfigInput) (profile.Version, error) {
	if s.minifluxErr != nil {
		return profile.Version{}, s.minifluxErr
	}
	return s.minifluxVersion, nil
}

func (s adminConfigUpdaterStub) UpdatePublish(_ context.Context, _ service.UpdatePublishConfigInput) (profile.Version, error) {
	if s.publishErr != nil {
		return profile.Version{}, s.publishErr
	}
	return s.publishVersion, nil
}

func (s adminConfigUpdaterStub) UpdateScheduler(_ context.Context, _ service.UpdateSchedulerConfigInput) (profile.Version, error) {
	if s.schedulerErr != nil {
		return profile.Version{}, s.schedulerErr
	}
	return s.schedulerVersion, nil
}

func (s adminConfigUpdaterStub) UpdatePrompts(_ context.Context, _ service.UpdatePromptConfigInput) (profile.Version, error) {
	if s.promptsErr != nil {
		return profile.Version{}, s.promptsErr
	}
	return s.promptsVersion, nil
}

type adminConnectivityTesterStub struct {
	llmResult      service.ConnectivityTestResult
	llmErr         error
	minifluxResult service.ConnectivityTestResult
	minifluxErr    error
	publishResult  service.ConnectivityTestResult
	publishErr     error
}

func (s adminConnectivityTesterStub) TestLLM(_ context.Context, _ service.LLMTestDraft) (service.ConnectivityTestResult, error) {
	return s.llmResult, s.llmErr
}

func (s adminConnectivityTesterStub) TestMiniflux(_ context.Context) (service.ConnectivityTestResult, error) {
	return s.minifluxResult, s.minifluxErr
}

func (s adminConnectivityTesterStub) TestPublish(_ context.Context) (service.ConnectivityTestResult, error) {
	return s.publishResult, s.publishErr
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

type adminJobTriggerStub struct {
	result service.JobTriggerResult
	err    error
	force  bool
}

func (s *adminJobTriggerStub) TriggerDailyDigest(_ context.Context, _ time.Time) (service.JobTriggerResult, error) {
	return s.result, s.err
}

func (s *adminJobTriggerStub) TriggerDailyDigestWithOptions(_ context.Context, _ time.Time, opts service.DailyDigestTriggerOptions) (service.JobTriggerResult, error) {
	s.force = opts.Force
	return s.result, s.err
}

func (s *adminJobTriggerStub) TriggerArticleReprocess(_ context.Context, _ string, _ bool) (service.JobTriggerResult, error) {
	return service.JobTriggerResult{}, nil
}

type adminAuthRepoRouteStub struct {
	user admin.User
}

func (s *adminAuthRepoRouteStub) Create(context.Context, admin.User) error {
	return nil
}

func (s *adminAuthRepoRouteStub) FindByUsername(_ context.Context, username string) (admin.User, error) {
	if username != s.user.Username {
		return admin.User{}, admin.ErrNotFound
	}
	return s.user, nil
}

type adminAuthServiceStub struct {
	loginResult  service.LoginResult
	loginSession string
	loginErr     error
	currentUser  service.LoginResult
	currentErr   error
	logoutErr    error
	sessionTTL   time.Duration
}

func (s adminAuthServiceStub) Login(_ context.Context, _ service.LoginInput) (service.LoginResult, string, error) {
	return s.loginResult, s.loginSession, s.loginErr
}

func (s adminAuthServiceStub) Logout(_ context.Context, _ string) error {
	return s.logoutErr
}

func (s adminAuthServiceStub) CurrentUser(_ context.Context, _ string) (service.LoginResult, error) {
	if s.currentErr != nil {
		return service.LoginResult{}, s.currentErr
	}
	return s.currentUser, nil
}

func (s adminAuthServiceStub) SessionTTL() time.Duration {
	if s.sessionTTL > 0 {
		return s.sessionTTL
	}
	return time.Hour
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
		ConfigUpdater: adminConfigUpdaterStub{llmVersion: profile.Version{
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

func TestAdminConfigsRouteReturnsFullSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		Configs: adminConfigReaderStub{snapshot: service.AdminConfigSnapshot{
			LLM: service.LLMConfigView{
				BaseURL:   "https://llm.local/v1",
				Model:     "gpt-4.1",
				TimeoutMS: 60000,
				APIKey:    service.SecretView{IsSet: true, MaskedValue: "abcd****"},
			},
			Miniflux: service.MinifluxConfigView{
				BaseURL:       "https://miniflux.local",
				FetchLimit:    30,
				LookbackHours: 24,
				APIToken:      service.SecretView{IsSet: true, MaskedValue: "mflt****"},
			},
			Publish: service.PublishConfigView{
				Provider:    "halo",
				HaloBaseURL: "https://halo.local",
				HaloToken:   service.SecretView{IsSet: true, MaskedValue: "halo****"},
				OutputDir:   "/tmp/publish",
				ArticlePublishMode: "suggested",
				ArticleReviewMode:  "manual_review",
			},
			Scheduler: service.SchedulerConfigView{
				Enabled:      true,
				ScheduleTime: "07:30",
				Timezone:     "Asia/Shanghai",
			},
			Prompts: service.PromptConfigView{
				TargetLanguage:    "zh-CN",
				TranslationPrompt: "translate",
				AnalysisPrompt:    "analyze",
				DossierPrompt:     "dossier",
				DigestPrompt:      "digest",
			},
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/configs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	for _, key := range []string{"llm", "miniflux", "publish", "scheduler", "prompts"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("want snapshot key %q in response: %#v", key, body)
		}
	}
}

func TestAdminUpdateMinifluxRouteReturnsProfileVersionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		ConfigUpdater: adminConfigUpdaterStub{minifluxVersion: profile.Version{
			ID:          "ver-miniflux-1",
			ProfileType: profile.TypeMiniflux,
			Name:        "admin-miniflux",
			Version:     2,
			IsActive:    true,
		}},
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/configs/miniflux",
		bytes.NewBufferString(`{"base_url":"https://miniflux.local","fetch_limit":30,"lookback_hours":48,"api_token":{"mode":"keep"}}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["profile_type"] != profile.TypeMiniflux {
		t.Fatalf("want profile_type %q got %#v", profile.TypeMiniflux, body["profile_type"])
	}
}

func TestAdminUpdatePublishRouteReturnsProfileVersionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		ConfigUpdater: adminConfigUpdaterStub{publishVersion: profile.Version{
			ID:          "ver-publish-1",
			ProfileType: profile.TypePublish,
			Name:        "admin-publish",
			Version:     4,
			IsActive:    true,
		}},
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/configs/publish",
		bytes.NewBufferString(`{"provider":"halo","halo_base_url":"https://halo.local","halo_token":{"mode":"keep"},"output_dir":"/tmp/publish"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["profile_type"] != profile.TypePublish {
		t.Fatalf("want profile_type %q got %#v", profile.TypePublish, body["profile_type"])
	}
}

func TestAdminUpdateSchedulerRouteReturnsProfileVersionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		ConfigUpdater: adminConfigUpdaterStub{schedulerVersion: profile.Version{
			ID:          "ver-scheduler-1",
			ProfileType: profile.TypeScheduler,
			Name:        "admin-scheduler",
			Version:     2,
			IsActive:    true,
		}},
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/configs/scheduler",
		bytes.NewBufferString(`{"enabled":true,"schedule_time":"08:00","timezone":"Asia/Shanghai"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["profile_type"] != profile.TypeScheduler {
		t.Fatalf("want profile_type %q got %#v", profile.TypeScheduler, body["profile_type"])
	}
}

func TestAdminRunDailyDigestRouteSupportsForce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	trigger := &adminJobTriggerStub{result: service.JobTriggerResult{DigestDate: "2026-04-15", Status: "accepted"}}
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		JobTrigger: trigger,
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/jobs/daily-digest/run",
		bytes.NewBufferString(`{"force":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202 got %d body=%s", rec.Code, rec.Body.String())
	}
	if !trigger.force {
		t.Fatal("expected force flag forwarded to trigger")
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "accepted" {
		t.Fatalf("want status accepted got %#v", body["status"])
	}
	if body["digest_date"] != "2026-04-15" {
		t.Fatalf("want digest_date 2026-04-15 got %#v", body["digest_date"])
	}
	if body["force"] != true {
		t.Fatalf("want force=true got %#v", body["force"])
	}
}

func TestAdminUpdatePromptsRouteReturnsProfileVersionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		ConfigUpdater: adminConfigUpdaterStub{promptsVersion: profile.Version{
			ID:          "ver-prompts-1",
			ProfileType: profile.TypePrompts,
			Name:        "admin-prompts",
			Version:     5,
			IsActive:    true,
		}},
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/configs/prompts",
		bytes.NewBufferString(`{"target_language":"zh-CN","translation_prompt":"translate","analysis_prompt":"analyze","dossier_prompt":"dossier","digest_prompt":"digest"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["profile_type"] != profile.TypePrompts {
		t.Fatalf("want profile_type %q got %#v", profile.TypePrompts, body["profile_type"])
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

func TestAdminMinifluxTestRouteReturnsConnectivityJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		Tester: adminConnectivityTesterStub{
			minifluxResult: service.ConnectivityTestResult{Status: "ok", Message: "connection succeeded", LatencyMS: 123},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/test/miniflux", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestAdminPublishTestRouteReturnsConnectivityJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers.RegisterAdminRoutes(router.Group("/api/v1"), handlers.AdminDeps{
		Tester: adminConnectivityTesterStub{
			publishResult: service.ConnectivityTestResult{Status: "ok", Message: "connection succeeded", LatencyMS: 88},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/test/publish", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"latency_ms":88`)) {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
}

func TestAdminAuthRoutesLoginMeLogoutAndProtectAdminEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("FluxDigest"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	authService := service.NewAdminAuthService(&adminAuthRepoRouteStub{
		user: admin.User{
			ID:                 "admin-1",
			Username:           "FluxDigest",
			PasswordHash:       string(passwordHash),
			MustChangePassword: true,
		},
	}, service.NewInMemoryAdminSessionStore())

	router := gin.New()
	sessionMiddleware := middleware.RequireAdminSession(authService, middleware.AdminSessionOptions{
		CookieName: service.DefaultAdminSessionCookieName,
	})
	handlers.RegisterAdminAuthRoutes(router.Group("/api/v1/admin/auth"), handlers.AdminAuthDeps{
		Auth:        authService,
		CookieName:  service.DefaultAdminSessionCookieName,
		CookiePath:  "/",
		SessionAuth: sessionMiddleware,
	})
	adminGroup := router.Group("/api/v1/admin")
	adminGroup.Use(sessionMiddleware)
	handlers.RegisterAdminRoutes(adminGroup, handlers.AdminDeps{
		Status: adminStatusReaderStub{view: service.AdminStatusView{
			Runtime: service.RuntimeStatusView{
				LatestDigestDate: "2026-04-11",
				LatestJobStatus:  "succeeded",
			},
		}},
	})

	unauthorizedReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	unauthorizedRec := httptest.NewRecorder()
	router.ServeHTTP(unauthorizedRec, unauthorizedReq)
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("want unauthorized 401 got %d body=%s", unauthorizedRec.Code, unauthorizedRec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", bytes.NewBufferString(`{"username":"FluxDigest","password":"FluxDigest"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("want login 200 got %d body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("want session cookie")
	}
	sessionCookie := cookies[0]
	if sessionCookie.Name != service.DefaultAdminSessionCookieName {
		t.Fatalf("want cookie %q got %q", service.DefaultAdminSessionCookieName, sessionCookie.Name)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("want me 200 got %d body=%s", meRec.Code, meRec.Body.String())
	}
	if !bytes.Contains(meRec.Body.Bytes(), []byte(`"must_change_password":true`)) {
		t.Fatalf("unexpected me body %s", meRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	statusReq.AddCookie(sessionCookie)
	statusRec := httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("want status 200 got %d body=%s", statusRec.Code, statusRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("want logout 204 got %d body=%s", logoutRec.Code, logoutRec.Body.String())
	}

	statusAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	statusAfterLogoutReq.AddCookie(sessionCookie)
	statusAfterLogoutRec := httptest.NewRecorder()
	router.ServeHTTP(statusAfterLogoutRec, statusAfterLogoutReq)
	if statusAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("want status after logout 401 got %d body=%s", statusAfterLogoutRec.Code, statusAfterLogoutRec.Body.String())
	}
}

func TestAdminAuthLoginSetsSecureCookieWhenForwardedHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlers.RegisterAdminAuthRoutes(router.Group("/api/v1/admin/auth"), handlers.AdminAuthDeps{
		Auth: adminAuthServiceStub{
			loginResult:  service.LoginResult{UserID: "admin-1", Username: "FluxDigest"},
			loginSession: "session-1",
		},
		CookieName: service.DefaultAdminSessionCookieName,
		CookiePath: "/",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", bytes.NewBufferString(`{"username":"FluxDigest","password":"FluxDigest"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("want session cookie")
	}
	if !cookies[0].Secure {
		t.Fatalf("want secure cookie when forwarded https, got %#v", cookies[0])
	}
}

func TestAdminAuthLoginKeepsCookieUsableOnPlainHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlers.RegisterAdminAuthRoutes(router.Group("/api/v1/admin/auth"), handlers.AdminAuthDeps{
		Auth: adminAuthServiceStub{
			loginResult:  service.LoginResult{UserID: "admin-1", Username: "FluxDigest"},
			loginSession: "session-1",
		},
		CookieName: service.DefaultAdminSessionCookieName,
		CookiePath: "/",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", bytes.NewBufferString(`{"username":"FluxDigest","password":"FluxDigest"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("want session cookie")
	}
	if cookies[0].Secure {
		t.Fatalf("did not expect secure cookie on plain http, got %#v", cookies[0])
	}
}

func TestAdminAuthLoginRedactsInternalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handlers.RegisterAdminAuthRoutes(router.Group("/api/v1/admin/auth"), handlers.AdminAuthDeps{
		Auth: adminAuthServiceStub{
			loginErr: errors.New("redis dial tcp 127.0.0.1:6379: connection refused"),
		},
		CookieName: service.DefaultAdminSessionCookieName,
		CookiePath: "/",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", bytes.NewBufferString(`{"username":"FluxDigest","password":"FluxDigest"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d body=%s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("connection refused")) {
		t.Fatalf("unexpected leaked error body=%s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"error":"authentication service unavailable"`)) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestAdminSessionMiddlewareRedactsInternalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	sessionMiddleware := middleware.RequireAdminSession(adminAuthServiceStub{
		currentErr: errors.New("redis get session: connection refused"),
	}, middleware.AdminSessionOptions{
		CookieName: service.DefaultAdminSessionCookieName,
	})
	adminGroup := router.Group("/api/v1/admin")
	adminGroup.Use(sessionMiddleware)
	handlers.RegisterAdminRoutes(adminGroup, handlers.AdminDeps{
		Status: adminStatusReaderStub{view: service.AdminStatusView{}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/status", nil)
	req.AddCookie(&http.Cookie{Name: service.DefaultAdminSessionCookieName, Value: "session-1"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d body=%s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("connection refused")) {
		t.Fatalf("unexpected leaked error body=%s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"error":"authentication service unavailable"`)) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}
