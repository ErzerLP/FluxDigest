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
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/service"
)

type profileRepoStub struct {
	created []profile.Version
	active  map[string]profile.Version
}

func (s *profileRepoStub) Create(_ context.Context, v profile.Version) error {
	s.created = append(s.created, v)
	if s.active == nil {
		s.active = make(map[string]profile.Version)
	}
	if v.IsActive {
		s.active[v.ProfileType] = v
	}
	return nil
}

func (s *profileRepoStub) GetActive(_ context.Context, profileType string) (profile.Version, error) {
	if s.active != nil {
		if v, ok := s.active[profileType]; ok {
			return v, nil
		}
	}
	return profile.Version{}, service.ErrProfileNotFound
}

func TestProfileServiceSeedsDefaults(t *testing.T) {
	repo := &profileRepoStub{}
	svc := service.NewProfileService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.created) != 4 {
		t.Fatalf("want 4 got %d", len(repo.created))
	}

	var aiPayload map[string]any
	if err := json.Unmarshal(repo.created[0].PayloadJSON, &aiPayload); err != nil {
		t.Fatalf("unmarshal ai payload: %v", err)
	}
	if aiPayload["translation_prompt_template"] != "configs/prompts/translation.tmpl" {
		t.Fatalf("missing translation prompt template path in ai payload: %+v", aiPayload)
	}
	if aiPayload["analysis_prompt_template"] != "configs/prompts/analysis.tmpl" {
		t.Fatalf("missing analysis prompt template path in ai payload: %+v", aiPayload)
	}
}

func TestProfileServiceSeedDefaultsIsIdempotentWhenActiveExists(t *testing.T) {
	repo := &profileRepoStub{}
	svc := service.NewProfileService(repo)

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	if len(repo.created) != 4 {
		t.Fatalf("want 4 created records after two seed calls, got %d", len(repo.created))
	}
}

type articleWriterStub struct {
	upserts []article.SourceArticle
}

func (s *articleWriterStub) Upsert(_ context.Context, input article.SourceArticle) error {
	s.upserts = append(s.upserts, input)
	return nil
}

func TestArticleIngestionServiceFetchAndPersistMapsFeedTitleAndFingerprint(t *testing.T) {
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
	if got.FeedID != 901 {
		t.Fatalf("want feed id 901 got %d", got.FeedID)
	}
	if got.Fingerprint == "" {
		t.Fatal("want non-empty fingerprint")
	}

	wantHash := sha256.Sum256([]byte("https://example.com/a|New Model|<p>Hello</p>"))
	wantFingerprint := hex.EncodeToString(wantHash[:])
	if got.Fingerprint != wantFingerprint {
		t.Fatalf("fingerprint mismatch: want %s got %s", wantFingerprint, got.Fingerprint)
	}
}
