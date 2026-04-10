package publisher

import "context"

// PublishDigestRequest 表示日报发布请求。
type PublishDigestRequest struct {
	Title           string
	Subtitle        string
	ContentMarkdown string
	ContentHTML     string
	Tags            []string
}

// PublishDigestResult 表示日报发布结果。
type PublishDigestResult struct {
	RemoteID  string
	RemoteURL string
}

// Publisher 定义日报发布器最小能力。
type Publisher interface {
	Name() string
	PublishDigest(ctx context.Context, req PublishDigestRequest) (PublishDigestResult, error)
}
