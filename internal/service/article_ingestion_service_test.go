package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rss-platform/internal/adapter/miniflux"
	"rss-platform/internal/domain/article"
	"rss-platform/internal/service"
)

type articleWriterStub struct {
	upserts []article.SourceArticle
}

func (s *articleWriterStub) Upsert(_ context.Context, input article.SourceArticle) error {
	s.upserts = append(s.upserts, input)
	return nil
}

func TestArticleIngestionServiceFetchAndPersistMapsFeedTitleAndCallsUpsert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"entries": []map[string]any{{
				"id":      1001,
				"title":   "New Model",
				"author":  "Alice",
				"url":     "https://example.com/a",
				"content": "<p>Hello</p>",
				"feed": map[string]any{
					"id":    901,
					"title": "AI Weekly",
				},
			}},
		})
	}))
	defer server.Close()

	client := miniflux.NewClient(server.URL, "secret-token")
	repo := &articleWriterStub{}
	svc := service.NewArticleIngestionService(client, repo)

	if err := svc.FetchAndPersist(context.Background(), time.Unix(1712803200, 0)); err != nil {
		t.Fatal(err)
	}

	if len(repo.upserts) != 1 {
		t.Fatalf("want 1 upsert got %d", len(repo.upserts))
	}
	got := repo.upserts[0]
	if got.FeedTitle != "AI Weekly" {
		t.Fatalf("want feed title AI Weekly got %q", got.FeedTitle)
	}

	wantHash := sha256.Sum256([]byte("https://example.com/a|New Model|<p>Hello</p>"))
	wantFingerprint := hex.EncodeToString(wantHash[:])
	if got.Fingerprint != wantFingerprint {
		t.Fatalf("fingerprint mismatch: want %s got %s", wantFingerprint, got.Fingerprint)
	}
}
