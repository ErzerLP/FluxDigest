package digest_planning

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	promptassets "rss-platform/configs/prompts"
	domaindigest "rss-platform/internal/domain/digest"
)

var errRunnerRequired = errors.New("digest planning runner is required")

const digestPromptFile = "digest.tmpl"

// Runner 抽象真正执行规划提示词的底层能力，后续可替换为 ADK Agent。
type Runner interface {
	Run(ctx context.Context, prompt string) (Plan, error)
}

// Planner 定义日报规划所需的最小能力边界。
type Planner interface {
	Plan(ctx context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error)
}

// Agent 负责把候选文章交给底层 runner 生成结构化 Plan。
type Agent struct {
	runner     Runner
	promptText string
}

// New 创建可替换底层实现的最小 planner agent。
func New(runner Runner) *Agent {
	return NewWithPrompt(runner, "")
}

// NewWithPrompt 创建可注入 prompt 文本的 planner agent。
func NewWithPrompt(runner Runner, promptText string) *Agent {
	return &Agent{runner: runner, promptText: promptText}
}

// Plan 根据候选文章生成结构化日报规划。
func (a *Agent) Plan(ctx context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
	if a == nil || a.runner == nil {
		return domaindigest.Plan{}, errRunnerRequired
	}

	prompt, err := buildPrompt(a.promptText, items)
	if err != nil {
		return domaindigest.Plan{}, err
	}

	plan, err := a.runner.Run(ctx, prompt)
	if err == nil {
		return plan, nil
	}
	if len(items) == 0 {
		return domaindigest.Plan{}, err
	}

	return fallbackPlan(items), nil
}

func buildPrompt(templateText string, items []domaindigest.CandidateArticle) (string, error) {
	if strings.TrimSpace(templateText) == "" {
		var err error
		templateText, err = promptassets.Read(digestPromptFile)
		if err != nil {
			return "", err
		}
	}

	payload, err := json.Marshal(struct {
		Articles []domaindigest.CandidateArticle `json:"articles"`
	}{Articles: items})
	if err != nil {
		return "", err
	}

	var prompt strings.Builder
	prompt.WriteString(strings.TrimSpace(templateText))
	prompt.WriteString("\n")
	prompt.WriteString(`仅输出 JSON：{"title":"","subtitle":"","opening_note":"","sections":[{"name":"","items":[{"dossier_id":"","article_id":"","title":"","core_summary":"","importance_bucket":"","is_featured":false}]}]}`)
	prompt.WriteString("\n")
	prompt.WriteString("约束：article_id 与 dossier_id 必须原样复用输入候选值，不得编造、改写、拼接或猜测；若无法对应输入候选，删除该 item。")
	prompt.WriteString("\n")
	prompt.WriteString("输入候选文章 JSON：")
	prompt.Write(payload)

	return prompt.String(), nil
}

func fallbackPlan(items []domaindigest.CandidateArticle) domaindigest.Plan {
	sorted := append([]domaindigest.CandidateArticle(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].ImportanceScore == sorted[j].ImportanceScore {
			if sorted[i].PriorityLevel == sorted[j].PriorityLevel {
				return sorted[i].Title < sorted[j].Title
			}
			return sorted[i].PriorityLevel > sorted[j].PriorityLevel
		}
		return sorted[i].ImportanceScore > sorted[j].ImportanceScore
	})

	featuredCount := 3
	if len(sorted) < featuredCount {
		featuredCount = len(sorted)
	}

	featuredItems := make([]domaindigest.SectionItem, 0, featuredCount)
	for idx, item := range sorted[:featuredCount] {
		featuredItems = append(featuredItems, fallbackSectionItem(item, idx == 0))
	}

	sections := []domaindigest.Section{{
		Name:  "重点关注",
		Items: featuredItems,
	}}

	if len(sorted) > featuredCount {
		moreItems := make([]domaindigest.SectionItem, 0, len(sorted)-featuredCount)
		for _, item := range sorted[featuredCount:] {
			moreItems = append(moreItems, fallbackSectionItem(item, false))
		}
		sections = append(sections, domaindigest.Section{
			Name:  "延伸阅读",
			Items: moreItems,
		})
	}

	return domaindigest.Plan{
		Title:       "FluxDigest 每日汇总",
		Subtitle:    "日报规划回退为稳定自动编排输出",
		OpeningNote: "本期因规划模型输出不稳定，已根据单篇处理结果自动整理重点文章。",
		Sections:    sections,
	}
}

func fallbackSectionItem(item domaindigest.CandidateArticle, featured bool) domaindigest.SectionItem {
	bucket := "normal"
	if featured {
		bucket = "featured"
	}

	return domaindigest.SectionItem{
		DossierID:        item.DossierID,
		ArticleID:        item.ID,
		Title:            item.Title,
		CoreSummary:      item.CoreSummary,
		ImportanceBucket: bucket,
		IsFeatured:       featured,
	}
}
