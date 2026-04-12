package postgres_test

import (
	"context"
	"testing"

	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"
)

func TestDossierRepositorySaveActiveVersionDeactivatesPrevious(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.ArticleDossierModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewDossierRepository(db)
	ctx := context.Background()

	first, err := repo.SaveActive(ctx, postgres.ArticleDossierRecord{
		ArticleID:                "art-1",
		ProcessingID:             "proc-1",
		DigestDate:               "2026-04-12",
		Version:                  1,
		TitleTranslated:          "标题",
		SummaryPolished:          "润色摘要",
		CoreSummary:              "核心",
		KeyPoints:                []string{"k1"},
		TopicCategory:            "AI",
		ImportanceScore:          0.8,
		ContentPolishedMarkdown:  "## 正文",
		AnalysisLongformMarkdown: "## 分析",
		DebatePoints:             []string{"争议点"},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.SaveActive(ctx, postgres.ArticleDossierRecord{
		ArticleID:                "art-1",
		ProcessingID:             "proc-2",
		DigestDate:               "2026-04-12",
		Version:                  2,
		TitleTranslated:          "标题 v2",
		SummaryPolished:          "润色摘要 v2",
		CoreSummary:              "核心 v2",
		KeyPoints:                []string{"k2"},
		TopicCategory:            "AI",
		ImportanceScore:          0.9,
		ContentPolishedMarkdown:  "## 正文 v2",
		AnalysisLongformMarkdown: "## 分析 v2",
		DebatePoints:             []string{"争议点 v2"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var firstModel models.ArticleDossierModel
	if err := db.WithContext(ctx).Where("id = ?", first.ID).First(&firstModel).Error; err != nil {
		t.Fatalf("load first dossier: %v", err)
	}
	if firstModel.IsActive {
		t.Fatalf("expected first dossier inactive: %+v", firstModel)
	}
	if !second.IsActive {
		t.Fatalf("expected second dossier active: %+v", second)
	}
}
