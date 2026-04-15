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
	task, err := asynqtask.NewDailyDigestTask(asynqtask.DailyDigestPayload{
		DigestDate: "2026-04-10",
		Force:      true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		DigestDate string `json:"digest_date"`
		Force      bool   `json:"force"`
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.DigestDate != "2026-04-10" {
		t.Fatalf("want digest_date 2026-04-10 got %q", payload.DigestDate)
	}
	if !payload.Force {
		t.Fatal("want force=true in payload")
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
	var gotPayload asynqtask.DailyDigestPayload
	handler := asynqtask.NewDailyDigestHandler(func(_ context.Context, payload asynqtask.DailyDigestPayload) error {
		gotPayload = payload
		return nil
	})

	task, err := asynqtask.NewDailyDigestTask(asynqtask.DailyDigestPayload{DigestDate: "2026-04-10", Force: true})
	if err != nil {
		t.Fatal(err)
	}

	if err := handler.ProcessTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if gotPayload.DigestDate != "2026-04-10" {
		t.Fatalf("want 2026-04-10 got %q", gotPayload.DigestDate)
	}
	if !gotPayload.Force {
		t.Fatal("want force=true callback payload")
	}
}

func TestDailyDigestHandlerRunsRuntimeService(t *testing.T) {
	called := 0
	handler := asynqtask.NewDailyDigestHandler(func(_ context.Context, payload asynqtask.DailyDigestPayload) error {
		called++
		if payload.DigestDate != "2026-04-11" {
			t.Fatalf("unexpected digest date %s", payload.DigestDate)
		}
		if !payload.Force {
			t.Fatal("expected force=true")
		}
		return nil
	})

	task, err := asynqtask.NewDailyDigestTask(asynqtask.DailyDigestPayload{DigestDate: "2026-04-11", Force: true})
	if err != nil {
		t.Fatal(err)
	}

	if err := handler.ProcessTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("want 1 call got %d", called)
	}
}

func TestNewReprocessArticleTaskEncodesPayload(t *testing.T) {
	task, err := asynqtask.NewReprocessArticleTask(asynqtask.ReprocessArticlePayload{
		ArticleID: "art-1",
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		ArticleID string `json:"article_id"`
		Force     bool   `json:"force"`
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ArticleID != "art-1" {
		t.Fatalf("want article_id art-1 got %s", payload.ArticleID)
	}
	if !payload.Force {
		t.Fatal("want force=true in payload")
	}
}

func TestArticleReprocessHandlerPassesPayloadToCallback(t *testing.T) {
	var gotPayload asynqtask.ReprocessArticlePayload
	handler := asynqtask.NewArticleReprocessHandler(func(_ context.Context, payload asynqtask.ReprocessArticlePayload) error {
		gotPayload = payload
		return nil
	})

	task, err := asynqtask.NewReprocessArticleTask(asynqtask.ReprocessArticlePayload{ArticleID: "art-1", Force: true})
	if err != nil {
		t.Fatal(err)
	}

	if err := handler.ProcessTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if gotPayload.ArticleID != "art-1" {
		t.Fatalf("want article id art-1 got %s", gotPayload.ArticleID)
	}
	if !gotPayload.Force {
		t.Fatal("want force=true callback payload")
	}
}
