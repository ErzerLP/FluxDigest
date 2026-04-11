package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"rss-platform/internal/repository/postgres/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s-query?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.SourceArticleModel{},
		&models.ArticleProcessingModel{},
		&models.DailyDigestModel{},
		&models.ProfileVersionModel{},
	); err != nil {
		t.Fatalf("auto migrate query tables: %v", err)
	}

	return db
}

func TestArticleQueryServiceListsProcessedArticles(t *testing.T) {
	db := newQueryTestDB(t)
	ctx := context.Background()

	if err := db.WithContext(ctx).Create(&models.SourceArticleModel{
		ID:              "art-1",
		MinifluxEntryID: 101,
		FeedID:          11,
		FeedTitle:       "Tech Feed",
		Title:           "Model News",
		Author:          "Alice",
		URL:             "https://example.com/model-news",
		ContentHTML:     "<p>hello</p>",
		ContentText:     "hello",
		Fingerprint:     "fp-art-1",
	}).Error; err != nil {
		t.Fatalf("create source article: %v", err)
	}

	firstCreatedAt := time.Date(2026, 4, 10, 7, 0, 0, 0, time.UTC)
	secondCreatedAt := firstCreatedAt.Add(time.Minute)
	for _, record := range []models.ArticleProcessingModel{
		{
			ID:                "proc-1",
			ArticleID:         "art-1",
			TitleTranslated:   "旧标题",
			SummaryTranslated: "旧摘要",
			ContentTranslated: "旧正文",
			CoreSummary:       "旧核心观点",
			KeyPointsJSON:     []byte(`["旧要点"]`),
			TopicCategory:     "Old",
			ImportanceScore:   0.5,
			CreatedAt:         firstCreatedAt,
		},
		{
			ID:                "proc-2",
			ArticleID:         "art-1",
			TitleTranslated:   "最新标题",
			SummaryTranslated: "最新摘要",
			ContentTranslated: "最新正文",
			CoreSummary:       "最新核心观点",
			KeyPointsJSON:     []byte(`["要点一","要点二"]`),
			TopicCategory:     "AI",
			ImportanceScore:   0.9,
			CreatedAt:         secondCreatedAt,
		},
	} {
		if err := db.WithContext(ctx).Create(&record).Error; err != nil {
			t.Fatalf("create processing record: %v", err)
		}
	}

	svc := NewArticleQueryService(db)
	items, err := svc.ListArticles(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 1 {
		t.Fatalf("want 1 processed article got %d", len(items))
	}
	if items[0].TitleTranslated != "最新标题" {
		t.Fatalf("want latest translated title got %s", items[0].TitleTranslated)
	}
	if items[0].CoreSummary != "最新核心观点" {
		t.Fatalf("want latest core summary got %s", items[0].CoreSummary)
	}
	if len(items[0].KeyPoints) != 2 {
		t.Fatalf("want 2 key points got %d", len(items[0].KeyPoints))
	}
}

func TestDigestQueryServiceReturnsLatestDigest(t *testing.T) {
	db := newQueryTestDB(t)
	ctx := context.Background()

	for _, digest := range []models.DailyDigestModel{
		{
			ID:              "digest-1",
			DigestDate:      time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			Title:           "昨日日报",
			Subtitle:        "旧副标题",
			ContentMarkdown: "# old",
			ContentHTML:     "<h1>old</h1>",
			RemoteID:        "remote-1",
			RemoteURL:       "https://example.com/old",
			PublishState:    "published",
			PublishError:    "",
			CreatedAt:       time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC),
		},
		{
			ID:              "digest-2",
			DigestDate:      time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
			Title:           "今日日报",
			Subtitle:        "新副标题",
			ContentMarkdown: "# new",
			ContentHTML:     "<h1>new</h1>",
			RemoteID:        "remote-2",
			RemoteURL:       "https://example.com/new",
			PublishState:    "published",
			PublishError:    "",
			CreatedAt:       time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
			UpdatedAt:       time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC),
		},
	} {
		if err := db.WithContext(ctx).Create(&digest).Error; err != nil {
			t.Fatalf("create digest: %v", err)
		}
	}

	svc := NewDigestQueryService(db)
	got, err := svc.LatestDigest(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "今日日报" {
		t.Fatalf("want 今日日报 got %s", got.Title)
	}
	if got.RemoteURL != "https://example.com/new" {
		t.Fatalf("want latest remote url got %s", got.RemoteURL)
	}
	if got.PublishState != "published" {
		t.Fatalf("want published got %s", got.PublishState)
	}
	if got.DigestDate != "2026-04-11" {
		t.Fatalf("want digest date 2026-04-11 got %s", got.DigestDate)
	}
}

func TestProfileQueryServiceReturnsActiveProfilePayload(t *testing.T) {
	db := newQueryTestDB(t)
	ctx := context.Background()

	payload := map[string]any{
		"provider": "openai",
		"model":    "gpt-4.1-mini",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if err := db.WithContext(ctx).Create(&models.ProfileVersionModel{
		ID:          "profile-1",
		ProfileType: "ai",
		Name:        "default-ai",
		Version:     1,
		IsActive:    true,
		PayloadJSON: payloadJSON,
	}).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}

	svc := NewProfileQueryService(db)
	got, err := svc.ActiveProfile(ctx, "ai")
	if err != nil {
		t.Fatal(err)
	}

	if got.Name != "default-ai" {
		t.Fatalf("want default-ai got %s", got.Name)
	}
	if got.Payload["model"] != "gpt-4.1-mini" {
		t.Fatalf("want payload model gpt-4.1-mini got %#v", got.Payload["model"])
	}
}
