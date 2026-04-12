package service_test

import (
	"context"
	"testing"

	"rss-platform/internal/domain/dossier"
	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/service"
)

type dossierBuilderStub struct {
	out dossier.ArticleDossier
	err error
}

func (s dossierBuilderStub) Build(_ context.Context, _ service.BuildDossierInput) (dossier.ArticleDossier, error) {
	return s.out, s.err
}

type dossierRepoStub struct {
	received []postgres.ArticleDossierRecord
	saved    []postgres.ArticleDossierRecord
}

func (s *dossierRepoStub) SaveActive(_ context.Context, input postgres.ArticleDossierRecord) (postgres.ArticleDossierRecord, error) {
	s.received = append(s.received, input)
	if input.Version == 0 {
		nextVersion := 1
		for _, item := range s.saved {
			if item.ArticleID == input.ArticleID && item.Version >= nextVersion {
				nextVersion = item.Version + 1
			}
		}
		input.Version = nextVersion
	}
	s.saved = append(s.saved, input)
	if input.ID == "" {
		input.ID = "dos-1"
		if len(s.saved) > 1 {
			input.ID = "dos-2"
		}
	}
	return input, nil
}

type publishStateRepoStub struct {
	saved []postgres.ArticlePublishStateRecord
}

func (s *publishStateRepoStub) Upsert(_ context.Context, input postgres.ArticlePublishStateRecord) error {
	s.saved = append(s.saved, input)
	return nil
}

func TestDossierServiceMaterializeCreatesSuggestedPublishState(t *testing.T) {
	builder := dossierBuilderStub{out: dossier.ArticleDossier{TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, RecommendationReason: "具备长期影响", ReadingValue: "适合持续关注", PriorityLevel: "high", ContentPolishedMarkdown: "## 正文", AnalysisLongformMarkdown: "## 分析", DebatePoints: []string{"争议点"}, PublishSuggestion: "suggested", SuggestionReason: "高价值", SuggestedChannels: []string{"holo"}}}
	dossiers := &dossierRepoStub{}
	publishStates := &publishStateRepoStub{}
	svc := service.NewDossierService(builder, dossiers, publishStates)

	out, err := svc.Materialize(context.Background(), service.MaterializeDossierInput{ArticleID: "art-1", ProcessingID: "proc-1", DigestDate: "2026-04-12", TitleTranslated: "模型新闻", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, TranslationPromptVersion: 6, AnalysisPromptVersion: 6, DossierPromptVersion: 6, LLMProfileVersion: 4})
	if err != nil {
		t.Fatal(err)
	}
	if out.PublishSuggestion != "suggested" {
		t.Fatalf("want suggested got %q", out.PublishSuggestion)
	}
	if len(publishStates.saved) != 1 || publishStates.saved[0].State != "suggested" {
		t.Fatalf("unexpected publish states %+v", publishStates.saved)
	}
}

func TestDossierServiceMaterializeUsesRepositoryAssignedVersionSequence(t *testing.T) {
	builder := dossierBuilderStub{out: dossier.ArticleDossier{TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.91, ContentPolishedMarkdown: "## 正文", AnalysisLongformMarkdown: "## 分析"}}
	dossiers := &dossierRepoStub{}
	publishStates := &publishStateRepoStub{}
	svc := service.NewDossierService(builder, dossiers, publishStates)

	first, err := svc.Materialize(context.Background(), service.MaterializeDossierInput{ArticleID: "art-1", ProcessingID: "proc-1", DigestDate: "2026-04-12", TitleTranslated: "模型新闻", CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.91, TranslationPromptVersion: 6, AnalysisPromptVersion: 6, DossierPromptVersion: 6, LLMProfileVersion: 4})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Materialize(context.Background(), service.MaterializeDossierInput{ArticleID: "art-1", ProcessingID: "proc-2", DigestDate: "2026-04-12", TitleTranslated: "模型新闻", CoreSummary: "核心总结", KeyPoints: []string{"k1"}, TopicCategory: "AI", ImportanceScore: 0.91, TranslationPromptVersion: 6, AnalysisPromptVersion: 6, DossierPromptVersion: 6, LLMProfileVersion: 4})
	if err != nil {
		t.Fatal(err)
	}

	if first.Version != 1 {
		t.Fatalf("expected first dossier version 1, got %d", first.Version)
	}
	if second.Version != 2 {
		t.Fatalf("expected second dossier version 2, got %d", second.Version)
	}
	if len(dossiers.saved) != 2 {
		t.Fatalf("expected two dossier saves, got %d", len(dossiers.saved))
	}
	if dossiers.received[0].Version != 0 || dossiers.received[1].Version != 0 {
		t.Fatalf("expected service to defer version assignment to repository, got %+v", dossiers.received)
	}
}
