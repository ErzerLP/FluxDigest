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
	"gorm.io/gorm"

	promptassets "rss-platform/configs/prompts"
	llmadapter "rss-platform/internal/adapter/llm"
	"rss-platform/internal/adapter/miniflux"
	adapterpublisher "rss-platform/internal/adapter/publisher"
	"rss-platform/internal/adapter/publisher/holo"
	"rss-platform/internal/adapter/publisher/markdown_export"
	"rss-platform/internal/agent/digest_planning"
	appworker "rss-platform/internal/app/worker"
	"rss-platform/internal/config"
	"rss-platform/internal/domain/article"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/domain/processing"
	"rss-platform/internal/render"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/service"
	asynqtask "rss-platform/internal/task/asynq"
	"rss-platform/internal/workflow/daily_digest_workflow"
)

const (
	translationPromptFile = "translation.tmpl"
	analysisPromptFile    = "analysis.tmpl"
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
	mux := appworker.NewServeMux(nil, asynqtask.NewDailyDigestHandler(func(ctx context.Context, digestDate string) error {
		now := time.Now().In(shanghaiLocation())
		result, err := runtimeSvc.Run(ctx, digestDate, now)
		if err != nil {
			return err
		}

		log.Printf("daily digest task consumed: date=%s url=%s", result.DigestDate, result.RemoteURL)
		return nil
	}))

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
	minifluxClient := miniflux.NewClient(cfg.Miniflux.BaseURL, cfg.Miniflux.AuthToken)
	translationTemplate, analysisTemplate, err := loadDefaultPromptTemplates()
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	publisher, err := buildPublisher(cfg)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, err
	}

	processingSvc := service.NewProcessingService(llmadapter.NewArticleProcessorFromTemplateText(invoker, translationTemplate, analysisTemplate))
	processingRunner := &runtimeProcessingRunner{
		client:     minifluxClient,
		articles:   articleRepo,
		processing: processingSvc,
		results:    processingRepo,
	}
	digestRunner := digestWorkflowRunner{
		workflow: daily_digest_workflow.New(
			digest_planning.New(digest_planning.NewOpenAIRunner(invoker)),
			render.NewDigestRenderer(),
		),
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

type runtimeProcessingRunner struct {
	client     entryLister
	articles   articleFinder
	processing articleProcessor
	results    processingStore
}

func (r *runtimeProcessingRunner) ProcessPending(ctx context.Context, windowStart, windowEnd time.Time) ([]domaindigest.CandidateArticle, error) {
	entries, err := r.client.ListEntries(ctx, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}

	candidates := make([]domaindigest.CandidateArticle, 0, len(entries))
	seen := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		if _, ok := seen[entry.ID]; ok {
			continue
		}
		seen[entry.ID] = struct{}{}

		source, err := r.articles.FindByMinifluxEntryID(ctx, entry.ID)
		if err != nil {
			return nil, err
		}

		existing, err := r.results.GetLatestByArticleID(ctx, source.ID)
		if err == nil {
			candidates = append(candidates, candidateFromStoredProcessing(source, existing))
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		processed, err := r.processing.ProcessArticle(ctx, source)
		if err != nil {
			return nil, err
		}

		if err := r.results.Save(ctx, postgres.ProcessedArticleRecord{
			ArticleID:         source.ID,
			TitleTranslated:   processed.Translation.TitleTranslated,
			SummaryTranslated: processed.Translation.SummaryTranslated,
			ContentTranslated: processed.Translation.ContentTranslated,
			CoreSummary:       processed.Analysis.CoreSummary,
			KeyPoints:         processed.Analysis.KeyPoints,
			TopicCategory:     processed.Analysis.TopicCategory,
			ImportanceScore:   processed.Analysis.ImportanceScore,
		}); err != nil {
			return nil, err
		}

		candidates = append(candidates, candidateFromProcessedArticle(source, processed))
	}

	return candidates, nil
}

type digestWorkflowRunner struct {
	workflow *daily_digest_workflow.Workflow
}

func (r digestWorkflowRunner) Generate(ctx context.Context, candidates []domaindigest.CandidateArticle) (daily_digest_workflow.Digest, error) {
	return r.workflow.Run(ctx, candidates)
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

func loadDefaultPromptTemplates() (string, string, error) {
	translationTemplate, err := promptassets.Read(translationPromptFile)
	if err != nil {
		return "", "", err
	}

	analysisTemplate, err := promptassets.Read(analysisPromptFile)
	if err != nil {
		return "", "", err
	}

	return translationTemplate, analysisTemplate, nil
}

type entryLister interface {
	ListEntries(ctx context.Context, windowStart, windowEnd time.Time) ([]miniflux.Entry, error)
}

type articleFinder interface {
	FindByMinifluxEntryID(ctx context.Context, minifluxEntryID int64) (article.SourceArticle, error)
}

type articleProcessor interface {
	ProcessArticle(ctx context.Context, input article.SourceArticle) (processing.ProcessedArticle, error)
}

type processingStore interface {
	GetLatestByArticleID(ctx context.Context, articleID string) (postgres.ProcessedArticleRecord, error)
	Save(ctx context.Context, input postgres.ProcessedArticleRecord) error
}

func candidateFromStoredProcessing(source article.SourceArticle, record postgres.ProcessedArticleRecord) domaindigest.CandidateArticle {
	title := record.TitleTranslated
	if title == "" {
		title = source.Title
	}

	return domaindigest.CandidateArticle{
		ID:          source.ID,
		Title:       title,
		CoreSummary: record.CoreSummary,
	}
}

func candidateFromProcessedArticle(source article.SourceArticle, processed processing.ProcessedArticle) domaindigest.CandidateArticle {
	title := processed.Translation.TitleTranslated
	if title == "" {
		title = source.Title
	}

	return domaindigest.CandidateArticle{
		ID:          source.ID,
		Title:       title,
		CoreSummary: processed.Analysis.CoreSummary,
	}
}
