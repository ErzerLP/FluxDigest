package postgres_test

import (
	"context"
	"testing"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	domaindigest "rss-platform/internal/domain/digest"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"
	"rss-platform/internal/workflow/daily_digest_workflow"
)

func TestDigestRepositoryBeginPublishStoresDigestItemsAndVersions(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.DailyDigestModel{}, &models.DailyDigestItemModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewDigestRepository(db)
	ok, err := repo.BeginPublish(context.Background(), "2026-04-12", daily_digest_workflow.Digest{
		Title:               "日报",
		Subtitle:            "副标题",
		ContentMarkdown:     "# 内容",
		ContentHTML:         "<h1>内容</h1>",
		DigestPromptVersion: 6,
		LLMProfileVersion:   4,
		Plan: domaindigest.Plan{
			Sections: []domaindigest.Section{{
				Name: "重点速览",
				Items: []domaindigest.SectionItem{{
					DossierID:        "dos-1",
					ArticleID:        "art-1",
					Title:            "模型新闻",
					CoreSummary:      "核心总结",
					ImportanceBucket: "featured",
					IsFeatured:       true,
				}},
			}},
		},
	})
	if err != nil || !ok {
		t.Fatalf("begin publish failed ok=%v err=%v", ok, err)
	}

	record, err := repo.GetByDigestDate(context.Background(), "2026-04-12")
	if err != nil {
		t.Fatal(err)
	}
	if record.DigestPromptVersion != 6 || record.LLMProfileVersion != 4 {
		t.Fatalf("unexpected digest versions %+v", record)
	}
	items, err := repo.ListItemsByDigestID(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DossierID != "dos-1" {
		t.Fatalf("unexpected digest items %+v", items)
	}
	if items[0].ImportanceBucket != "featured" || !items[0].IsFeatured {
		t.Fatalf("unexpected digest item detail %+v", items[0])
	}
}

func TestDigestRepositoryBeginPublishRejectsMissingDossierTrace(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.DailyDigestModel{}, &models.DailyDigestItemModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewDigestRepository(db)
	_, err := repo.BeginPublish(context.Background(), "2026-04-12", daily_digest_workflow.Digest{
		Title:           "日报",
		ContentMarkdown: "# 内容",
		ContentHTML:     "<h1>内容</h1>",
		Plan: domaindigest.Plan{
			Sections: []domaindigest.Section{{
				Name: "重点速览",
				Items: []domaindigest.SectionItem{{
					ArticleID:   "art-1",
					Title:       "模型新闻",
					CoreSummary: "核心总结",
				}},
			}},
		},
	})
	if err == nil {
		t.Fatal("expected missing dossier trace error")
	}
}

func TestDigestRepositoryBeginPublishReplacesDigestItemsAfterFailedRetry(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.DailyDigestModel{}, &models.DailyDigestItemModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewDigestRepository(db)
	ctx := context.Background()

	ok, err := repo.BeginPublish(ctx, "2026-04-12", daily_digest_workflow.Digest{
		Title:           "日报 v1",
		ContentMarkdown: "# 内容 1",
		ContentHTML:     "<h1>内容1</h1>",
		Plan: domaindigest.Plan{
			Sections: []domaindigest.Section{{
				Name: "重点速览",
				Items: []domaindigest.SectionItem{{
					DossierID:   "dos-1",
					ArticleID:   "art-1",
					Title:       "模型新闻 1",
					CoreSummary: "核心总结 1",
				}},
			}},
		},
	})
	if err != nil || !ok {
		t.Fatalf("begin publish v1 failed ok=%v err=%v", ok, err)
	}
	record, err := repo.GetByDigestDate(ctx, "2026-04-12")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkFailed(ctx, "2026-04-12", "retry"); err != nil {
		t.Fatal(err)
	}

	ok, err = repo.BeginPublish(ctx, "2026-04-12", daily_digest_workflow.Digest{
		Title:           "日报 v2",
		ContentMarkdown: "# 内容 2",
		ContentHTML:     "<h1>内容2</h1>",
		Plan: domaindigest.Plan{
			Sections: []domaindigest.Section{{
				Name: "重点速览",
				Items: []domaindigest.SectionItem{{
					DossierID:   "dos-2",
					ArticleID:   "art-2",
					Title:       "模型新闻 2",
					CoreSummary: "核心总结 2",
				}},
			}},
		},
	})
	if err != nil || !ok {
		t.Fatalf("begin publish v2 failed ok=%v err=%v", ok, err)
	}

	items, err := repo.ListItemsByDigestID(ctx, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one digest item after retry, got %d", len(items))
	}
	if items[0].DossierID != "dos-2" {
		t.Fatalf("expected replaced dossier id dos-2, got %+v", items[0])
	}
}

func TestDigestRepositoryForceRerunRetryPreservesRemoteTraceUntilNewPublishSucceeds(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.DailyDigestModel{}, &models.DailyDigestItemModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewDigestRepository(db)
	ctx := context.Background()
	digestDate := "2026-04-12"

	ok, err := repo.BeginPublish(ctx, digestDate, daily_digest_workflow.Digest{
		Title:           "日报 v1",
		ContentMarkdown: "# 内容 1",
		ContentHTML:     "<h1>内容1</h1>",
	})
	if err != nil || !ok {
		t.Fatalf("begin publish v1 failed ok=%v err=%v", ok, err)
	}
	if err := repo.MarkPublished(ctx, digestDate, adapterpublisher.PublishDigestResult{
		RemoteID:  "remote-old",
		RemoteURL: "https://example.com/old",
	}); err != nil {
		t.Fatalf("mark published old: %v", err)
	}

	if err := repo.MarkFailed(ctx, digestDate, "force rerun requested"); err != nil {
		t.Fatalf("mark failed for force rerun: %v", err)
	}
	ok, err = repo.BeginPublish(ctx, digestDate, daily_digest_workflow.Digest{
		Title:           "日报 v2",
		ContentMarkdown: "# 内容 2",
		ContentHTML:     "<h1>内容2</h1>",
	})
	if err != nil || !ok {
		t.Fatalf("begin publish v2 failed ok=%v err=%v", ok, err)
	}

	if err := repo.MarkFailed(ctx, digestDate, "publish failed"); err != nil {
		t.Fatalf("mark failed after force rerun publish error: %v", err)
	}

	record, err := repo.GetByDigestDate(ctx, digestDate)
	if err != nil {
		t.Fatal(err)
	}
	if record.RemoteID != "remote-old" {
		t.Fatalf("want preserved remote id remote-old got %s", record.RemoteID)
	}
	if record.RemoteURL != "https://example.com/old" {
		t.Fatalf("want preserved remote url got %s", record.RemoteURL)
	}
}

func TestDigestRepositoryForceRerunAmbiguousFailurePreservesRemoteTrace(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.DailyDigestModel{}, &models.DailyDigestItemModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := postgres.NewDigestRepository(db)
	ctx := context.Background()
	digestDate := "2026-04-12"

	ok, err := repo.BeginPublish(ctx, digestDate, daily_digest_workflow.Digest{
		Title:           "日报 v1",
		ContentMarkdown: "# 内容 1",
		ContentHTML:     "<h1>内容1</h1>",
	})
	if err != nil || !ok {
		t.Fatalf("begin publish v1 failed ok=%v err=%v", ok, err)
	}
	if err := repo.MarkPublished(ctx, digestDate, adapterpublisher.PublishDigestResult{
		RemoteID:  "remote-old",
		RemoteURL: "https://example.com/old",
	}); err != nil {
		t.Fatalf("mark published old: %v", err)
	}

	if err := repo.MarkFailed(ctx, digestDate, "force rerun requested"); err != nil {
		t.Fatalf("mark failed for force rerun: %v", err)
	}
	ok, err = repo.BeginPublish(ctx, digestDate, daily_digest_workflow.Digest{
		Title:           "日报 v2",
		ContentMarkdown: "# 内容 2",
		ContentHTML:     "<h1>内容2</h1>",
	})
	if err != nil || !ok {
		t.Fatalf("begin publish v2 failed ok=%v err=%v", ok, err)
	}

	if err := repo.MarkRecoveryRequired(ctx, digestDate, adapterpublisher.PublishDigestResult{}, "network timeout"); err != nil {
		t.Fatalf("mark recovery required after ambiguous error: %v", err)
	}

	record, err := repo.GetByDigestDate(ctx, digestDate)
	if err != nil {
		t.Fatal(err)
	}
	if record.PublishState != "recovery_required" {
		t.Fatalf("want recovery_required got %s", record.PublishState)
	}
	if record.RemoteID != "remote-old" {
		t.Fatalf("want preserved remote id remote-old got %s", record.RemoteID)
	}
	if record.RemoteURL != "https://example.com/old" {
		t.Fatalf("want preserved remote url got %s", record.RemoteURL)
	}
}
