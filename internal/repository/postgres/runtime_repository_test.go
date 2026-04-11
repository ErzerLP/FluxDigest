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

	first := postgres.ProcessedArticleRecord{
		ArticleID:         "art-1",
		TitleTranslated:   "标题-1",
		SummaryTranslated: "摘要-1",
		ContentTranslated: "正文-1",
		CoreSummary:       "核心总结-1",
		KeyPoints:         []string{"a"},
		TopicCategory:     "Old",
		ImportanceScore:   0.6,
	}
	if err := repo.Save(ctx, first); err != nil {
		t.Fatal(err)
	}

	second := postgres.ProcessedArticleRecord{
		ArticleID:         "art-1",
		TitleTranslated:   "标题-2",
		SummaryTranslated: "摘要-2",
		ContentTranslated: "正文-2",
		CoreSummary:       "核心总结-2",
		KeyPoints:         []string{"a", "b"},
		TopicCategory:     "AI",
		ImportanceScore:   0.8,
	}
	if err := repo.Save(ctx, second); err != nil {
		t.Fatal(err)
	}

	var rows []models.ArticleProcessingModel
	if err := db.WithContext(ctx).Where("article_id = ?", "art-1").Order("created_at ASC").Find(&rows).Error; err != nil {
		t.Fatalf("list processing rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows got %d", len(rows))
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
	if len(got.KeyPoints) != 2 || got.KeyPoints[0] != "a" || got.KeyPoints[1] != "b" {
		t.Fatalf("unexpected key points: %#v", got.KeyPoints)
	}
}

func TestDigestRepositorySaveUpsertsAndGetByDigestDate(t *testing.T) {
	db := newRuntimeTestDB(t)
	migrateRuntimeTables(t, db)
	repo := postgres.NewDigestRepository(db)
	ctx := context.Background()

	if err := repo.Save(ctx, "2026-04-11", daily_digest_workflow.Digest{
		Title:           "第一次日报",
		Subtitle:        "第一版",
		ContentMarkdown: "# first",
		ContentHTML:     "<h1>first</h1>",
	}, adapterpublisher.PublishDigestResult{RemoteURL: "https://example.com/first"}); err != nil {
		t.Fatalf("save first digest: %v", err)
	}

	if err := repo.Save(ctx, "2026-04-11", daily_digest_workflow.Digest{
		Title:           "第二次日报",
		Subtitle:        "第二版",
		ContentMarkdown: "# second",
		ContentHTML:     "<h1>second</h1>",
	}, adapterpublisher.PublishDigestResult{RemoteURL: "https://example.com/second"}); err != nil {
		t.Fatalf("save second digest: %v", err)
	}

	var count int64
	if err := db.Model(&models.DailyDigestModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count digests: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 digest row got %d", count)
	}

	got, err := repo.GetByDigestDate(ctx, "2026-04-11")
	if err != nil {
		t.Fatalf("get by digest date: %v", err)
	}
	if got.DigestDate != "2026-04-11" {
		t.Fatalf("want digest date 2026-04-11 got %s", got.DigestDate)
	}
	if got.Title != "第二次日报" {
		t.Fatalf("want latest title 第二次日报 got %s", got.Title)
	}
	if got.RemoteURL != "https://example.com/second" {
		t.Fatalf("want latest remote url got %s", got.RemoteURL)
	}
}
