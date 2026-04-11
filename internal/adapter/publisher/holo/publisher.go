package holo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
)

// Publisher 负责把日报发布到 Holo 风格的 HTTP 端点。
type Publisher struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

// New 创建最小 Holo 发布器。
func New(endpoint, token string) *Publisher {
	return &Publisher{endpoint: endpoint, token: token, httpClient: &http.Client{Timeout: 15 * time.Second}}
}

// Name 返回发布器名称。
func (p *Publisher) Name() string { return "holo" }

// PublishDigest 通过 HTTP JSON POST 发布日报。
func (p *Publisher) PublishDigest(ctx context.Context, req adapterpublisher.PublishDigestRequest) (adapterpublisher.PublishDigestResult, error) {
	payload, err := json.Marshal(map[string]any{
		"title":            req.Title,
		"subtitle":         req.Subtitle,
		"content_markdown": req.ContentMarkdown,
		"content_html":     req.ContentHTML,
		"tags":             req.Tags,
	})
	if err != nil {
		return adapterpublisher.PublishDigestResult{}, adapterpublisher.NewRetryablePublishError(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return adapterpublisher.PublishDigestResult{}, adapterpublisher.NewRetryablePublishError(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return adapterpublisher.PublishDigestResult{}, adapterpublisher.NewAmbiguousPublishError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return adapterpublisher.PublishDigestResult{}, adapterpublisher.NewRetryablePublishError(fmt.Errorf("holo publish failed: status=%d body=%s", resp.StatusCode, string(body)))
	}

	var out struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return adapterpublisher.PublishDigestResult{}, adapterpublisher.NewAmbiguousPublishError(err)
	}

	return adapterpublisher.PublishDigestResult{RemoteID: out.ID, RemoteURL: out.URL}, nil
}
