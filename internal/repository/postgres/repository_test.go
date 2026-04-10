package postgres_test

import (
	"context"
	"testing"

	"rss-platform/internal/domain/article"
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestArticleRepositoryUpsertAndFindByMinifluxID(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	_ = db.AutoMigrate(&models.SourceArticleModel{})
	repo := postgres.NewArticleRepository(db)
	err := repo.Upsert(context.Background(), article.SourceArticle{MinifluxEntryID: 101, Title: "Hello", URL: "https://example.com", Fingerprint: "fp-101"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByMinifluxEntryID(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Hello" {
		t.Fatalf("want Hello got %s", got.Title)
	}
}

func TestProfileRepositoryCreateVersionAndActivate(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	_ = db.AutoMigrate(&models.ProfileVersionModel{})
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
}
