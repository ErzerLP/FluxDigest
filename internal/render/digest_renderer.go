package render

import (
	"html/template"
	"strings"

	domaindigest "rss-platform/internal/domain/digest"
)

// DigestRenderer 负责把结构化日报规划渲染为 Markdown 与 HTML。
type DigestRenderer struct{}

// NewDigestRenderer 创建最小日报渲染器。
func NewDigestRenderer() *DigestRenderer {
	return &DigestRenderer{}
}

// Render 只根据 Plan 渲染 Markdown 与 HTML。
func (r *DigestRenderer) Render(plan domaindigest.Plan) (string, string, error) {
	return renderMarkdown(plan), renderHTML(plan), nil
}

func renderMarkdown(plan domaindigest.Plan) string {
	var builder strings.Builder
	builder.WriteString("# ")
	builder.WriteString(plan.Title)
	builder.WriteString("\n\n")

	if plan.Subtitle != "" {
		builder.WriteString(plan.Subtitle)
		builder.WriteString("\n\n")
	}
	if plan.OpeningNote != "" {
		builder.WriteString(plan.OpeningNote)
		builder.WriteString("\n\n")
	}

	for _, section := range plan.Sections {
		builder.WriteString("## ")
		builder.WriteString(section.Name)
		builder.WriteString("\n")
		for _, item := range section.Items {
			builder.WriteString("- **")
			builder.WriteString(item.Title)
			builder.WriteString("**")
			if item.CoreSummary != "" {
				builder.WriteString("：")
				builder.WriteString(item.CoreSummary)
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

func renderHTML(plan domaindigest.Plan) string {
	var builder strings.Builder
	builder.WriteString("<article>")
	builder.WriteString("<h1>")
	builder.WriteString(template.HTMLEscapeString(plan.Title))
	builder.WriteString("</h1>")

	if plan.Subtitle != "" {
		builder.WriteString("<p>")
		builder.WriteString(template.HTMLEscapeString(plan.Subtitle))
		builder.WriteString("</p>")
	}
	if plan.OpeningNote != "" {
		builder.WriteString("<p>")
		builder.WriteString(template.HTMLEscapeString(plan.OpeningNote))
		builder.WriteString("</p>")
	}

	for _, section := range plan.Sections {
		builder.WriteString("<section><h2>")
		builder.WriteString(template.HTMLEscapeString(section.Name))
		builder.WriteString("</h2><ul>")
		for _, item := range section.Items {
			builder.WriteString("<li><strong>")
			builder.WriteString(template.HTMLEscapeString(item.Title))
			builder.WriteString("</strong>")
			if item.CoreSummary != "" {
				builder.WriteString("：")
				builder.WriteString(template.HTMLEscapeString(item.CoreSummary))
			}
			builder.WriteString("</li>")
		}
		builder.WriteString("</ul></section>")
	}

	builder.WriteString("</article>")
	return builder.String()
}
