package digest_planning

import (
	"context"
	"encoding/json"
	"errors"
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
	runner Runner
}

// New 创建可替换底层实现的最小 planner agent。
func New(runner Runner) *Agent {
	return &Agent{runner: runner}
}

// Plan 根据候选文章生成结构化日报规划。
func (a *Agent) Plan(ctx context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error) {
	if a == nil || a.runner == nil {
		return domaindigest.Plan{}, errRunnerRequired
	}

	prompt, err := buildPrompt(items)
	if err != nil {
		return domaindigest.Plan{}, err
	}

	return a.runner.Run(ctx, prompt)
}

func buildPrompt(items []domaindigest.CandidateArticle) (string, error) {
	templateText, err := promptassets.Read(digestPromptFile)
	if err != nil {
		return "", err
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
	prompt.WriteString(`仅输出 JSON：{"title":"","subtitle":"","opening_note":"","sections":[{"name":"","items":[{"article_id":"","title":"","core_summary":""}]}]}`)
	prompt.WriteString("\n")
	prompt.WriteString("输入候选文章 JSON：")
	prompt.Write(payload)

	return prompt.String(), nil
}
