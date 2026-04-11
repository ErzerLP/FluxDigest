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

	err := repo.Save(context.Background(), postgres.ProcessedArticleRecord{
		ArticleID:         "art-1",
		TitleTranslated:   "标题",
		SummaryTranslated: "摘要",
		ContentTranslated: "正文",
		CoreSummary:       "核心总结",
		KeyPoints:         []string{"a", "b"},
		TopicCategory:     "AI",
		ImportanceScore:   0.8,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetLatestByArticleID(context.Background(), "art-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.TopicCategory != "AI" {
		t.Fatalf("want AI got %s", got.TopicCategory)
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
	zone := time.FixedZone("CST", 8*3600)

	firstRunAt := time.Date(2026, 4, 11, 7, 0, 0, 0, zone)
	if err := repo.Save(ctx, firstRunAt, daily_digest_workflow.Digest{
		Title:           "第一次日报",
		Subtitle:        "第一版",
		ContentMarkdown: "# first",
		ContentHTML:     "<h1>first</h1>",
	}, adapterpublisher.PublishDigestResult{RemoteURL: "https://example.com/first"}); err != nil {
		t.Fatalf("save first digest: %v", err)
	}

	secondRunAt := time.Date(2026, 4, 11, 23, 30, 0, 0, zone)
	if err := repo.Save(ctx, secondRunAt, daily_digest_workflow.Digest{
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
