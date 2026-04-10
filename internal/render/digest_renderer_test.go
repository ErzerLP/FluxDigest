package render_test

import (
	"strings"
	"testing"

	domaindigest "rss-platform/internal/domain/digest"
	renderpkg "rss-platform/internal/render"
)

func TestDigestRendererRenderOutputsMarkdownAndHTMLFromPlanOnly(t *testing.T) {
	r := renderpkg.NewDigestRenderer()

	markdown, html, err := r.Render(domaindigest.Plan{
		Title:       "今日 AI 日报",
		Subtitle:    "聚焦模型与产品动态",
		OpeningNote: "以下为今日重点。",
		Sections: []domaindigest.Section{{
			Name: "重点速览",
			Items: []domaindigest.SectionItem{{
				ArticleID:   "art-1",
				Title:       "Model News",
				CoreSummary: "Summary",
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown, "# 今日 AI 日报") {
		t.Fatalf("expected markdown title, got %s", markdown)
	}
	if !strings.Contains(markdown, "Model News") {
		t.Fatalf("expected markdown planned item, got %s", markdown)
	}
	if strings.Contains(markdown, "候选文章") {
		t.Fatalf("renderer should not inject candidate article section, got %s", markdown)
	}
	if !strings.Contains(html, "<h1>今日 AI 日报</h1>") {
		t.Fatalf("expected html title, got %s", html)
	}
	if !strings.Contains(html, "Model News") {
		t.Fatalf("expected html planned item, got %s", html)
	}
}
