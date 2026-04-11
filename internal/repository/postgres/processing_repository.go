package postgres

import (
	"context"
	"encoding/json"

	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
)

// ProcessedArticleRecord 表示文章处理落库记录。
type ProcessedArticleRecord struct {
	ID                string
	ArticleID         string
	TitleTranslated   string
	SummaryTranslated string
	ContentTranslated string
	CoreSummary       string
	KeyPoints         []string
	TopicCategory     string
	ImportanceScore   float64
}

// ProcessingRepository 负责保存文章处理结果。
type ProcessingRepository struct {
	db *gorm.DB
}

// NewProcessingRepository 创建 ProcessingRepository。
func NewProcessingRepository(db *gorm.DB) *ProcessingRepository {
	return &ProcessingRepository{db: db}
}

// Save 持久化单篇文章的处理结果。
func (r *ProcessingRepository) Save(ctx context.Context, input ProcessedArticleRecord) error {
	keyPoints := input.KeyPoints
	if keyPoints == nil {
		keyPoints = []string{}
	}

	keyPointsJSON, err := json.Marshal(keyPoints)
	if err != nil {
		return err
	}

	model := models.ArticleProcessingModel{
		ID:                ensureID(input.ID),
		ArticleID:         input.ArticleID,
		TitleTranslated:   input.TitleTranslated,
		SummaryTranslated: input.SummaryTranslated,
		ContentTranslated: input.ContentTranslated,
		CoreSummary:       input.CoreSummary,
		KeyPointsJSON:     keyPointsJSON,
		TopicCategory:     input.TopicCategory,
		ImportanceScore:   input.ImportanceScore,
	}

	return r.db.WithContext(ctx).Create(&model).Error
}

// GetLatestByArticleID 返回文章最新一次处理结果。
func (r *ProcessingRepository) GetLatestByArticleID(ctx context.Context, articleID string) (ProcessedArticleRecord, error) {
	var model models.ArticleProcessingModel
	if err := r.db.WithContext(ctx).
		Where("article_id = ?", articleID).
		Order("created_at DESC").
		Order("id DESC").
		First(&model).Error; err != nil {
		return ProcessedArticleRecord{}, err
	}

	var keyPoints []string
	if err := json.Unmarshal(model.KeyPointsJSON, &keyPoints); err != nil {
		return ProcessedArticleRecord{}, err
	}

	return ProcessedArticleRecord{
		ID:                model.ID,
		ArticleID:         model.ArticleID,
		TitleTranslated:   model.TitleTranslated,
		SummaryTranslated: model.SummaryTranslated,
		ContentTranslated: model.ContentTranslated,
		CoreSummary:       model.CoreSummary,
		KeyPoints:         keyPoints,
		TopicCategory:     model.TopicCategory,
		ImportanceScore:   model.ImportanceScore,
	}, nil
}
