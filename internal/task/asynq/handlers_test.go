package asynqtask_test

import (
	"context"
	"encoding/json"
	"testing"

	asynqtask "rss-platform/internal/task/asynq"
)

func TestNewProcessArticleTaskEncodesArticleID(t *testing.T) {
	task, err := asynqtask.NewProcessArticleTask("art-1")
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]string
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 1 {
		t.Fatalf("want exactly 1 payload field got %d", len(payload))
	}
	if payload["article_id"] != "art-1" {
		t.Fatalf("want article_id art-1 got %q", payload["article_id"])
	}
}

func TestArticleProcessingHandlerPassesArticleIDToCallback(t *testing.T) {
	var gotArticleID string
	handler := asynqtask.NewArticleProcessingHandler(func(_ context.Context, articleID string) error {
		gotArticleID = articleID
		return nil
	})

	task, err := asynqtask.NewProcessArticleTask("art-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := handler.ProcessTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if gotArticleID != "art-1" {
		t.Fatalf("want art-1 got %q", gotArticleID)
	}
}
