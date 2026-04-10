package markdown_export_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	"rss-platform/internal/adapter/publisher/markdown_export"
)

func TestPublishDigestRequiresMarkdownContent(t *testing.T) {
	outputDir := t.TempDir()
	p := markdown_export.New(outputDir)

	_, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{
		Title:       "今日 AI 日报",
		ContentHTML: "<h1>HTML only</h1>",
	})
	if err == nil {
		t.Fatal("expected error when markdown content is empty")
	}

	if _, statErr := os.Stat(filepath.Join(outputDir, "今日-AI-日报.md")); !os.IsNotExist(statErr) {
		t.Fatalf("expected markdown file to not be created, got stat err %v", statErr)
	}
}
