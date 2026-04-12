package main

import (
	"context"
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

	runtimeSvc, dbCloser, err := buildRuntimeService(context.Background(), cfg)
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
		asynqtask.NewArticleReprocessHandler(func(_ context.Context, payload asynqtask.ReprocessArticlePayload) error {
			log.Printf("article reprocess task consumed: article_id=%s force=%t", payload.ArticleID, payload.Force)
			return nil
		}),
	)

	log.Println("rss-worker started")
	if err := server.Run(mux); err != nil {
		log.Fatalf("run worker: %v", err)
	}
}

func buildRuntimeService(ctx context.Context, cfg *config.Config) (*service.DailyDigestRuntimeService, func() error, error) {
	db, err := postgres.Open(cfg.Database.DSN)
	if err != nil {
		return nil, nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}

	runtimeConfigs := service.NewRuntimeConfigService(postgres.NewProfileRepository(db), cfg)
	runtimeSnapshot, err := runtimeConfigs.Snapshot(ctx)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}
	if runtimeSnapshot.LLM.Model == "" {
		_ = sqlDB.Close()
		return nil, nil, errors.New("APP_LLM_MODEL is required")
	}

	chatModel, err := llmadapter.NewChatModel(ctx, llmadapter.FactoryConfig{
		BaseURL: runtimeSnapshot.LLM.BaseURL,
		APIKey:  runtimeSnapshot.LLM.APIKey,
		Model:   runtimeSnapshot.LLM.Model,
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	invoker := chatModelInvoker{chat: chatModel}
	articleRepo := postgres.NewArticleRepository(db)
	processingRepo := postgres.NewProcessingRepository(db)
	digestRepo := postgres.NewDigestRepository(db)
	dossierRepo := postgres.NewDossierRepository(db)
	publishStateRepo := postgres.NewPublishStateRepository(db)
	minifluxClient := miniflux.NewClient(cfg.Miniflux.BaseURL, cfg.Miniflux.AuthToken)
	translationTemplate, analysisTemplate, dossierTemplate, digestTemplate, err := loadDefaultPromptTemplates()
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
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
		return nil, nil, err
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

	return runtimeSvc, sqlDB.Close, nil
}

type chatModelInvoker struct {
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
	if i.chat == nil {
		return "", errors.New("chat model is required")
	}

	message, err := i.chat.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		return "", err
	}
	if message == nil {
		return "", errors.New("empty llm response")
	}

	return strings.TrimSpace(message.Content), nil
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
	case channel == "" && cfg.Publish.OutputDir != "":
		return markdown_export.New(cfg.Publish.OutputDir), nil
	case channel == "" && cfg.Publish.HoloEndpoint != "":
		return holo.New(cfg.Publish.HoloEndpoint, cfg.Publish.HoloToken), nil
	case channel == "" || channel == "markdown" || channel == "markdown_export":
		if cfg.Publish.OutputDir == "" {
			return nil, errors.New("APP_PUBLISH_OUTPUT_DIR is required for markdown publisher")
		}
		return markdown_export.New(cfg.Publish.OutputDir), nil
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
