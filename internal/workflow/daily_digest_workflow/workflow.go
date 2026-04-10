package daily_digest_workflow

import (
	"context"
	"errors"

	domaindigest "rss-platform/internal/domain/digest"
)

var (
	errPlannerRequired  = errors.New("digest planner is required")
	errRendererRequired = errors.New("digest renderer is required")
)

// Planner 定义日报规划所需的最小能力。
type Planner interface {
	Plan(ctx context.Context, items []domaindigest.CandidateArticle) (domaindigest.Plan, error)
}

// Renderer 定义日报渲染所需的最小能力。
type Renderer interface {
	Render(plan domaindigest.Plan) (string, string, error)
}

// Digest 表示日报工作流输出。
type Digest struct {
	Title           string
	Subtitle        string
	ContentMarkdown string
	ContentHTML     string
	Plan            domaindigest.Plan
}

// Workflow 负责编排日报规划与渲染。
type Workflow struct {
	planner  Planner
	renderer Renderer
}

// New 创建日报工作流。
func New(planner Planner, renderer Renderer) *Workflow {
	return &Workflow{planner: planner, renderer: renderer}
}

// Run 顺序执行 planner 与 renderer，输出日报内容。
func (w *Workflow) Run(ctx context.Context, items []domaindigest.CandidateArticle) (Digest, error) {
	if w == nil || w.planner == nil {
		return Digest{}, errPlannerRequired
	}
	if w.renderer == nil {
		return Digest{}, errRendererRequired
	}

	plan, err := w.planner.Plan(ctx, items)
	if err != nil {
		return Digest{}, err
	}

	markdown, html, err := w.renderer.Render(plan)
	if err != nil {
		return Digest{}, err
	}

	return Digest{
		Title:           plan.Title,
		Subtitle:        plan.Subtitle,
		ContentMarkdown: markdown,
		ContentHTML:     html,
		Plan:            plan,
	}, nil
}
