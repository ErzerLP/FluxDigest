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

func TestNewDailyDigestTaskEncodesDigestDate(t *testing.T) {
	task, err := asynqtask.NewDailyDigestTask("2026-04-10")
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]string
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["digest_date"] != "2026-04-10" {
		t.Fatalf("want digest_date 2026-04-10 got %q", payload["digest_date"])
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

func TestDailyDigestHandlerPassesDigestDateToCallback(t *testing.T) {
	var gotDigestDate string
	handler := asynqtask.NewDailyDigestHandler(func(_ context.Context, digestDate string) error {
		gotDigestDate = digestDate
		return nil
	})

	task, err := asynqtask.NewDailyDigestTask("2026-04-10")
	if err != nil {
		t.Fatal(err)
	}

	if err := handler.ProcessTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if gotDigestDate != "2026-04-10" {
		t.Fatalf("want 2026-04-10 got %q", gotDigestDate)
	}
}
