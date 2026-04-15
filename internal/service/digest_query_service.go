package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// DigestItemView 表示日报引用条目。
type DigestItemView struct {
	DossierID        string `json:"dossier_id"`
	SectionName      string `json:"section_name"`
	ImportanceBucket string `json:"importance_bucket"`
	Position         int    `json:"position"`
	IsFeatured       bool   `json:"is_featured"`
}

// DigestView 表示最新日报查询结果。
type DigestView struct {
	DigestDate      string           `json:"digest_date"`
	Title           string           `json:"title"`
	Subtitle        string           `json:"subtitle"`
	ContentMarkdown string           `json:"content_markdown"`
	ContentHTML     string           `json:"content_html"`
	RemoteURL       string           `json:"remote_url"`
	PublishState    string           `json:"publish_state"`
	PublishError    string           `json:"publish_error"`
	Items           []DigestItemView `json:"items"`
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
		ID              string
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
			"id",
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

	items, err := s.listDigestItems(ctx, row.ID)
	if err != nil {
		return DigestView{}, err
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
		Items:           items,
	}, nil
}

func (s *DigestQueryService) listDigestItems(ctx context.Context, digestID string) ([]DigestItemView, error) {
	if strings.TrimSpace(digestID) == "" {
		return []DigestItemView{}, nil
	}

	type digestItemRow struct {
		DossierID        string
		SectionName      string
		ImportanceBucket string
		Position         int
		IsFeatured       bool
	}

	rows := []digestItemRow{}
	if err := s.db.WithContext(ctx).
		Table("daily_digest_items").
		Select("dossier_id", "section_name", "importance_bucket", "position", "is_featured").
		Where("digest_id = ?", digestID).
		Order("position ASC").
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]DigestItemView, 0, len(rows))
	for _, row := range rows {
		items = append(items, DigestItemView{
			DossierID:        row.DossierID,
			SectionName:      row.SectionName,
			ImportanceBucket: row.ImportanceBucket,
			Position:         row.Position,
			IsFeatured:       row.IsFeatured,
		})
	}

	return items, nil
}
