package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"rss-platform/internal/repository/postgres/models"
)

// DossierListFilter 表示 dossier 列表查询条件。
type DossierListFilter struct {
	Limit int
}

// DossierListItem 表示 dossier 列表项。
type DossierListItem struct {
	ID                string    `json:"id"`
	ArticleID         string    `json:"article_id"`
	DigestDate        string    `json:"digest_date"`
	TitleTranslated   string    `json:"title_translated"`
	CoreSummary       string    `json:"core_summary"`
	TopicCategory     string    `json:"topic_category"`
	ImportanceScore   float64   `json:"importance_score"`
	PriorityLevel     string    `json:"priority_level"`
	PublishSuggestion string    `json:"publish_suggestion"`
	PublishState      string    `json:"publish_state"`
	CreatedAt         time.Time `json:"created_at"`
}

// DossierDetail 表示 dossier 详情。
type DossierDetail struct {
	ID                       string    `json:"id"`
	ArticleID                string    `json:"article_id"`
	ProcessingID             string    `json:"processing_id"`
	DigestDate               string    `json:"digest_date"`
	Version                  int       `json:"version"`
	IsActive                 bool      `json:"is_active"`
	TitleTranslated          string    `json:"title_translated"`
	SummaryPolished          string    `json:"summary_polished"`
	CoreSummary              string    `json:"core_summary"`
	KeyPoints                []string  `json:"key_points"`
	TopicCategory            string    `json:"topic_category"`
	ImportanceScore          float64   `json:"importance_score"`
	RecommendationReason     string    `json:"recommendation_reason"`
	ReadingValue             string    `json:"reading_value"`
	PriorityLevel            string    `json:"priority_level"`
	ContentPolishedMarkdown  string    `json:"content_polished_markdown"`
	AnalysisLongformMarkdown string    `json:"analysis_longform_markdown"`
	BackgroundContext        string    `json:"background_context"`
	ImpactAnalysis           string    `json:"impact_analysis"`
	DebatePoints             []string  `json:"debate_points"`
	TargetAudience           string    `json:"target_audience"`
	PublishSuggestion        string    `json:"publish_suggestion"`
	SuggestionReason         string    `json:"suggestion_reason"`
	SuggestedChannels        []string  `json:"suggested_channels"`
	SuggestedTags            []string  `json:"suggested_tags"`
	SuggestedCategories      []string  `json:"suggested_categories"`
	PublishState             string    `json:"publish_state"`
	PublishChannel           string    `json:"publish_channel"`
	RemoteURL                string    `json:"remote_url"`
	ErrorMessage             string    `json:"error_message"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// DossierQueryService 负责读取 dossier 资产。
type DossierQueryService struct {
	db *gorm.DB
}

// NewDossierQueryService 创建 DossierQueryService。
func NewDossierQueryService(db *gorm.DB) *DossierQueryService {
	return &DossierQueryService{db: db}
}

// ListDossiers 返回活跃 dossier 列表。
func (s *DossierQueryService) ListDossiers(ctx context.Context, filter DossierListFilter) ([]DossierListItem, error) {
	if s == nil || s.db == nil {
		return []DossierListItem{}, nil
	}

	type dossierListRow struct {
		ID                string
		ArticleID         string
		DigestDate        time.Time
		TitleTranslated   string
		CoreSummary       string
		TopicCategory     string
		ImportanceScore   float64
		PriorityLevel     string
		PublishSuggestion string
		PublishState      string
		CreatedAt         string
	}

	rows := []dossierListRow{}
	if err := s.db.WithContext(ctx).
		Table("article_dossiers AS d").
		Select(
			"d.id",
			"d.article_id",
			"d.digest_date",
			"d.title_translated",
			"d.core_summary",
			"d.topic_category",
			"d.importance_score",
			"d.priority_level",
			"d.publish_suggestion",
			"COALESCE(ps.state, 'draft') AS publish_state",
			"d.created_at",
		).
		Joins("LEFT JOIN article_publish_states ps ON ps.dossier_id = d.id").
		Where("d.is_active = ?", true).
		Order("d.created_at DESC").
		Limit(normalizeDossierListLimit(filter.Limit)).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]DossierListItem, 0, len(rows))
	for _, row := range rows {
		createdAt, err := parseQueryTime(row.CreatedAt)
		if err != nil {
			return nil, err
		}

		items = append(items, DossierListItem{
			ID:                row.ID,
			ArticleID:         row.ArticleID,
			DigestDate:        row.DigestDate.Format("2006-01-02"),
			TitleTranslated:   row.TitleTranslated,
			CoreSummary:       row.CoreSummary,
			TopicCategory:     row.TopicCategory,
			ImportanceScore:   row.ImportanceScore,
			PriorityLevel:     row.PriorityLevel,
			PublishSuggestion: row.PublishSuggestion,
			PublishState:      row.PublishState,
			CreatedAt:         createdAt,
		})
	}

	return items, nil
}

// GetDossier 返回 dossier 详情。
func (s *DossierQueryService) GetDossier(ctx context.Context, dossierID string) (DossierDetail, error) {
	if s == nil || s.db == nil {
		return DossierDetail{}, nil
	}

	var dossierModel models.ArticleDossierModel
	if err := s.db.WithContext(ctx).Where("id = ?", dossierID).First(&dossierModel).Error; err != nil {
		return DossierDetail{}, err
	}

	var publishModel models.ArticlePublishStateModel
	if err := s.db.WithContext(ctx).Where("dossier_id = ?", dossierID).First(&publishModel).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return DossierDetail{}, err
	}

	keyPoints, err := decodeStringSlice(dossierModel.KeyPointsJSON)
	if err != nil {
		return DossierDetail{}, err
	}
	debatePoints, err := decodeStringSlice(dossierModel.DebatePointsJSON)
	if err != nil {
		return DossierDetail{}, err
	}
	suggestedChannels, err := decodeStringSlice(dossierModel.SuggestedChannelsJSON)
	if err != nil {
		return DossierDetail{}, err
	}
	suggestedTags, err := decodeStringSlice(dossierModel.SuggestedTagsJSON)
	if err != nil {
		return DossierDetail{}, err
	}
	suggestedCategories, err := decodeStringSlice(dossierModel.SuggestedCategoriesJSON)
	if err != nil {
		return DossierDetail{}, err
	}

	publishState := strings.TrimSpace(publishModel.State)
	if publishState == "" {
		publishState = "draft"
	}

	return DossierDetail{
		ID:                       dossierModel.ID,
		ArticleID:                dossierModel.ArticleID,
		ProcessingID:             dossierModel.ProcessingID,
		DigestDate:               dossierModel.DigestDate.Format("2006-01-02"),
		Version:                  dossierModel.Version,
		IsActive:                 dossierModel.IsActive,
		TitleTranslated:          dossierModel.TitleTranslated,
		SummaryPolished:          dossierModel.SummaryPolished,
		CoreSummary:              dossierModel.CoreSummary,
		KeyPoints:                keyPoints,
		TopicCategory:            dossierModel.TopicCategory,
		ImportanceScore:          dossierModel.ImportanceScore,
		RecommendationReason:     dossierModel.RecommendationReason,
		ReadingValue:             dossierModel.ReadingValue,
		PriorityLevel:            dossierModel.PriorityLevel,
		ContentPolishedMarkdown:  dossierModel.ContentPolishedMarkdown,
		AnalysisLongformMarkdown: dossierModel.AnalysisLongformMarkdown,
		BackgroundContext:        dossierModel.BackgroundContext,
		ImpactAnalysis:           dossierModel.ImpactAnalysis,
		DebatePoints:             debatePoints,
		TargetAudience:           dossierModel.TargetAudience,
		PublishSuggestion:        dossierModel.PublishSuggestion,
		SuggestionReason:         dossierModel.SuggestionReason,
		SuggestedChannels:        suggestedChannels,
		SuggestedTags:            suggestedTags,
		SuggestedCategories:      suggestedCategories,
		PublishState:             publishState,
		PublishChannel:           publishModel.PublishChannel,
		RemoteURL:                publishModel.RemoteURL,
		ErrorMessage:             publishModel.ErrorMessage,
		CreatedAt:                dossierModel.CreatedAt,
		UpdatedAt:                dossierModel.UpdatedAt,
	}, nil
}

func normalizeDossierListLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 20
	}
	return limit
}

func decodeStringSlice(raw []byte) ([]string, error) {
	out := []string{}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseQueryTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}

	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("parse query time %q", value)
}
