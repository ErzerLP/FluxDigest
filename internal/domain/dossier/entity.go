package dossier

import "time"

// ArticleDossier 表示一篇文章在指定日报日的可发布内容资产。
type ArticleDossier struct {
	ID                       string
	ArticleID                string
	ProcessingID             string
	DigestDate               time.Time
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

// ArticlePublishState 表示 dossier 的发布决策与外部发布状态。
type ArticlePublishState struct {
	ID             string
	DossierID      string
	State          string
	ApprovedBy     string
	DecisionNote   string
	PublishChannel string
	RemoteID       string
	RemoteURL      string
	ErrorMessage   string
	PublishedAt    *time.Time
	UpdatedAt      time.Time
}
