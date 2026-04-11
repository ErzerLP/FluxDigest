package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// DigestView 表示最新日报查询结果。
type DigestView struct {
	DigestDate      string `json:"digest_date"`
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	ContentMarkdown string `json:"content_markdown"`
	ContentHTML     string `json:"content_html"`
	RemoteURL       string `json:"remote_url"`
	PublishState    string `json:"publish_state"`
	PublishError    string `json:"publish_error"`
}

// DigestQueryService 负责读取最新日报结果。
type DigestQueryService struct {
	db *gorm.DB
}

// NewDigestQueryService 创建 DigestQueryService。
func NewDigestQueryService(db *gorm.DB) *DigestQueryService {
	return &DigestQueryService{db: db}
}

// LatestDigest 返回最近一个 digest_date 的日报结果。
func (s *DigestQueryService) LatestDigest(ctx context.Context) (DigestView, error) {
	if s == nil || s.db == nil {
		return DigestView{}, nil
	}

	type digestQueryRow struct {
		DigestDate      string
		Title           string
		Subtitle        string
		ContentMarkdown string
		ContentHTML     string
		RemoteURL       string
		PublishState    string
		PublishError    string
	}

	var row digestQueryRow
	err := s.db.WithContext(ctx).
		Table("daily_digests").
		Select(
			"CAST(digest_date AS TEXT) AS digest_date",
			"title",
			"subtitle",
			"content_markdown",
			"content_html",
			"remote_url",
			"publish_state",
			"publish_error",
		).
		Order("digest_date DESC").
		Order("updated_at DESC").
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DigestView{}, nil
		}
		return DigestView{}, err
	}

	digestDate := strings.TrimSpace(row.DigestDate)
	if idx := strings.IndexByte(digestDate, ' '); idx >= 0 {
		digestDate = digestDate[:idx]
	}

	return DigestView{
		DigestDate:      digestDate,
		Title:           row.Title,
		Subtitle:        row.Subtitle,
		ContentMarkdown: row.ContentMarkdown,
		ContentHTML:     row.ContentHTML,
		RemoteURL:       row.RemoteURL,
		PublishState:    row.PublishState,
		PublishError:    row.PublishError,
	}, nil
}
