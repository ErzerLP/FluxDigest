package markdown_export

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	adapterpublisher "rss-platform/internal/adapter/publisher"
)

// Publisher 负责把日报落地为本地 Markdown 文件。
type Publisher struct {
	outputDir string
}

var errMarkdownRequired = errors.New("markdown content is required")

// New 创建 Markdown 导出发布器。
func New(outputDir string) *Publisher {
	return &Publisher{outputDir: outputDir}
}

// Name 返回发布器名称。
func (p *Publisher) Name() string { return "markdown_export" }

// PublishDigest 把日报内容写入本地 Markdown 文件。
func (p *Publisher) PublishDigest(ctx context.Context, req adapterpublisher.PublishDigestRequest) (adapterpublisher.PublishDigestResult, error) {
	if err := ctx.Err(); err != nil {
		return adapterpublisher.PublishDigestResult{}, err
	}
	if err := os.MkdirAll(p.outputDir, 0o755); err != nil {
		return adapterpublisher.PublishDigestResult{}, err
	}

	fileName := sanitizeFileName(req.Title)
	if fileName == "" {
		fileName = "digest"
	}

	path := filepath.Join(p.outputDir, fileName+".md")
	content := req.ContentMarkdown
	if content == "" {
		return adapterpublisher.PublishDigestResult{}, errMarkdownRequired
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return adapterpublisher.PublishDigestResult{}, err
	}

	return adapterpublisher.PublishDigestResult{RemoteID: path, RemoteURL: path}, nil
}

func sanitizeFileName(title string) string {
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return strings.Trim(replacer.Replace(strings.TrimSpace(title)), "-")
}
