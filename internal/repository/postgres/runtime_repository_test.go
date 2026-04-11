package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"
	"rss-platform/internal/workflow/daily_digest_workflow"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRuntimeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-runtime?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open runtime db: %v", err)
	}
	return db
}

func migrateRuntimeTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&models.ArticleProcessingModel{}, &models.DailyDigestModel{}); err != nil {
		t.Fatalf("auto migrate runtime tables: %v", err)
	}
}

func TestProcessingRepositorySaveAndListLatestByArticle(t *testing.T) {
	db := newRuntimeTestDB(t)
	migrateRuntimeTables(t, db)
	repo := postgres.NewProcessingRepository(db)
	ctx := context.Background()

	first := postgres.ProcessedArticleRecord{ArticleID: "art-1", TitleTranslated: "标题-1", SummaryTranslated: "摘要-1", ContentTranslated: "正文-1", CoreSummary: "核心总结-1", KeyPoints: []string{"a"}, TopicCategory: "Old", ImportanceScore: 0.6}
	if err := repo.Save(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := postgres.ProcessedArticleRecord{ArticleID: "art-1", TitleTranslated: "标题-2", SummaryTranslated: "摘要-2", ContentTranslated: "正文-2", CoreSummary: "核心总结-2", KeyPoints: []string{"a", "b"}, TopicCategory: "AI", ImportanceScore: 0.8}
	if err := repo.Save(ctx, second); err != nil {
		t.Fatal(err)
	}

	sharedCreatedAt := time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)
	if err := db.WithContext(ctx).Model(&models.ArticleProcessingModel{}).Where("article_id = ?", "art-1").Update("created_at", sharedCreatedAt).Error; err != nil {
		t.Fatalf("align created_at: %v", err)
	}

	got, err := repo.GetLatestByArticleID(ctx, "art-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.TopicCategory != "AI" {
		t.Fatalf("want AI got %s", got.TopicCategory)
	}
	if got.TitleTranslated != "标题-2" {
		t.Fatalf("want latest translated title 标题-2 got %s", got.TitleTranslated)
	}
}

func TestDigestRepositoryReserveMarkPublishedAndGetByDigestDate(t *testing.T) {
	db := newRuntimeTestDB(t)
	migrateRuntimeTables(t, db)
	repo := postgres.NewDigestRepository(db)
	ctx := context.Background()
	digest := daily_digest_workflow.Digest{Title: "第一次日报", Subtitle: "第一版", ContentMarkdown: "# first", ContentHTML: "<h1>first</h1>"}

	reserved, err := repo.Reserve(ctx, "2026-04-11", digest)
	if err != nil {
		t.Fatalf("reserve digest: %v", err)
	}
	if !reserved {
		t.Fatal("expected first reserve to succeed")
	}

	reserved, err = repo.Reserve(ctx, "2026-04-11", daily_digest_workflow.Digest{Title: "第二次日报", Subtitle: "第二版", ContentMarkdown: "# second", ContentHTML: "<h1>second</h1>"})
	if err != nil {
		t.Fatalf("reserve existing digest: %v", err)
	}
	if reserved {
		t.Fatal("expected second reserve to be ignored")
	}

	got, err := repo.GetByDigestDate(ctx, "2026-04-11")
	if err != nil {
		t.Fatalf("get pending digest: %v", err)
	}
	if got.RemoteURL != "" {
		t.Fatalf("want pending digest remote url empty got %s", got.RemoteURL)
	}
	if got.Title != "第一次日报" {
		t.Fatalf("want reserved title 第一次日报 got %s", got.Title)
	}

	if err := repo.MarkPublished(ctx, "2026-04-11", adapterpublisher.PublishDigestResult{RemoteURL: "https://example.com/second"}); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	got, err = repo.GetByDigestDate(ctx, "2026-04-11")
	if err != nil {
		t.Fatalf("get published digest: %v", err)
	}
	if got.RemoteURL != "https://example.com/second" {
		t.Fatalf("want published remote url got %s", got.RemoteURL)
	}
}
