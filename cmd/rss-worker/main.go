package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/hibiken/asynq"

	promptassets "rss-platform/configs/prompts"
	llmadapter "rss-platform/internal/adapter/llm"
	"rss-platform/internal/adapter/miniflux"
	adapterpublisher "rss-platform/internal/adapter/publisher"
	"rss-platform/internal/adapter/publisher/halo"
	"rss-platform/internal/adapter/publisher/holo"
	"rss-platform/internal/adapter/publisher/markdown_export"
	"rss-platform/internal/agent/digest_planning"
	appworker "rss-platform/internal/app/worker"
	"rss-platform/internal/config"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/domain/dossier"
	"rss-platform/internal/render"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/service"
	asynqtask "rss-platform/internal/task/asynq"
	"rss-platform/internal/workflow/daily_digest_workflow"
)

const (
	translationPromptFile = "translation.tmpl"
	analysisPromptFile    = "analysis.tmpl"
	dossierPromptFile     = "dossier.tmpl"
	digestPromptFile      = "digest.tmpl"
	maxChatGenerateTry    = 3
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.Redis.Addr == "" {
		log.Fatal("APP_REDIS_ADDR is required")
	}
	if cfg.Database.DSN == "" {
		log.Fatal("APP_DATABASE_DSN is required")
	}
	if cfg.Miniflux.BaseURL == "" {
		log.Fatal("APP_MINIFLUX_BASE_URL is required")
	}
	if cfg.Miniflux.AuthToken == "" {
		log.Fatal("APP_MINIFLUX_AUTH_TOKEN is required")
	}

	runtimeSvc, processingRunner, dbCloser, err := buildRuntimeService(context.Background(), cfg)
	if err != nil {
		log.Fatalf("build runtime service: %v", err)
	}
	defer func() {
		if err := dbCloser(); err != nil {
			log.Printf("close postgres: %v", err)
		}
	}()

	server := appworker.NewServer(asynq.RedisClientOpt{Addr: cfg.Redis.Addr}, appworker.ServerConfig{
		Concurrency: cfg.Worker.Concurrency,
		Queues: map[string]int{
			cfg.Job.Queue: 1,
		},
	})
	mux := appworker.NewServeMux(
		nil,
		asynqtask.NewDailyDigestHandler(func(ctx context.Context, payload asynqtask.DailyDigestPayload) error {
			now := time.Now().In(shanghaiLocation())
			result, err := runtimeSvc.Run(ctx, payload.DigestDate, now, service.RunOptions{Force: payload.Force})
			if err != nil {
				return err
			}

			log.Printf("daily digest task consumed: date=%s force=%t url=%s", result.DigestDate, payload.Force, result.RemoteURL)
			return nil
		}),
		asynqtask.NewArticleReprocessHandler(func(ctx context.Context, payload asynqtask.ReprocessArticlePayload) error {
			if processingRunner == nil {
				return errors.New("runtime processing runner is required")
			}

			if err := processingRunner.ReprocessArticle(ctx, payload.ArticleID, payload.Force); err != nil {
				return err
			}

			log.Printf("article reprocess task consumed: article_id=%s force=%t", payload.ArticleID, payload.Force)
			return nil
		}),
	)

	log.Println("rss-worker started")
	if err := server.Run(mux); err != nil {
		log.Fatalf("run worker: %v", err)
	}
}

func buildRuntimeService(ctx context.Context, cfg *config.Config) (*service.DailyDigestRuntimeService, *service.RuntimeProcessingRunner, func() error, error) {
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		return nil, nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, nil, err
	}

	runtimeConfigs := service.NewRuntimeConfigService(postgres.NewProfileRepository(db), cfg)
	runtimeSnapshot, err := runtimeConfigs.Snapshot(ctx)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}
	if runtimeSnapshot.LLM.Model == "" {
		_ = sqlDB.Close()
		return nil, nil, nil, errors.New("APP_LLM_MODEL is required")
	}

	invoker, err := buildChatModelInvoker(ctx, runtimeSnapshot.LLM, llmadapter.NewChatModel)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}
	articleRepo := postgres.NewArticleRepository(db)
	processingRepo := postgres.NewProcessingRepository(db)
	digestRepo := postgres.NewDigestRepository(db)
	dossierRepo := postgres.NewDossierRepository(db)
	publishStateRepo := postgres.NewPublishStateRepository(db)
	minifluxClient := miniflux.NewClient(cfg.Miniflux.BaseURL, cfg.Miniflux.AuthToken)
	translationTemplate, analysisTemplate, dossierTemplate, digestTemplate, err := loadDefaultPromptTemplates()
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}
	if strings.TrimSpace(runtimeSnapshot.Prompts.TranslationPrompt) != "" {
		translationTemplate = runtimeSnapshot.Prompts.TranslationPrompt
	}
	if strings.TrimSpace(runtimeSnapshot.Prompts.AnalysisPrompt) != "" {
		analysisTemplate = runtimeSnapshot.Prompts.AnalysisPrompt
	}
	if strings.TrimSpace(runtimeSnapshot.Prompts.DossierPrompt) != "" {
		dossierTemplate = runtimeSnapshot.Prompts.DossierPrompt
	}
	if strings.TrimSpace(runtimeSnapshot.Prompts.DigestPrompt) != "" {
		digestTemplate = runtimeSnapshot.Prompts.DigestPrompt
	}

	publisher, err := buildPublisher(cfg)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, nil, err
	}

	processingSvc := service.NewProcessingService(llmadapter.NewArticleProcessorFromTemplateText(invoker, translationTemplate, analysisTemplate))
	dossierSvc := service.NewDossierService(
		dossierBuilderAdapter{builder: llmadapter.NewDossierBuilderFromTemplateText(invoker, dossierTemplate)},
		dossierRepo,
		publishStateRepo,
	)
	processingRunner := service.NewRuntimeProcessingRunner(
		minifluxClient,
		articleRepo,
		processingSvc,
		processingRepo,
		dossierSvc,
		service.RuntimePromptVersions{
			Translation: runtimeSnapshot.Prompts.TranslationVersion,
			Analysis:    runtimeSnapshot.Prompts.AnalysisVersion,
			Dossier:     runtimeSnapshot.Prompts.DossierVersion,
			LLM:         runtimeSnapshot.LLM.Version,
		},
	)
	digestRunner := digestWorkflowRunner{
		workflow: daily_digest_workflow.New(
			digest_planning.NewWithPrompt(digest_planning.NewOpenAIRunner(invoker), digestTemplate),
			render.NewDigestRenderer(),
		),
		digestPromptVersion: runtimeSnapshot.Prompts.DigestVersion,
		llmProfileVersion:   runtimeSnapshot.LLM.Version,
	}

	runtimeSvc := service.NewDailyDigestRuntimeService(
		service.NewArticleIngestionService(minifluxClient, articleRepo),
		processingRunner,
		digestRunner,
		digestRepo,
		publisher,
	)

	return runtimeSvc, processingRunner, sqlDB.Close, nil
}

func runtimeLLMFactoryConfig(cfg service.LLMRuntimeConfig) llmadapter.FactoryConfig {
	factoryCfg := llmadapter.FactoryConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	}
	if cfg.TimeoutMS > 0 {
		factoryCfg.Timeout = time.Duration(cfg.TimeoutMS) * time.Millisecond
	}
	return factoryCfg
}

func runtimeLLMFactoryConfigs(cfg service.LLMRuntimeConfig) []llmadapter.FactoryConfig {
	models := normalizeLLMModelChain(cfg.Model, cfg.FallbackModels)
	configs := make([]llmadapter.FactoryConfig, 0, len(models))
	for _, modelName := range models {
		cfg.Model = modelName
		configs = append(configs, runtimeLLMFactoryConfig(cfg))
	}
	return configs
}

func normalizeLLMModelChain(primary string, fallbacks []string) []string {
	items := append([]string{primary}, fallbacks...)
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

type chatModelFactory func(ctx context.Context, cfg llmadapter.FactoryConfig) (einomodel.BaseChatModel, error)

func buildChatModelInvoker(ctx context.Context, cfg service.LLMRuntimeConfig, newChatModel chatModelFactory) (chatModelInvoker, error) {
	if newChatModel == nil {
		newChatModel = llmadapter.NewChatModel
	}

	factoryConfigs := runtimeLLMFactoryConfigs(cfg)
	if len(factoryConfigs) == 0 {
		return chatModelInvoker{}, errors.New("APP_LLM_MODEL is required")
	}

	models := make([]namedChatModel, 0, len(factoryConfigs))
	for _, factoryCfg := range factoryConfigs {
		chatModel, err := newChatModel(ctx, factoryCfg)
		if err != nil {
			return chatModelInvoker{}, err
		}
		models = append(models, namedChatModel{
			name: factoryCfg.Model,
			chat: chatModel,
		})
	}

	return chatModelInvoker{models: models}, nil
}

type chatModelInvoker struct {
	chat   einomodel.BaseChatModel
	models []namedChatModel
}

type namedChatModel struct {
	name string
	chat einomodel.BaseChatModel
}

type dossierBuilderAdapter struct {
	builder *llmadapter.DossierBuilder
}

func (a dossierBuilderAdapter) Build(ctx context.Context, input service.BuildDossierInput) (dossier.ArticleDossier, error) {
	return a.builder.Build(ctx, llmadapter.DossierBuildInput{
		Article:    input.Article,
		Processing: input.Processing,
	})
}

func (i chatModelInvoker) Generate(ctx context.Context, prompt string) (string, error) {
	return i.generate(ctx, prompt, nil)
}

func (i chatModelInvoker) GenerateStructuredJSON(ctx context.Context, prompt string) (string, error) {
	return i.generate(ctx, prompt, validateStructuredJSONObject)
}

func (i chatModelInvoker) generate(ctx context.Context, prompt string, validator func(string) error) (string, error) {
	models := i.models
	if len(models) == 0 && i.chat != nil {
		models = []namedChatModel{{name: "default", chat: i.chat}}
	}
	if len(models) == 0 {
		return "", errors.New("chat model is required")
	}

	var lastErr error
	for idx, item := range models {
		result, err := i.generateWithModel(ctx, item.chat, prompt, validator)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if idx == len(models)-1 || !shouldFallbackChatGenerateError(ctx, err) {
			return "", err
		}

		log.Printf("chat model fallback activated: from=%s to=%s err=%v", item.name, models[idx+1].name, err)
	}

	return "", lastErr
}

func (i chatModelInvoker) generateWithModel(
	ctx context.Context,
	chat einomodel.BaseChatModel,
	prompt string,
	validator func(string) error,
) (string, error) {
	for attempt := 1; attempt <= maxChatGenerateTry; attempt++ {
		message, err := chat.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
		if err != nil {
			if attempt == maxChatGenerateTry || !shouldRetryChatGenerateError(ctx, err) {
				return "", err
			}
			continue
		}
		if message == nil {
			err = errors.New("empty llm response")
			if attempt == maxChatGenerateTry || !shouldFallbackChatGenerateError(ctx, err) {
				return "", err
			}
			continue
		}

		content := strings.TrimSpace(message.Content)
		if content == "" {
			err = errors.New("empty llm response")
			if attempt == maxChatGenerateTry || !shouldFallbackChatGenerateError(ctx, err) {
				return "", err
			}
			continue
		}
		if validator != nil {
			if err := validator(content); err != nil {
				if attempt == maxChatGenerateTry || !shouldFallbackChatGenerateError(ctx, err) {
					return "", err
				}
				continue
			}
		}

		return content, nil
	}

	return "", errors.New("failed to create chat completion")
}

func shouldFallbackChatGenerateError(ctx context.Context, err error) bool {
	if shouldRetryChatGenerateError(ctx, err) {
		return true
	}
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "empty llm response") ||
		strings.Contains(text, "invalid structured json") ||
		strings.Contains(text, "invalid character") ||
		strings.Contains(text, "unexpected end of json input") ||
		strings.Contains(text, "cannot unmarshal")
}

func shouldRetryChatGenerateError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"500 internal server error",
		"status code: 500",
		"502 bad gateway",
		"status code: 502",
		"503 service unavailable",
		"status code: 503",
		"504 gateway time-out",
		"504 gateway timeout",
		"status code: 504",
		" 504",
		"529",
		"temporary",
		"connection reset by peer",
		"read: connection reset",
		"i/o timeout",
		"context deadline exceeded",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func validateStructuredJSONObject(raw string) error {
	normalized := normalizeStructuredJSONObject(raw)
	if !json.Valid([]byte(normalized)) {
		return errors.New("invalid structured json")
	}
	return nil
}

func normalizeStructuredJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end >= start {
		return trimmed[start : end+1]
	}

	return trimmed
}

type digestWorkflowRunner struct {
	workflow            *daily_digest_workflow.Workflow
	digestPromptVersion int
	llmProfileVersion   int
}

func (r digestWorkflowRunner) Generate(ctx context.Context, candidates []domaindigest.CandidateArticle) (daily_digest_workflow.Digest, error) {
	digest, err := r.workflow.Run(ctx, candidates)
	if err != nil {
		return daily_digest_workflow.Digest{}, err
	}
	digest.DigestPromptVersion = r.digestPromptVersion
	digest.LLMProfileVersion = r.llmProfileVersion
	return digest, nil
}

func buildPublisher(cfg *config.Config) (adapterpublisher.Publisher, error) {
	channel := strings.ToLower(strings.TrimSpace(cfg.Publish.Channel))

	switch {
	case channel == "" && cfg.Publish.HaloBaseURL != "":
		if cfg.Publish.HaloToken == "" {
			return nil, errors.New("APP_PUBLISH_HALO_TOKEN is required for halo publisher")
		}
		return halo.New(cfg.Publish.HaloBaseURL, cfg.Publish.HaloToken), nil
	case channel == "" && cfg.Publish.HoloEndpoint != "":
		return holo.New(cfg.Publish.HoloEndpoint, cfg.Publish.HoloToken), nil
	case channel == "" && cfg.Publish.OutputDir != "":
		return markdown_export.New(cfg.Publish.OutputDir), nil
	case channel == "" || channel == "markdown" || channel == "markdown_export":
		if cfg.Publish.OutputDir == "" {
			return nil, errors.New("APP_PUBLISH_OUTPUT_DIR is required for markdown publisher")
		}
		return markdown_export.New(cfg.Publish.OutputDir), nil
	case channel == "halo":
		if cfg.Publish.HaloBaseURL == "" {
			return nil, errors.New("APP_PUBLISH_HALO_BASE_URL is required for halo publisher")
		}
		if cfg.Publish.HaloToken == "" {
			return nil, errors.New("APP_PUBLISH_HALO_TOKEN is required for halo publisher")
		}
		return halo.New(cfg.Publish.HaloBaseURL, cfg.Publish.HaloToken), nil
	case channel == "holo":
		if cfg.Publish.HoloEndpoint == "" {
			return nil, errors.New("APP_PUBLISH_HOLO_ENDPOINT is required for holo publisher")
		}
		return holo.New(cfg.Publish.HoloEndpoint, cfg.Publish.HoloToken), nil
	default:
		return nil, fmt.Errorf("unsupported publish channel %q", cfg.Publish.Channel)
	}
}

func shanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		return loc
	}

	return time.FixedZone("CST", 8*3600)
}

func loadDefaultPromptTemplates() (string, string, string, string, error) {
	translationTemplate, err := promptassets.Read(translationPromptFile)
	if err != nil {
		return "", "", "", "", err
	}

	analysisTemplate, err := promptassets.Read(analysisPromptFile)
	if err != nil {
		return "", "", "", "", err
	}

	dossierTemplate, err := promptassets.Read(dossierPromptFile)
	if err != nil {
		return "", "", "", "", err
	}
	digestTemplate, err := promptassets.Read(digestPromptFile)
	if err != nil {
		return "", "", "", "", err
	}

	return translationTemplate, analysisTemplate, dossierTemplate, digestTemplate, nil
}
