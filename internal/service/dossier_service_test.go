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

func TestDossierServiceMaterializeStoresNormalizedSuggestionAndPublishState(t *testing.T) {
	builder := dossierBuilderStub{out: dossier.ArticleDossier{TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, RecommendationReason: "具备长期影响", ReadingValue: "适合持续关注", PriorityLevel: "high", ContentPolishedMarkdown: "## 正文", AnalysisLongformMarkdown: "## 分析", DebatePoints: []string{"争议点"}, PublishSuggestion: "suggested", SuggestionReason: "高价值", SuggestedChannels: []string{"halo"}}}
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
	if len(publishStates.saved) != 1 || publishStates.saved[0].State != "draft" {
		t.Fatalf("unexpected publish states %+v", publishStates.saved)
	}
}

func TestDossierServiceMaterializePreservesNaturalLanguageSuggestionReason(t *testing.T) {
	natural := "这篇文章在 AI 与自动化交汇处提供了独到洞察，建议立即纳入日报"
	builder := dossierBuilderStub{out: dossier.ArticleDossier{TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, PublishSuggestion: natural}}
	dossiers := &dossierRepoStub{}
	publishStates := &publishStateRepoStub{}
	svc := service.NewDossierService(builder, dossiers, publishStates)

	out, err := svc.Materialize(context.Background(), service.MaterializeDossierInput{ArticleID: "art-1", ProcessingID: "proc-1", DigestDate: "2026-04-12", TitleTranslated: "模型新闻", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, TranslationPromptVersion: 6, AnalysisPromptVersion: 6, DossierPromptVersion: 6, LLMProfileVersion: 4})
	if err != nil {
		t.Fatal(err)
	}
	if out.PublishSuggestion != "suggested" {
		t.Fatalf("want normalized suggested got %q", out.PublishSuggestion)
	}
	if out.SuggestionReason != natural {
		t.Fatalf("expected suggestion reason to keep natural text got %q", out.SuggestionReason)
	}
	if len(publishStates.saved) != 1 || publishStates.saved[0].State != "draft" {
		t.Fatalf("unexpected publish states %+v", publishStates.saved)
	}
}

func TestDossierServiceMaterializeTreatsNegativeSuggestionAsDraft(t *testing.T) {
	builder := dossierBuilderStub{out: dossier.ArticleDossier{TitleTranslated: "模型新闻", SummaryPolished: "润色摘要", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, PublishSuggestion: "暂缓观察"}}
	dossiers := &dossierRepoStub{}
	publishStates := &publishStateRepoStub{}
	svc := service.NewDossierService(builder, dossiers, publishStates)

	out, err := svc.Materialize(context.Background(), service.MaterializeDossierInput{ArticleID: "art-1", ProcessingID: "proc-1", DigestDate: "2026-04-12", TitleTranslated: "模型新闻", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, TranslationPromptVersion: 6, AnalysisPromptVersion: 6, DossierPromptVersion: 6, LLMProfileVersion: 4})
	if err != nil {
		t.Fatal(err)
	}
	if out.PublishSuggestion != "draft" {
		t.Fatalf("want draft got %q", out.PublishSuggestion)
	}
	if len(publishStates.saved) != 1 || publishStates.saved[0].State != "draft" {
		t.Fatalf("unexpected publish states %+v", publishStates.saved)
	}
}

func TestDossierServiceMaterializeHandlesMixedEnglishPublishSignals(t *testing.T) {
	testCases := []struct {
		name               string
		rawSuggestion      string
		wantSuggestion     string
		wantSuggestionKeep bool
	}{
		{
			name:               "not ready to publish yet stays draft",
			rawSuggestion:      "not ready to publish yet",
			wantSuggestion:     "draft",
			wantSuggestionKeep: true,
		},
		{
			name:               "wait before publish stays draft",
			rawSuggestion:      "wait before publish",
			wantSuggestion:     "draft",
			wantSuggestionKeep: true,
		},
		{
			name:               "ready for review not for publish stays draft",
			rawSuggestion:      "ready for review, not for publish",
			wantSuggestion:     "draft",
			wantSuggestionKeep: true,
		},
		{
			name:               "worth publishing becomes suggested",
			rawSuggestion:      "worth publishing today",
			wantSuggestion:     "suggested",
			wantSuggestionKeep: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := dossierBuilderStub{out: dossier.ArticleDossier{
				TitleTranslated:   "模型新闻",
				SummaryPolished:   "润色摘要",
				CoreSummary:       "核心总结",
				KeyPoints:         []string{"k1", "k2"},
				TopicCategory:     "AI",
				ImportanceScore:   0.91,
				PublishSuggestion: tc.rawSuggestion,
			}}
			dossiers := &dossierRepoStub{}
			publishStates := &publishStateRepoStub{}
			svc := service.NewDossierService(builder, dossiers, publishStates)

			out, err := svc.Materialize(context.Background(), service.MaterializeDossierInput{ArticleID: "art-1", ProcessingID: "proc-1", DigestDate: "2026-04-12", TitleTranslated: "模型新闻", CoreSummary: "核心总结", KeyPoints: []string{"k1", "k2"}, TopicCategory: "AI", ImportanceScore: 0.91, TranslationPromptVersion: 6, AnalysisPromptVersion: 6, DossierPromptVersion: 6, LLMProfileVersion: 4})
			if err != nil {
				t.Fatal(err)
			}
			if out.PublishSuggestion != tc.wantSuggestion {
				t.Fatalf("want publish suggestion %q got %q", tc.wantSuggestion, out.PublishSuggestion)
			}
			if len(publishStates.saved) != 1 || publishStates.saved[0].State != "draft" {
				t.Fatalf("unexpected publish states %+v", publishStates.saved)
			}
			if tc.wantSuggestionKeep && out.SuggestionReason != tc.rawSuggestion {
				t.Fatalf("want suggestion reason %q got %q", tc.rawSuggestion, out.SuggestionReason)
			}
		})
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
