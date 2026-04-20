package worker

import (
	"context"
	"testing"

	asynqtask "rss-platform/internal/task/asynq"
)

func TestNewServeMuxRegistersArticleReprocessHandler(t *testing.T) {
	called := 0
	reprocessHandler := asynqtask.NewArticleReprocessHandler(func(_ context.Context, payload asynqtask.ReprocessArticlePayload) error {
		called++
		if payload.ArticleID != "art-1" {
			t.Fatalf("want article id art-1 got %s", payload.ArticleID)
		}
		if !payload.Force {
			t.Fatal("want force=true")
		}
		return nil
	})

	mux := NewServeMux(nil, nil, reprocessHandler)
	task, err := asynqtask.NewReprocessArticleTask(asynqtask.ReprocessArticlePayload{
		ArticleID: "art-1",
		Force:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := mux.ProcessTask(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("want 1 call got %d", called)
	}
}
