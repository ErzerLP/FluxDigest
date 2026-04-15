package postgres

import (
	"context"
	"encoding/json"
	"time"

	"rss-platform/internal/repository/postgres/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProcessedArticleRecord 表示文章处理落库记录。
type ProcessedArticleRecord struct {
	ID                       string
	ArticleID                string
	TitleTranslated          string
	SummaryTranslated        string
	ContentTranslated        string
	CoreSummary              string
	KeyPoints                []string
	TopicCategory            string
	ImportanceScore          float64
	TranslationPromptVersion int
	AnalysisPromptVersion    int
	LLMProfileVersion        int
	Status                   string
	ErrorMessage             string
	ProcessedAt              time.Time
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
	keyPointsJSON, err := json.Marshal(defaultStringSlice(input.KeyPoints))
	if err != nil {
		return err
	}

	model := models.ArticleProcessingModel{
		ID:                       ensureProcessingID(input.ID),
		ArticleID:                input.ArticleID,
		TitleTranslated:          input.TitleTranslated,
		SummaryTranslated:        input.SummaryTranslated,
		ContentTranslated:        input.ContentTranslated,
		CoreSummary:              input.CoreSummary,
		KeyPointsJSON:            keyPointsJSON,
		TopicCategory:            input.TopicCategory,
		ImportanceScore:          input.ImportanceScore,
		TranslationPromptVersion: defaultPositiveInt(input.TranslationPromptVersion, 1),
		AnalysisPromptVersion:    defaultPositiveInt(input.AnalysisPromptVersion, 1),
		LLMProfileVersion:        defaultPositiveInt(input.LLMProfileVersion, 1),
		Status:                   defaultString(input.Status, "completed"),
		ErrorMessage:             input.ErrorMessage,
		ProcessedAt:              defaultTime(input.ProcessedAt, time.Now()),
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
		ID:                       model.ID,
		ArticleID:                model.ArticleID,
		TitleTranslated:          model.TitleTranslated,
		SummaryTranslated:        model.SummaryTranslated,
		ContentTranslated:        model.ContentTranslated,
		CoreSummary:              model.CoreSummary,
		KeyPoints:                keyPoints,
		TopicCategory:            model.TopicCategory,
		ImportanceScore:          model.ImportanceScore,
		TranslationPromptVersion: model.TranslationPromptVersion,
		AnalysisPromptVersion:    model.AnalysisPromptVersion,
		LLMProfileVersion:        model.LLMProfileVersion,
		Status:                   model.Status,
		ErrorMessage:             model.ErrorMessage,
		ProcessedAt:              model.ProcessedAt,
	}, nil
}

func ensureProcessingID(id string) string {
	if id != "" {
		return id
	}

	orderedID, err := uuid.NewV7()
	if err != nil {
		return ensureID("")
	}

	return orderedID.String()
}

func defaultStringSlice(input []string) []string {
	if input == nil {
		return []string{}
	}
	return input
}

func defaultPositiveInt(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}
