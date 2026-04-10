package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestArticleRepositoryUpsertAndFindByMinifluxID(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.SourceArticleModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewArticleRepository(db)
	in := article.SourceArticle{
		MinifluxEntryID: 101,
		FeedID:          11,
		FeedTitle:       "Tech Feed",
		Title:           "Hello",
		Author:          "Alice",
		URL:             "https://example.com",
		ContentHTML:     "<p>Hello</p>",
		ContentText:     "Hello",
		Fingerprint:     "fp-101",
	}
	if err := repo.Upsert(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByMinifluxEntryID(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Hello" {
		t.Fatalf("want Hello got %s", got.Title)
	}
	if got.FeedID != in.FeedID || got.FeedTitle != in.FeedTitle || got.Author != in.Author || got.ContentHTML != in.ContentHTML || got.ContentText != in.ContentText {
		t.Fatalf("unexpected article mapping: %+v", got)
	}
	if got.ID == "" {
		t.Fatal("want generated id, got empty")
	}
}

func TestArticleRepositoryUpsertGeneratesIDForEmptyInput(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.SourceArticleModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewArticleRepository(db)
	ctx := context.Background()

	first := article.SourceArticle{MinifluxEntryID: 101, Title: "A", URL: "https://example.com/a", Fingerprint: "fp-a"}
	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatalf("upsert first: %v", err)
	}
	second := article.SourceArticle{MinifluxEntryID: 102, Title: "B", URL: "https://example.com/b", Fingerprint: "fp-b"}
	if err := repo.Upsert(ctx, second); err != nil {
		t.Fatalf("upsert second: %v", err)
	}

	firstSaved, err := repo.FindByMinifluxEntryID(ctx, 101)
	if err != nil {
		t.Fatalf("find first: %v", err)
	}
	secondSaved, err := repo.FindByMinifluxEntryID(ctx, 102)
	if err != nil {
		t.Fatalf("find second: %v", err)
	}
	if firstSaved.ID == "" || secondSaved.ID == "" {
		t.Fatalf("ids should be generated: first=%q second=%q", firstSaved.ID, secondSaved.ID)
	}
	if firstSaved.ID == secondSaved.ID {
		t.Fatalf("generated ids should be unique, both=%q", firstSaved.ID)
	}
}

func TestProfileRepositoryCreateVersionAndGetActive(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.ProfileVersionModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewProfileRepository(db)
	err := repo.Create(context.Background(), profile.Version{ProfileType: "ai", Name: "default-ai", Version: 1, IsActive: true, PayloadJSON: []byte(`{"model":"gpt-4.1-mini"}`)})
	if err != nil {
		t.Fatal(err)
	}
	active, err := repo.GetActive(context.Background(), "ai")
	if err != nil {
		t.Fatal(err)
	}
	if active.Name != "default-ai" {
		t.Fatalf("want default-ai got %s", active.Name)
	}
	if active.ID == "" {
		t.Fatal("want generated id, got empty")
	}
}

func TestProfileRepositoryCreateActiveVersionSwitchesActive(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.ProfileVersionModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewProfileRepository(db)
	ctx := context.Background()
	if err := repo.Create(ctx, profile.Version{ProfileType: "ai", Name: "default-ai", Version: 1, IsActive: true, PayloadJSON: []byte(`{"model":"gpt-4.1-mini"}`)}); err != nil {
		t.Fatalf("create v1: %v", err)
	}
	if err := repo.Create(ctx, profile.Version{ProfileType: "ai", Name: "default-ai", Version: 2, IsActive: true, PayloadJSON: []byte(`{"model":"gpt-5-mini"}`)}); err != nil {
		t.Fatalf("create v2: %v", err)
	}

	var activeCount int64
	if err := db.Model(&models.ProfileVersionModel{}).Where("profile_type = ? AND is_active = ?", "ai", true).Count(&activeCount).Error; err != nil {
		t.Fatalf("count active: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("want exactly 1 active version, got %d", activeCount)
	}

	active, err := repo.GetActive(ctx, "ai")
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active.Version != 2 {
		t.Fatalf("want version 2 got %d", active.Version)
	}
}
