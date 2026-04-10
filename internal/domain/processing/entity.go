package processing

import "rss-platform/internal/domain/article"

// Translation 表示翻译结果。
type Translation struct {
	TitleTranslated   string
	SummaryTranslated string
	ContentTranslated string
}

// Analysis 表示文章分析结果。
type Analysis struct {
	CoreSummary     string
	KeyPoints       []string
	TopicCategory   string
	ImportanceScore float64
}

// ProcessedArticle 聚合单篇文章处理结果。
type ProcessedArticle struct {
	Article     article.SourceArticle
	Translation Translation
	Analysis    Analysis
}
