package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"

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
