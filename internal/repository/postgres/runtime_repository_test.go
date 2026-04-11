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

func TestDigestRepositoryStateTransitions(t *testing.T) {
	db := newRuntimeTestDB(t)
	migrateRuntimeTables(t, db)
	repo := postgres.NewDigestRepository(db)
	ctx := context.Background()
	digest := daily_digest_workflow.Digest{Title: "第一次日报", Subtitle: "第一版", ContentMarkdown: "# first", ContentHTML: "<h1>first</h1>"}

	started, err := repo.BeginPublish(ctx, "2026-04-11", digest)
	if err != nil {
		t.Fatalf("begin publish: %v", err)
	}
	if !started {
		t.Fatal("expected first begin publish to succeed")
	}

	state, remoteURL, found, err := repo.GetState(ctx, "2026-04-11")
	if err != nil {
		t.Fatalf("get publishing state: %v", err)
	}
	if !found || state != "publishing" || remoteURL != "" {
		t.Fatalf("unexpected publishing state found=%v state=%s remote_url=%s", found, state, remoteURL)
	}

	if err := repo.MarkFailed(ctx, "2026-04-11", "server 500"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	state, _, found, err = repo.GetState(ctx, "2026-04-11")
	if err != nil {
		t.Fatalf("get failed state: %v", err)
	}
	if !found || state != "failed" {
		t.Fatalf("want failed state got found=%v state=%s", found, state)
	}

	started, err = repo.BeginPublish(ctx, "2026-04-11", daily_digest_workflow.Digest{Title: "第二次日报", Subtitle: "第二版", ContentMarkdown: "# second", ContentHTML: "<h1>second</h1>"})
	if err != nil {
		t.Fatalf("begin publish after failed: %v", err)
	}
	if !started {
		t.Fatal("expected failed digest to be restartable")
	}

	if err := repo.MarkPublished(ctx, "2026-04-11", adapterpublisher.PublishDigestResult{RemoteID: "remote-2", RemoteURL: "https://example.com/second"}); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	state, remoteURL, found, err = repo.GetState(ctx, "2026-04-11")
	if err != nil {
		t.Fatalf("get published state: %v", err)
	}
	if !found || state != "published" || remoteURL != "https://example.com/second" {
		t.Fatalf("unexpected published state found=%v state=%s remote_url=%s", found, state, remoteURL)
	}

	if err := repo.MarkRecoveryRequired(ctx, "2026-04-11", "decode failed"); err != nil {
		t.Fatalf("mark recovery required: %v", err)
	}
	state, _, found, err = repo.GetState(ctx, "2026-04-11")
	if err != nil {
		t.Fatalf("get recovery_required state: %v", err)
	}
	if !found || state != "recovery_required" {
		t.Fatalf("want recovery_required state got found=%v state=%s", found, state)
	}
}
