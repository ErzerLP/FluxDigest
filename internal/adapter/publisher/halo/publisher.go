package halo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	adapterpublisher "rss-platform/internal/adapter/publisher"
)

const (
	postAPIVersion   = "content.halo.run/v1alpha1"
	postKind         = "Post"
	postVisibility   = "PUBLIC"
	postRawTypeHTML  = "html"
	postRawTypeMD    = "markdown"
	defaultPostTitle = "FluxDigest Daily Digest"
)

// Publisher 负责把日报发布到 Halo 官方 Console API。
type Publisher struct {
	baseURL    string
	token      string
	httpClient *http.Client
	now        func() time.Time
}

// New 创建 Halo 官方发布器。
func New(baseURL, token string) *Publisher {
	return &Publisher{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		now: time.Now,
	}
}

// Name 返回发布器名称。
func (p *Publisher) Name() string { return "halo" }

// PublishDigest 通过 Halo Console API 创建并发布日报文章。
func (p *Publisher) PublishDigest(ctx context.Context, req adapterpublisher.PublishDigestRequest) (adapterpublisher.PublishDigestResult, error) {
	draftPayload, postName := p.buildDraftPayload(req)

	created, err := p.postDraft(ctx, draftPayload)
	if err != nil {
		return adapterpublisher.PublishDigestResult{}, err
	}

	if strings.TrimSpace(created.Metadata.Name) != "" {
		postName = strings.TrimSpace(created.Metadata.Name)
	}
	if postName == "" {
		return adapterpublisher.PublishDigestResult{}, adapterpublisher.NewAmbiguousPublishError(fmt.Errorf("halo draft response missing metadata.name"))
	}

	published, err := p.publishPost(ctx, postName)
	if err != nil {
		return adapterpublisher.PublishDigestResult{}, err
	}

	return adapterpublisher.PublishDigestResult{
		RemoteID:  postName,
		RemoteURL: strings.TrimSpace(published.Status.Permalink),
	}, nil
}

func (p *Publisher) postDraft(ctx context.Context, payload postRequest) (postEnvelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return postEnvelope{}, adapterpublisher.NewRetryablePublishError(err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/apis/api.console.halo.run/v1alpha1/posts", bytes.NewReader(body))
	if err != nil {
		return postEnvelope{}, adapterpublisher.NewRetryablePublishError(err)
	}

	var out postEnvelope
	if err := p.doJSON(httpReq, &out, "halo draft post", false); err != nil {
		return postEnvelope{}, err
	}
	return out, nil
}

func (p *Publisher) publishPost(ctx context.Context, postName string) (postEnvelope, error) {
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		p.baseURL+"/apis/api.console.halo.run/v1alpha1/posts/"+url.PathEscape(postName)+"/publish",
		nil,
	)
	if err != nil {
		return postEnvelope{}, adapterpublisher.NewRetryablePublishError(err)
	}

	var out postEnvelope
	if err := p.doJSON(httpReq, &out, "halo publish post", true); err != nil {
		return postEnvelope{}, err
	}
	return out, nil
}

func (p *Publisher) doJSON(httpReq *http.Request, out any, action string, ambiguousOnNon2xx bool) error {
	httpReq.Header.Set("Content-Type", "application/json")
	if p.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return adapterpublisher.NewAmbiguousPublishError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("%s failed: status=%d body=%s", action, resp.StatusCode, strings.TrimSpace(string(body)))
		if ambiguousOnNon2xx {
			return adapterpublisher.NewAmbiguousPublishError(err)
		}
		return adapterpublisher.NewRetryablePublishError(err)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return adapterpublisher.NewAmbiguousPublishError(err)
	}
	return nil
}

func (p *Publisher) buildDraftPayload(req adapterpublisher.PublishDigestRequest) (postRequest, string) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = defaultPostTitle
	}

	postName, postSlug := buildPostIdentity(title, p.now())
	content, raw, rawType := resolveContent(req.ContentMarkdown, req.ContentHTML)
	excerpt := excerptSpec{AutoGenerate: true}
	if subtitle := strings.TrimSpace(req.Subtitle); subtitle != "" {
		// Halo 没有独立 subtitle 字段，这里把日报副标题收敛到 excerpt.raw，
		// 既能保留摘要位信息，也不会污染正文 Markdown/HTML。
		excerpt = excerptSpec{
			AutoGenerate: false,
			Raw:          subtitle,
		}
	}

	return postRequest{
		Content: contentUpdateParam{
			Content: content,
			Raw:     raw,
			RawType: rawType,
		},
		Post: postEnvelope{
			APIVersion: postAPIVersion,
			Kind:       postKind,
			Metadata: metadata{
				Name: postName,
			},
			Spec: postSpec{
				Title:        title,
				Slug:         postSlug,
				AllowComment: true,
				Deleted:      false,
				Excerpt:      excerpt,
				Pinned:       false,
				Priority:     0,
				Publish:      false,
				Tags:         cleanTags(req.Tags),
				Visible:      postVisibility,
			},
		},
	}, postName
}

func resolveContent(markdown, html string) (content string, raw string, rawType string) {
	cleanMarkdown := strings.TrimSpace(markdown)
	cleanHTML := strings.TrimSpace(html)

	if cleanMarkdown != "" {
		content = cleanHTML
		if content == "" {
			content = cleanMarkdown
		}
		return content, cleanMarkdown, postRawTypeMD
	}

	if cleanHTML != "" {
		return cleanHTML, cleanHTML, postRawTypeHTML
	}

	return "", "", postRawTypeMD
}

func cleanTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, item := range tags {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func buildPostIdentity(title string, now time.Time) (name string, slug string) {
	timestamp := now.UTC()
	name = fmt.Sprintf("fluxdigest-%d", timestamp.UnixNano())

	slugBase := slugifyASCII(title)
	if slugBase == "" {
		slugBase = "daily-digest"
	}
	slug = fmt.Sprintf("%s-%s", slugBase, timestamp.Format("20060102-150405"))
	return name, slug
}

func slugifyASCII(title string) string {
	var builder strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			builder.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/' || r == '\\':
			if lastDash || builder.Len() == 0 {
				continue
			}
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

type postRequest struct {
	Content contentUpdateParam `json:"content"`
	Post    postEnvelope       `json:"post"`
}

type contentUpdateParam struct {
	Content string `json:"content"`
	Raw     string `json:"raw"`
	RawType string `json:"rawType"`
}

type postEnvelope struct {
	APIVersion string     `json:"apiVersion,omitempty"`
	Kind       string     `json:"kind,omitempty"`
	Metadata   metadata   `json:"metadata"`
	Spec       postSpec   `json:"spec"`
	Status     postStatus `json:"status,omitempty"`
}

type metadata struct {
	Name string `json:"name"`
}

type postSpec struct {
	AllowComment bool        `json:"allowComment"`
	Deleted      bool        `json:"deleted"`
	Excerpt      excerptSpec `json:"excerpt"`
	Pinned       bool        `json:"pinned"`
	Priority     int         `json:"priority"`
	Publish      bool        `json:"publish"`
	Slug         string      `json:"slug"`
	Tags         []string    `json:"tags,omitempty"`
	Title        string      `json:"title"`
	Visible      string      `json:"visible"`
}

type excerptSpec struct {
	AutoGenerate bool   `json:"autoGenerate"`
	Raw          string `json:"raw,omitempty"`
}

type postStatus struct {
	Permalink string `json:"permalink,omitempty"`
}
