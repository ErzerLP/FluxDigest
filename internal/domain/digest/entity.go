package digest

// CandidateArticle 表示进入日报规划阶段的候选文章。
type CandidateArticle struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	CoreSummary string `json:"core_summary"`
}

// SectionItem 表示日报中被选中的稳定文章引用。
type SectionItem struct {
	ArticleID   string `json:"article_id"`
	Title       string `json:"title"`
	CoreSummary string `json:"core_summary"`
}

// Section 表示日报中的一个结构化分节。
type Section struct {
	Name  string        `json:"name"`
	Items []SectionItem `json:"items"`
}

// Plan 表示 planner 输出的日报规划结果。
type Plan struct {
	Title       string    `json:"title"`
	Subtitle    string    `json:"subtitle"`
	OpeningNote string    `json:"opening_note"`
	Sections    []Section `json:"sections"`
}
