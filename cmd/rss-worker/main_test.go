package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"rss-platform/internal/config"
	"rss-platform/internal/domain/article"
	domaindossier "rss-platform/internal/domain/dossier"
	"rss-platform/internal/domain/processing"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/service"

	"gorm.io/gorm"

	"rss-platform/internal/adapter/miniflux"
	domaindigest "rss-platform/internal/domain/digest"
)

type entryListerStub struct {
	entries []miniflux.Entry
}

func (s entryListerStub) ListEntries(_ context.Context, _, _ time.Time) ([]miniflux.Entry, error) {
	return s.entries, nil
}

type articleFinderStub struct {
	article article.SourceArticle
}

func (s articleFinderStub) FindByMinifluxEntryID(_ context.Context, _ int64) (article.SourceArticle, error) {
	return s.article, nil
}

func (s articleFinderStub) FindByID(_ context.Context, _ string) (article.SourceArticle, error) {
	return s.article, nil
}

type processingServiceStub struct {
	called int
	result processing.ProcessedArticle
}

func (s *processingServiceStub) ProcessArticle(_ context.Context, _ article.SourceArticle) (processing.ProcessedArticle, error) {
	s.called++
	return s.result, nil
}

type processingStoreStub struct {
	record       postgres.ProcessedArticleRecord
	err          error
	getLatestHit int
	saveHit      int
}

func (s *processingStoreStub) GetLatestByArticleID(_ context.Context, _ string) (postgres.ProcessedArticleRecord, error) {
	s.getLatestHit++
	return s.record, s.err
}

func (s *processingStoreStub) Save(_ context.Context, _ postgres.ProcessedArticleRecord) error {
	s.saveHit++
	return nil
}

type dossierMaterializerStub struct {
	dossier domaindossier.ArticleDossier
}

func (s dossierMaterializerStub) Materialize(_ context.Context, _ service.MaterializeDossierInput) (domaindossier.ArticleDossier, error) {
	return s.dossier, nil
}

type chatGenerateResult struct {
	message *schema.Message
	err     error
}

type chatModelStub struct {
	results []chatGenerateResult
	calls   int
}

type runtimeTaskUnitStub struct {
	runCalls         int
	reprocessCalls   int
	runResult        service.RunResult
	runErr           error
	reprocessErr     error
	gotDigestDates   []string
	gotDigestForces  []bool
	gotArticleIDs    []string
	gotArticleForces []bool
}

func (s *chatModelStub) Generate(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.Message, error) {
	s.calls++
	idx := s.calls - 1
	if idx >= len(s.results) {
		return nil, errors.New("unexpected generate call")
	}
	return s.results[idx].message, s.results[idx].err
}

func (s *chatModelStub) Stream(_ context.Context, _ []*schema.Message, _ ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not implemented")
}

func (s *runtimeTaskUnitStub) Run(_ context.Context, digestDate string, _ time.Time, opts ...service.RunOptions) (service.RunResult, error) {
	s.runCalls++
	s.gotDigestDates = append(s.gotDigestDates, digestDate)
	force := false
	if len(opts) > 0 {
		force = opts[len(opts)-1].Force
	}
	s.gotDigestForces = append(s.gotDigestForces, force)
	return s.runResult, s.runErr
}

func (s *runtimeTaskUnitStub) ReprocessArticle(_ context.Context, articleID string, force bool) error {
	s.reprocessCalls++
	s.gotArticleIDs = append(s.gotArticleIDs, articleID)
	s.gotArticleForces = append(s.gotArticleForces, force)
	return s.reprocessErr
}

func TestLoadDefaultPromptTemplatesIgnoresWorkingDirectory(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if chdirErr := os.Chdir(oldWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	}()

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	translationTemplate, analysisTemplate, _, _, err := loadDefaultPromptTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if translationTemplate == "" {
		t.Fatal("want translation template content")
	}
	if analysisTemplate == "" {
		t.Fatal("want analysis template content")
	}
}

func TestRefreshingRuntimeExecutorRunBuildsFreshRuntimePerInvocation(t *testing.T) {
	first := &runtimeTaskUnitStub{
		runResult: service.RunResult{DigestDate: "2026-04-15", RemoteURL: "/archives/first"},
	}
	second := &runtimeTaskUnitStub{
		runResult: service.RunResult{DigestDate: "2026-04-16", RemoteURL: "/archives/second"},
	}

	buildCalls := 0
	executor := refreshingRuntimeExecutor{
		builder: runtimeTaskUnitBuilderFunc(func(_ context.Context) (runtimeTaskUnit, error) {
			buildCalls++
			if buildCalls == 1 {
				return first, nil
			}
			return second, nil
		}),
	}

	gotFirst, err := executor.Run(context.Background(), "2026-04-15", time.Now(), service.RunOptions{Force: true})
	if err != nil {
		t.Fatalf("Run() first error = %v", err)
	}
	gotSecond, err := executor.Run(context.Background(), "2026-04-16", time.Now(), service.RunOptions{Force: false})
	if err != nil {
		t.Fatalf("Run() second error = %v", err)
	}

	if buildCalls != 2 {
		t.Fatalf("builder calls = %d, want 2", buildCalls)
	}
	if first.runCalls != 1 || second.runCalls != 1 {
		t.Fatalf("run calls = first:%d second:%d, want 1/1", first.runCalls, second.runCalls)
	}
	if len(first.gotDigestForces) != 1 || !first.gotDigestForces[0] {
		t.Fatalf("first force flags = %#v, want [true]", first.gotDigestForces)
	}
	if len(second.gotDigestForces) != 1 || second.gotDigestForces[0] {
		t.Fatalf("second force flags = %#v, want [false]", second.gotDigestForces)
	}
	if gotFirst.RemoteURL != "/archives/first" || gotSecond.RemoteURL != "/archives/second" {
		t.Fatalf("unexpected run results: first=%+v second=%+v", gotFirst, gotSecond)
	}
}

func TestRefreshingRuntimeExecutorReprocessBuildsFreshRuntimePerInvocation(t *testing.T) {
	first := &runtimeTaskUnitStub{}
	second := &runtimeTaskUnitStub{}

	buildCalls := 0
	executor := refreshingRuntimeExecutor{
		builder: runtimeTaskUnitBuilderFunc(func(_ context.Context) (runtimeTaskUnit, error) {
			buildCalls++
			if buildCalls == 1 {
				return first, nil
			}
			return second, nil
		}),
	}

	if err := executor.ReprocessArticle(context.Background(), "article-1", true); err != nil {
		t.Fatalf("ReprocessArticle() first error = %v", err)
	}
	if err := executor.ReprocessArticle(context.Background(), "article-2", false); err != nil {
		t.Fatalf("ReprocessArticle() second error = %v", err)
	}

	if buildCalls != 2 {
		t.Fatalf("builder calls = %d, want 2", buildCalls)
	}
	if first.reprocessCalls != 1 || second.reprocessCalls != 1 {
		t.Fatalf("reprocess calls = first:%d second:%d, want 1/1", first.reprocessCalls, second.reprocessCalls)
	}
	if len(first.gotArticleIDs) != 1 || first.gotArticleIDs[0] != "article-1" || !first.gotArticleForces[0] {
		t.Fatalf("first reprocess args = ids:%#v forces:%#v", first.gotArticleIDs, first.gotArticleForces)
	}
	if len(second.gotArticleIDs) != 1 || second.gotArticleIDs[0] != "article-2" || second.gotArticleForces[0] {
		t.Fatalf("second reprocess args = ids:%#v forces:%#v", second.gotArticleIDs, second.gotArticleForces)
	}
}

func TestRuntimeProcessingRunnerReusesExistingProcessingResult(t *testing.T) {
	processor := &processingServiceStub{}
	store := &processingStoreStub{
		record: postgres.ProcessedArticleRecord{
			ArticleID:       "art-1",
			TitleTranslated: "已翻译标题",
			CoreSummary:     "已有核心总结",
			TopicCategory:   "AI",
			ImportanceScore: 0.9,
		},
	}

	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		articleFinderStub{article: article.SourceArticle{
			ID:              "art-1",
			MinifluxEntryID: 101,
			Title:           "Original",
		}},
		processor,
		store,
		dossierMaterializerStub{dossier: domaindossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1", TitleTranslated: "已翻译标题", CoreSummary: "已有核心总结", TopicCategory: "AI", ImportanceScore: 0.9}},
		service.RuntimePromptVersions{Translation: 1, Analysis: 1, Dossier: 1, LLM: 1},
	)

	candidates, err := runner.ProcessPending(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("want 1 candidate got %d", len(candidates))
	}
	if got := candidates[0].Title; got != "已翻译标题" {
		t.Fatalf("want reused translated title got %s", got)
	}
	if got := candidates[0].DossierID; got != "dos-1" {
		t.Fatalf("want reused dossier id got %s", got)
	}
	if processor.called != 0 {
		t.Fatalf("want processor not called got %d", processor.called)
	}
	if store.saveHit != 0 {
		t.Fatalf("want save not called got %d", store.saveHit)
	}
	if store.getLatestHit != 1 {
		t.Fatalf("want get latest called once got %d", store.getLatestHit)
	}
}

func TestRuntimeProcessingRunnerProcessesAndSavesWhenNoExistingResult(t *testing.T) {
	processor := &processingServiceStub{
		result: processing.ProcessedArticle{
			Translation: processing.Translation{TitleTranslated: "新标题"},
			Analysis: processing.Analysis{
				CoreSummary:     "新核心总结",
				TopicCategory:   "AI",
				ImportanceScore: 0.8,
			},
		},
	}
	store := &processingStoreStub{err: gorm.ErrRecordNotFound}

	runner := service.NewRuntimeProcessingRunner(
		entryListerStub{entries: []miniflux.Entry{{ID: 101}}},
		articleFinderStub{article: article.SourceArticle{
			ID:              "art-1",
			MinifluxEntryID: 101,
			Title:           "Original",
		}},
		processor,
		store,
		dossierMaterializerStub{dossier: domaindossier.ArticleDossier{ID: "dos-1", ArticleID: "art-1", TitleTranslated: "新标题", CoreSummary: "新核心总结", TopicCategory: "AI", ImportanceScore: 0.8}},
		service.RuntimePromptVersions{Translation: 1, Analysis: 1, Dossier: 1, LLM: 1},
	)

	candidates, err := runner.ProcessPending(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("want 1 candidate got %d", len(candidates))
	}
	if processor.called != 1 {
		t.Fatalf("want processor called once got %d", processor.called)
	}
	if store.saveHit != 1 {
		t.Fatalf("want save called once got %d", store.saveHit)
	}
	if candidates[0] != (domaindigest.CandidateArticle{ID: "art-1", DossierID: "dos-1", Title: "新标题", CoreSummary: "新核心总结", TopicCategory: "AI", ImportanceScore: 0.8}) {
		t.Fatalf("unexpected candidate %+v", candidates[0])
	}
}

func TestChatModelInvokerGenerateRetriesTransientErrorThenSucceeds(t *testing.T) {
	chat := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("failed to create chat completion: 504 Gateway Time-out")},
			{message: &schema.Message{Content: "  ok  "}},
		},
	}
	invoker := chatModelInvoker{chat: chat}

	got, err := invoker.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("Generate() = %q, want %q", got, "ok")
	}
	if chat.calls != 2 {
		t.Fatalf("Generate() calls = %d, want 2", chat.calls)
	}
}

func TestChatModelInvokerGenerateNoRetryForNonTransientError(t *testing.T) {
	chat := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("failed to create chat completion: 400 invalid request")},
		},
	}
	invoker := chatModelInvoker{chat: chat}

	_, err := invoker.Generate(context.Background(), "prompt")
	if err == nil {
		t.Fatal("Generate() error = nil, want non-nil")
	}
	if chat.calls != 1 {
		t.Fatalf("Generate() calls = %d, want 1", chat.calls)
	}
}

func TestChatModelInvokerGenerateStopsRetryWhenContextCanceled(t *testing.T) {
	chat := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("failed to create chat completion: 504 Gateway Time-out")},
			{message: &schema.Message{Content: "ok"}},
		},
	}
	invoker := chatModelInvoker{chat: chat}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := invoker.Generate(ctx, "prompt")
	if err == nil {
		t.Fatal("Generate() error = nil, want non-nil")
	}
	if chat.calls != 1 {
		t.Fatalf("Generate() calls = %d, want 1", chat.calls)
	}
}

func TestChatModelInvokerGenerateFallsBackToNextModelAfterTransientFailures(t *testing.T) {
	primary := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("failed to create chat completion: 504 Gateway Time-out")},
			{err: errors.New("failed to create chat completion: 504 Gateway Time-out")},
			{err: errors.New("failed to create chat completion: 504 Gateway Time-out")},
		},
	}
	secondary := &chatModelStub{
		results: []chatGenerateResult{
			{message: &schema.Message{Content: "  fallback ok  "}},
		},
	}
	invoker := chatModelInvoker{
		models: []namedChatModel{
			{name: "MiniMax-M2.7", chat: primary},
			{name: "mimo-v2-pro", chat: secondary},
		},
	}

	got, err := invoker.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "fallback ok" {
		t.Fatalf("Generate() = %q, want %q", got, "fallback ok")
	}
	if primary.calls != 3 {
		t.Fatalf("primary calls = %d, want 3", primary.calls)
	}
	if secondary.calls != 1 {
		t.Fatalf("secondary calls = %d, want 1", secondary.calls)
	}
}

func TestChatModelInvokerGenerateStructuredJSONFallsBackWhenPrimaryReturnsInvalidJSON(t *testing.T) {
	primary := &chatModelStub{
		results: []chatGenerateResult{
			{message: &schema.Message{Content: "Before {\"broken\": true trailing"}},
			{message: &schema.Message{Content: "Before {\"broken\": true trailing"}},
			{message: &schema.Message{Content: "Before {\"broken\": true trailing"}},
		},
	}
	secondary := &chatModelStub{
		results: []chatGenerateResult{
			{message: &schema.Message{Content: "{\"ok\":true}"}},
		},
	}
	invoker := chatModelInvoker{
		models: []namedChatModel{
			{name: "MiniMax-M2.7", chat: primary},
			{name: "mimo-v2-pro", chat: secondary},
		},
	}

	got, err := invoker.GenerateStructuredJSON(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("GenerateStructuredJSON() error = %v", err)
	}
	if got != "{\"ok\":true}" {
		t.Fatalf("GenerateStructuredJSON() = %q", got)
	}
	if primary.calls != 3 {
		t.Fatalf("primary calls = %d, want 3", primary.calls)
	}
	if secondary.calls != 1 {
		t.Fatalf("secondary calls = %d, want 1", secondary.calls)
	}
}

func TestChatModelInvokerGenerateFallsBackAfterInternalServerError(t *testing.T) {
	primary := &chatModelStub{
		results: []chatGenerateResult{
			{err: errors.New("error, status code: 500, status: 500 Internal Server Error, message: unknown error")},
			{err: errors.New("error, status code: 500, status: 500 Internal Server Error, message: unknown error")},
			{err: errors.New("error, status code: 500, status: 500 Internal Server Error, message: unknown error")},
		},
	}
	secondary := &chatModelStub{
		results: []chatGenerateResult{
			{message: &schema.Message{Content: "server recovered"}},
		},
	}
	invoker := chatModelInvoker{
		models: []namedChatModel{
			{name: "MiniMax-M2.7", chat: primary},
			{name: "mimo-v2-pro", chat: secondary},
		},
	}

	got, err := invoker.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "server recovered" {
		t.Fatalf("Generate() = %q", got)
	}
	if primary.calls != 3 {
		t.Fatalf("primary calls = %d, want 3", primary.calls)
	}
	if secondary.calls != 1 {
		t.Fatalf("secondary calls = %d, want 1", secondary.calls)
	}
}

func TestRuntimeLLMFactoryConfigUsesTimeoutMS(t *testing.T) {
	cfg := runtimeLLMFactoryConfig(service.LLMRuntimeConfig{
		BaseURL:   "https://llm.local/v1",
		APIKey:    "token",
		Model:     "kimi-k2.5",
		TimeoutMS: 45000,
	})

	if cfg.BaseURL != "https://llm.local/v1" {
		t.Fatalf("want base_url passthrough got %q", cfg.BaseURL)
	}
	if cfg.APIKey != "token" {
		t.Fatalf("want api key passthrough got %q", cfg.APIKey)
	}
	if cfg.Model != "kimi-k2.5" {
		t.Fatalf("want model passthrough got %q", cfg.Model)
	}
	if cfg.Timeout != 45*time.Second {
		t.Fatalf("want timeout 45s got %s", cfg.Timeout)
	}
}

func TestRuntimeLLMFactoryConfigsIncludesFallbackModels(t *testing.T) {
	configs := runtimeLLMFactoryConfigs(service.LLMRuntimeConfig{
		BaseURL:        "https://llm.local/v1",
		APIKey:         "token",
		Model:          "MiniMax-M2.7",
		FallbackModels: []string{"mimo-v2-pro", "MiniMax-M2.7", " kimi-k2.5 "},
		TimeoutMS:      45000,
	})

	if len(configs) != 3 {
		t.Fatalf("want 3 configs got %d", len(configs))
	}
	if configs[0].Model != "MiniMax-M2.7" || configs[1].Model != "mimo-v2-pro" || configs[2].Model != "kimi-k2.5" {
		t.Fatalf("unexpected model chain %#v", configs)
	}
	for _, cfg := range configs {
		if cfg.Timeout != 45*time.Second {
			t.Fatalf("want timeout 45s got %s", cfg.Timeout)
		}
	}
}

func TestBuildPublisherReturnsHaloPublisher(t *testing.T) {
	cfg := &config.Config{}
	cfg.Publish.Channel = "halo"
	cfg.Publish.HaloBaseURL = "https://halo.local"
	cfg.Publish.HaloToken = "pat-token"

	publisher, err := buildPublisher(cfg)
	if err != nil {
		t.Fatalf("buildPublisher() error = %v", err)
	}
	if publisher.Name() != "halo" {
		t.Fatalf("want halo publisher got %q", publisher.Name())
	}
}

func TestBuildPublisherRequiresHaloToken(t *testing.T) {
	cfg := &config.Config{}
	cfg.Publish.Channel = "halo"
	cfg.Publish.HaloBaseURL = "https://halo.local"

	_, err := buildPublisher(cfg)
	if err == nil {
		t.Fatal("want error when halo token missing")
	}
	assertErrorContains(t, err, "APP_PUBLISH_HALO_TOKEN")
	assertErrorContains(t, err, "halo publisher")
}

func TestBuildPublisherPrefersHaloWhenChannelEmpty(t *testing.T) {
	cfg := &config.Config{}
	cfg.Publish.HaloBaseURL = "https://halo.local"
	cfg.Publish.HaloToken = "pat-token"
	cfg.Publish.OutputDir = "data/output"

	publisher, err := buildPublisher(cfg)
	if err != nil {
		t.Fatalf("buildPublisher() error = %v", err)
	}
	if publisher.Name() != "halo" {
		t.Fatalf("want halo publisher got %q", publisher.Name())
	}
}

func TestBuildPublisherUsesMarkdownExportWhenChannelEmptyWithoutHalo(t *testing.T) {
	cfg := &config.Config{}
	cfg.Publish.OutputDir = "data/output"

	publisher, err := buildPublisher(cfg)
	if err != nil {
		t.Fatalf("buildPublisher() error = %v", err)
	}
	if publisher.Name() != "markdown_export" {
		t.Fatalf("want markdown_export publisher got %q", publisher.Name())
	}
}

func TestBuildPublisherFromRuntimeConfigUsesMarkdownExportWhenProviderEmptyWithoutHalo(t *testing.T) {
	publisher, err := buildPublisherFromRuntimeConfig(service.PublishRuntimeConfig{
		OutputDir: "data/output",
	})
	if err != nil {
		t.Fatalf("buildPublisherFromRuntimeConfig() error = %v", err)
	}
	if publisher.Name() != "markdown_export" {
		t.Fatalf("want markdown_export publisher got %q", publisher.Name())
	}
}

func TestBuildPublisherRejectsDeprecatedLegacyAlias(t *testing.T) {
	deprecatedAlias := strings.Join([]string{"ho", "lo"}, "")
	cfg := &config.Config{}
	cfg.Publish.Channel = "  " + strings.ToUpper(deprecatedAlias) + "  "
	cfg.Publish.HaloBaseURL = "https://halo.local"
	cfg.Publish.HaloToken = "pat-token"

	_, err := buildPublisher(cfg)
	if err == nil {
		t.Fatal("want error for unsupported legacy alias")
	}
	assertErrorContains(t, err, "unsupported publish channel")
	assertErrorContains(t, err, `"`+deprecatedAlias+`"`)
}

func TestValidateRuntimeSnapshotRequiresResolvedMinifluxBaseURL(t *testing.T) {
	err := validateRuntimeSnapshot(service.RuntimeSnapshot{
		LLM:       service.LLMRuntimeConfig{Model: "MiniMax-M2.7"},
		Miniflux:  service.MinifluxRuntimeConfig{AuthToken: "token"},
		Publish:   service.PublishRuntimeConfig{Provider: "markdown_export", OutputDir: "data/output"},
		Scheduler: service.SchedulerRuntimeConfig{Enabled: true},
	})
	if err == nil {
		t.Fatal("want error for missing resolved miniflux base_url")
	}
	assertErrorContains(t, err, "APP_MINIFLUX_BASE_URL")
}

func TestValidateRuntimeSnapshotRequiresResolvedMinifluxAuthToken(t *testing.T) {
	err := validateRuntimeSnapshot(service.RuntimeSnapshot{
		LLM:       service.LLMRuntimeConfig{Model: "MiniMax-M2.7"},
		Miniflux:  service.MinifluxRuntimeConfig{BaseURL: "https://miniflux.local"},
		Publish:   service.PublishRuntimeConfig{Provider: "markdown_export", OutputDir: "data/output"},
		Scheduler: service.SchedulerRuntimeConfig{Enabled: true},
	})
	if err == nil {
		t.Fatal("want error for missing resolved miniflux auth token")
	}
	assertErrorContains(t, err, "APP_MINIFLUX_AUTH_TOKEN")
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("want error containing %q got %q", want, err.Error())
	}
}
