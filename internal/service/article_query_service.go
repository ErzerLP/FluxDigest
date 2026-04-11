package service

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"
)

// ArticleView 表示对外暴露的已处理文章摘要。
type ArticleView struct {
	ID                string   `json:"id"`
	MinifluxEntryID   int64    `json:"miniflux_entry_id"`
	FeedID            int64    `json:"feed_id"`
	FeedTitle         string   `json:"feed_title"`
	Title             string   `json:"title"`
	Author            string   `json:"author"`
	URL               string   `json:"url"`
	TitleTranslated   string   `json:"title_translated"`
	SummaryTranslated string   `json:"summary_translated"`
	ContentTranslated string   `json:"content_translated"`
	CoreSummary       string   `json:"core_summary"`
	KeyPoints         []string `json:"key_points"`
	TopicCategory     string   `json:"topic_category"`
	ImportanceScore   float64  `json:"importance_score"`
}

// ArticleQueryService 负责读取最新处理结果可见的文章列表。
type ArticleQueryService struct {
	db *gorm.DB
}

// NewArticleQueryService 创建 ArticleQueryService。
func NewArticleQueryService(db *gorm.DB) *ArticleQueryService {
	return &ArticleQueryService{db: db}
}

// ListArticles 返回带最新翻译/分析结果的文章列表。
func (s *ArticleQueryService) ListArticles(ctx context.Context) ([]ArticleView, error) {
	if s == nil || s.db == nil {
		return []ArticleView{}, nil
	}

	const query = `
SELECT
  sa.id,
  sa.miniflux_entry_id,
  sa.feed_id,
  sa.feed_title,
  sa.title,
  sa.author,
  sa.url,
  latest.title_translated,
  latest.summary_translated,
  latest.content_translated,
  latest.core_summary,
  latest.key_points_json,
  latest.topic_category,
  latest.importance_score
FROM source_articles sa
INNER JOIN (
  SELECT *
  FROM (
    SELECT
      ap.*,
      ROW_NUMBER() OVER (PARTITION BY ap.article_id ORDER BY ap.created_at DESC, ap.id DESC) AS rn
    FROM article_processings ap
  ) ranked
  WHERE ranked.rn = 1
) latest ON latest.article_id = sa.id
ORDER BY latest.created_at DESC, sa.miniflux_entry_id DESC
`

	type articleQueryRow struct {
		ID                string
		MinifluxEntryID   int64
		FeedID            int64
		FeedTitle         string
		Title             string
		Author            string
		URL               string
		TitleTranslated   string
		SummaryTranslated string
		ContentTranslated string
		CoreSummary       string
		KeyPointsJSON     []byte
		TopicCategory     string
		ImportanceScore   float64
	}

	rows := []articleQueryRow{}
	if err := s.db.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]ArticleView, 0, len(rows))
	for _, row := range rows {
		keyPoints := []string{}
		if len(row.KeyPointsJSON) > 0 {
			if err := json.Unmarshal(row.KeyPointsJSON, &keyPoints); err != nil {
				return nil, err
			}
		}

		items = append(items, ArticleView{
			ID:                row.ID,
			MinifluxEntryID:   row.MinifluxEntryID,
			FeedID:            row.FeedID,
			FeedTitle:         row.FeedTitle,
			Title:             row.Title,
			Author:            row.Author,
			URL:               row.URL,
			TitleTranslated:   row.TitleTranslated,
			SummaryTranslated: row.SummaryTranslated,
			ContentTranslated: row.ContentTranslated,
			CoreSummary:       row.CoreSummary,
			KeyPoints:         keyPoints,
			TopicCategory:     row.TopicCategory,
			ImportanceScore:   row.ImportanceScore,
		})
	}

	return items, nil
}
