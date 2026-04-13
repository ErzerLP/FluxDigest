package postgres_test

import (
	"context"
	"testing"

	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"
)

func TestPublishStateRepositoryUpsertSuggestion(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.ArticlePublishStateModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewPublishStateRepository(db)
	ctx := context.Background()
	if err := repo.Upsert(ctx, postgres.ArticlePublishStateRecord{DossierID: "dos-1", State: "suggested", DecisionNote: "high score", PublishChannel: "halo"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(ctx, postgres.ArticlePublishStateRecord{DossierID: "dos-1", State: "ignored", DecisionNote: "manual skip", PublishChannel: "halo"}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByDossierID(ctx, "dos-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "ignored" || got.DecisionNote != "manual skip" {
		t.Fatalf("unexpected state %+v", got)
	}
}
