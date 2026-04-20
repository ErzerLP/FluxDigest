package postgres_test

import (
	"context"
	"testing"

	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"
)

func TestProcessingRepositorySaveStoresVersionMetadata(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.ArticleProcessingModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewProcessingRepository(db)
	err := repo.Save(context.Background(), postgres.ProcessedArticleRecord{
		ArticleID:                 "art-1",
		TitleTranslated:           "标题",
		SummaryTranslated:         "摘要",
		ContentTranslated:         "全文",
		CoreSummary:               "核心",
		KeyPoints:                 []string{"k1", "k2"},
		TopicCategory:             "AI",
		ImportanceScore:           0.92,
		TranslationPromptVersion:  3,
		AnalysisPromptVersion:     5,
		LLMProfileVersion:         7,
		Status:                    "completed",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetLatestByArticleID(context.Background(), "art-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.TranslationPromptVersion != 3 || got.AnalysisPromptVersion != 5 || got.LLMProfileVersion != 7 {
		t.Fatalf("unexpected version metadata %+v", got)
	}
}
