package digest_planning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	errPromptRunnerRequired = errors.New("digest planning prompt runner is required")
	errPlanTitleRequired    = errors.New("digest plan title is required")
	errPlanSectionsRequired = errors.New("digest plan sections are required")
)

const defaultPlanTitle = "FluxDigest 每日汇总"

const planStructuredOutputAttempts = 2

// PromptRunner 定义最小文本生成边界。
type PromptRunner interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type structuredJSONPromptRunner interface {
	GenerateStructuredJSON(ctx context.Context, prompt string) (string, error)
}

// OpenAIRunner 负责把原始 JSON 响应解析为 Plan。
type OpenAIRunner struct {
	runner PromptRunner
}

// NewOpenAIRunner 创建规划执行器。
func NewOpenAIRunner(runner PromptRunner) *OpenAIRunner {
	return &OpenAIRunner{runner: runner}
}

// Run 执行 prompt 并解析结构化日报规划。
func (r *OpenAIRunner) Run(ctx context.Context, prompt string) (Plan, error) {
	if r == nil || r.runner == nil {
		return Plan{}, errPromptRunnerRequired
	}

	var lastStructuredErr error
	for attempt := 0; attempt < planStructuredOutputAttempts; attempt++ {
		raw, err := generateStructuredPlanJSON(ctx, r.runner, prompt)
		if err != nil {
			return Plan{}, err
		}

		var plan Plan
		if err := json.Unmarshal([]byte(normalizePlanJSON(raw)), &plan); err != nil {
			lastStructuredErr = err
			continue
		}
		plan = normalizePlan(plan)
		if err := validatePlan(plan); err != nil {
			lastStructuredErr = err
			continue
		}
		return plan, nil
	}

	return Plan{}, lastStructuredErr
}

func normalizePlanJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end >= start {
		return trimmed[start : end+1]
	}

	return trimmed
}

func normalizePlan(plan Plan) Plan {
	if strings.TrimSpace(plan.Title) == "" {
		plan.Title = defaultPlanTitle
	}
	for i := range plan.Sections {
		for j := range plan.Sections[i].Items {
			if strings.TrimSpace(plan.Sections[i].Items[j].ImportanceBucket) == "" {
				plan.Sections[i].Items[j].ImportanceBucket = "normal"
			}
		}
	}

	return plan
}

func validatePlan(plan Plan) error {
	if strings.TrimSpace(plan.Title) == "" {
		return errPlanTitleRequired
	}
	if len(plan.Sections) == 0 {
		return errPlanSectionsRequired
	}

	for i, section := range plan.Sections {
		if strings.TrimSpace(section.Name) == "" {
			return fmt.Errorf("digest plan section[%d] name is required", i)
		}
		if len(section.Items) == 0 {
			return fmt.Errorf("digest plan section[%d] items are required", i)
		}
		for j, item := range section.Items {
			if strings.TrimSpace(item.ArticleID) == "" {
				return fmt.Errorf("digest plan section[%d] item[%d] article_id is required", i, j)
			}
			if strings.TrimSpace(item.Title) == "" {
				return fmt.Errorf("digest plan section[%d] item[%d] title is required", i, j)
			}
		}
	}

	return nil
}

func generateStructuredPlanJSON(ctx context.Context, runner PromptRunner, prompt string) (string, error) {
	if structured, ok := runner.(structuredJSONPromptRunner); ok {
		return structured.GenerateStructuredJSON(ctx, prompt)
	}
	return runner.Generate(ctx, prompt)
}
