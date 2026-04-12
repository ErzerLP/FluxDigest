package postgres

import (
	"context"
	"encoding/json"
	"time"

	"rss-platform/internal/repository/postgres/models"

	"gorm.io/gorm"
)

const dossierDateLayout = "2006-01-02"

// ArticleDossierRecord 表示 dossier 持久化记录。
type ArticleDossierRecord struct {
	ID                       string
	ArticleID                string
	ProcessingID             string
	DigestDate               string
	Version                  int
	IsActive                 bool
	TitleTranslated          string
	SummaryPolished          string
	CoreSummary              string
	KeyPoints                []string
	TopicCategory            string
	ImportanceScore          float64
	RecommendationReason     string
	ReadingValue             string
	PriorityLevel            string
	ContentPolishedMarkdown  string
	AnalysisLongformMarkdown string
	BackgroundContext        string
	ImpactAnalysis           string
	DebatePoints             []string
	TargetAudience           string
	PublishSuggestion        string
	SuggestionReason         string
	SuggestedChannels        []string
	SuggestedTags            []string
	SuggestedCategories      []string
	TranslationPromptVersion int
	AnalysisPromptVersion    int
	DossierPromptVersion     int
	LLMProfileVersion        int
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// DossierRepository 负责管理文章 dossier 版本。
type DossierRepository struct {
	db *gorm.DB
}

// NewDossierRepository 创建 DossierRepository。
func NewDossierRepository(db *gorm.DB) *DossierRepository {
	return &DossierRepository{db: db}
}

// SaveActive 将当前输入保存为 active，并关闭同文章旧 active 版本。
func (r *DossierRepository) SaveActive(ctx context.Context, input ArticleDossierRecord) (ArticleDossierRecord, error) {
	model, err := dossierRecordToModel(input)
	if err != nil {
		return ArticleDossierRecord{}, err
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ArticleDossierModel{}).
			Where("article_id = ? AND is_active = ?", model.ArticleID, true).
			Updates(map[string]any{"is_active": false, "updated_at": time.Now()}).Error; err != nil {
			return err
		}

		return tx.Create(&model).Error
	})
	if err != nil {
		return ArticleDossierRecord{}, err
	}

	return dossierModelToRecord(model)
}

func dossierRecordToModel(input ArticleDossierRecord) (models.ArticleDossierModel, error) {
	digestDate, err := time.Parse(dossierDateLayout, input.DigestDate)
	if err != nil {
		return models.ArticleDossierModel{}, err
	}

	keyPointsJSON, err := json.Marshal(defaultStringSlice(input.KeyPoints))
	if err != nil {
		return models.ArticleDossierModel{}, err
	}
	debatePointsJSON, err := json.Marshal(defaultStringSlice(input.DebatePoints))
	if err != nil {
		return models.ArticleDossierModel{}, err
	}
	suggestedChannelsJSON, err := json.Marshal(defaultStringSlice(input.SuggestedChannels))
	if err != nil {
		return models.ArticleDossierModel{}, err
	}
	suggestedTagsJSON, err := json.Marshal(defaultStringSlice(input.SuggestedTags))
	if err != nil {
		return models.ArticleDossierModel{}, err
	}
	suggestedCategoriesJSON, err := json.Marshal(defaultStringSlice(input.SuggestedCategories))
	if err != nil {
		return models.ArticleDossierModel{}, err
	}

	now := time.Now()

	return models.ArticleDossierModel{
		ID:                       ensureID(input.ID),
		ArticleID:                input.ArticleID,
		ProcessingID:             input.ProcessingID,
		DigestDate:               digestDate,
		Version:                  defaultPositiveInt(input.Version, 1),
		IsActive:                 true,
		TitleTranslated:          input.TitleTranslated,
		SummaryPolished:          input.SummaryPolished,
		CoreSummary:              input.CoreSummary,
		KeyPointsJSON:            keyPointsJSON,
		TopicCategory:            input.TopicCategory,
		ImportanceScore:          input.ImportanceScore,
		RecommendationReason:     input.RecommendationReason,
		ReadingValue:             input.ReadingValue,
		PriorityLevel:            defaultString(input.PriorityLevel, "normal"),
		ContentPolishedMarkdown:  input.ContentPolishedMarkdown,
		AnalysisLongformMarkdown: input.AnalysisLongformMarkdown,
		BackgroundContext:        input.BackgroundContext,
		ImpactAnalysis:           input.ImpactAnalysis,
		DebatePointsJSON:         debatePointsJSON,
		TargetAudience:           input.TargetAudience,
		PublishSuggestion:        defaultString(input.PublishSuggestion, "draft"),
		SuggestionReason:         input.SuggestionReason,
		SuggestedChannelsJSON:    suggestedChannelsJSON,
		SuggestedTagsJSON:        suggestedTagsJSON,
		SuggestedCategoriesJSON:  suggestedCategoriesJSON,
		TranslationPromptVersion: defaultPositiveInt(input.TranslationPromptVersion, 1),
		AnalysisPromptVersion:    defaultPositiveInt(input.AnalysisPromptVersion, 1),
		DossierPromptVersion:     defaultPositiveInt(input.DossierPromptVersion, 1),
		LLMProfileVersion:        defaultPositiveInt(input.LLMProfileVersion, 1),
		CreatedAt:                defaultTime(input.CreatedAt, now),
		UpdatedAt:                now,
	}, nil
}

func dossierModelToRecord(model models.ArticleDossierModel) (ArticleDossierRecord, error) {
	var keyPoints []string
	if err := json.Unmarshal(model.KeyPointsJSON, &keyPoints); err != nil {
		return ArticleDossierRecord{}, err
	}
	var debatePoints []string
	if err := json.Unmarshal(model.DebatePointsJSON, &debatePoints); err != nil {
		return ArticleDossierRecord{}, err
	}
	var suggestedChannels []string
	if err := json.Unmarshal(model.SuggestedChannelsJSON, &suggestedChannels); err != nil {
		return ArticleDossierRecord{}, err
	}
	var suggestedTags []string
	if err := json.Unmarshal(model.SuggestedTagsJSON, &suggestedTags); err != nil {
		return ArticleDossierRecord{}, err
	}
	var suggestedCategories []string
	if err := json.Unmarshal(model.SuggestedCategoriesJSON, &suggestedCategories); err != nil {
		return ArticleDossierRecord{}, err
	}

	return ArticleDossierRecord{
		ID:                       model.ID,
		ArticleID:                model.ArticleID,
		ProcessingID:             model.ProcessingID,
		DigestDate:               model.DigestDate.Format(dossierDateLayout),
		Version:                  model.Version,
		IsActive:                 model.IsActive,
		TitleTranslated:          model.TitleTranslated,
		SummaryPolished:          model.SummaryPolished,
		CoreSummary:              model.CoreSummary,
		KeyPoints:                keyPoints,
		TopicCategory:            model.TopicCategory,
		ImportanceScore:          model.ImportanceScore,
		RecommendationReason:     model.RecommendationReason,
		ReadingValue:             model.ReadingValue,
		PriorityLevel:            model.PriorityLevel,
		ContentPolishedMarkdown:  model.ContentPolishedMarkdown,
		AnalysisLongformMarkdown: model.AnalysisLongformMarkdown,
		BackgroundContext:        model.BackgroundContext,
		ImpactAnalysis:           model.ImpactAnalysis,
		DebatePoints:             debatePoints,
		TargetAudience:           model.TargetAudience,
		PublishSuggestion:        model.PublishSuggestion,
		SuggestionReason:         model.SuggestionReason,
		SuggestedChannels:        suggestedChannels,
		SuggestedTags:            suggestedTags,
		SuggestedCategories:      suggestedCategories,
		TranslationPromptVersion: model.TranslationPromptVersion,
		AnalysisPromptVersion:    model.AnalysisPromptVersion,
		DossierPromptVersion:     model.DossierPromptVersion,
		LLMProfileVersion:        model.LLMProfileVersion,
		CreatedAt:                model.CreatedAt,
		UpdatedAt:                model.UpdatedAt,
	}, nil
}
