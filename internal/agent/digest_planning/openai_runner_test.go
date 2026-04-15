package digest_planning_test

import (
	"context"
	"errors"
	"testing"

	"rss-platform/internal/agent/digest_planning"
)

type promptRunnerStub struct {
	response  string
	responses []string
	errors    []error
	prompts   []string
}

func (s *promptRunnerStub) Generate(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if len(s.errors) > 0 {
		err := s.errors[0]
		s.errors = s.errors[1:]
		if err != nil {
			return "", err
		}
	}
	if len(s.responses) > 0 {
		resp := s.responses[0]
		s.responses = s.responses[1:]
		return resp, nil
	}
	return s.response, nil
}

func TestOpenAIRunnerRunParsesPlanJSON(t *testing.T) {
	runner := digest_planning.NewOpenAIRunner(&promptRunnerStub{
		response: `{"title":"今日 AI 日报","subtitle":"聚焦模型动态","opening_note":"以下为重点","sections":[{"name":"重点速览","items":[{"article_id":"art-1","title":"Model News","core_summary":"核心总结"}]}]}`,
	})

	plan, err := runner.Run(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Title != "今日 AI 日报" {
		t.Fatalf("want 今日 AI 日报 got %s", plan.Title)
	}
	if got := plan.Sections[0].Items[0].ArticleID; got != "art-1" {
		t.Fatalf("want art-1 got %s", got)
	}
}

func TestOpenAIRunnerRunFillsDefaultTitleWhenMissing(t *testing.T) {
	runner := digest_planning.NewOpenAIRunner(&promptRunnerStub{
		response: `{"sections":[{"name":"重点速览","items":[{"article_id":"art-1","title":"Model News","core_summary":"核心总结"}]}]}`,
	})

	plan, err := runner.Run(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Title != "FluxDigest 每日汇总" {
		t.Fatalf("want default title got %s", plan.Title)
	}
}

func TestOpenAIRunnerRunRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{
			name:     "missing sections",
			response: `{"title":"今日 AI 日报","sections":[]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := digest_planning.NewOpenAIRunner(&promptRunnerStub{response: tc.response})

			_, err := runner.Run(context.Background(), "prompt")
			if err == nil {
				t.Fatal("want validation error")
			}
		})
	}
}

func TestOpenAIRunnerRunRetriesOnInvalidStructuredOutput(t *testing.T) {
	stub := &promptRunnerStub{
		responses: []string{
			`{"title":"今日 AI 日报","sections":[{"name":"重点速览","items":[]}]}`,
			`{"title":"今日 AI 日报","sections":[{"name":"重点速览","items":[{"article_id":"art-1","title":"Model News","core_summary":"核心总结"}]}]}`,
		},
	}
	runner := digest_planning.NewOpenAIRunner(stub)

	plan, err := runner.Run(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.prompts) != 2 {
		t.Fatalf("want 2 generate attempts got %d", len(stub.prompts))
	}
	if got := plan.Sections[0].Items[0].ArticleID; got != "art-1" {
		t.Fatalf("want art-1 got %s", got)
	}
}

func TestOpenAIRunnerRunReturnsErrorWhenStructuredOutputAlwaysInvalid(t *testing.T) {
	stub := &promptRunnerStub{
		responses: []string{
			`{"title":"今日 AI 日报","sections":[{"name":"重点速览","items":[]}]}`,
			`{"title":"今日 AI 日报","sections":[{"name":"重点速览","items":[]}]}`,
		},
	}
	runner := digest_planning.NewOpenAIRunner(stub)

	_, err := runner.Run(context.Background(), "prompt")
	if err == nil {
		t.Fatal("want validation error")
	}
	if len(stub.prompts) != 2 {
		t.Fatalf("want 2 generate attempts got %d", len(stub.prompts))
	}
}

func TestOpenAIRunnerRunDoesNotRetryWhenGenerateFails(t *testing.T) {
	stub := &promptRunnerStub{
		errors: []error{errors.New("dial tcp timeout")},
		responses: []string{
			`{"title":"今日 AI 日报","sections":[{"name":"重点速览","items":[{"article_id":"art-1","title":"Model News","core_summary":"核心总结"}]}]}`,
		},
	}
	runner := digest_planning.NewOpenAIRunner(stub)

	_, err := runner.Run(context.Background(), "prompt")
	if err == nil {
		t.Fatal("want generate error")
	}
	if len(stub.prompts) != 1 {
		t.Fatalf("want 1 generate attempt got %d", len(stub.prompts))
	}
}
