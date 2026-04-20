package halo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adapterpublisher "rss-platform/internal/adapter/publisher"
)

func TestPublishDigestCreatesAndPublishesPostToHalo(t *testing.T) {
	t.Helper()

	var draftBody postRequest
	var publishPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/apis/api.console.halo.run/v1alpha1/posts":
			if got := r.Header.Get("Authorization"); got != "Bearer pat-token" {
				t.Fatalf("want bearer token got %s", got)
			}
			if got := r.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
				t.Fatalf("want json content-type got %s", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&draftBody); err != nil {
				t.Fatalf("decode draft request: %v", err)
			}

			if draftBody.Post.APIVersion != postAPIVersion {
				t.Fatalf("want apiVersion %q got %q", postAPIVersion, draftBody.Post.APIVersion)
			}
			if draftBody.Post.Kind != postKind {
				t.Fatalf("want kind %q got %q", postKind, draftBody.Post.Kind)
			}
			if draftBody.Post.Spec.Title != "Daily Digest" {
				t.Fatalf("want title in payload got %q", draftBody.Post.Spec.Title)
			}
			if draftBody.Post.Spec.Excerpt.Raw != "AI 日报副标题" {
				t.Fatalf("want subtitle mapped to excerpt.raw got %q", draftBody.Post.Spec.Excerpt.Raw)
			}
			if draftBody.Content.Raw != "# Digest" {
				t.Fatalf("want markdown raw got %q", draftBody.Content.Raw)
			}
			if draftBody.Content.Content != "<h1>Digest</h1>" {
				t.Fatalf("want html content got %q", draftBody.Content.Content)
			}
			if draftBody.Content.RawType != postRawTypeMD {
				t.Fatalf("want rawType markdown got %q", draftBody.Content.RawType)
			}
			if !strings.HasPrefix(draftBody.Post.Metadata.Name, "fluxdigest-") {
				t.Fatalf("want generated post name prefix got %q", draftBody.Post.Metadata.Name)
			}
			if draftBody.Post.Spec.Slug != "daily-digest-20240413-030000" {
				t.Fatalf("want stable slug got %q", draftBody.Post.Spec.Slug)
			}

			_ = json.NewEncoder(w).Encode(postEnvelope{
				Metadata: metadata{Name: draftBody.Post.Metadata.Name},
			})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/apis/api.console.halo.run/v1alpha1/posts/"):
			publishPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(postEnvelope{
				Metadata: metadata{Name: draftBody.Post.Metadata.Name},
				Status:   postStatus{Permalink: "https://blog.example.com/archives/daily-digest"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	p := New(server.URL+"/", "pat-token")
	p.now = func() time.Time {
		return time.Date(2024, 4, 13, 3, 0, 0, 0, time.UTC)
	}

	result, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{
		Title:           "Daily Digest",
		Subtitle:        "AI 日报副标题",
		ContentMarkdown: "# Digest",
		ContentHTML:     "<h1>Digest</h1>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RemoteID != draftBody.Post.Metadata.Name {
		t.Fatalf("want remote id %q got %q", draftBody.Post.Metadata.Name, result.RemoteID)
	}
	if result.RemoteURL != "https://blog.example.com/archives/daily-digest" {
		t.Fatalf("want remote url got %q", result.RemoteURL)
	}
	wantPublishPath := "/apis/api.console.halo.run/v1alpha1/posts/" + draftBody.Post.Metadata.Name + "/publish"
	if publishPath != wantPublishPath {
		t.Fatalf("want publish path %q got %q", wantPublishPath, publishPath)
	}
}

func TestPublishDigestReturnsRetryableErrorWhenDraftPostFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	p := New(server.URL, "pat-token")
	_, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{Title: "Daily Digest", ContentMarkdown: "# Digest"})
	if !adapterpublisher.IsRetryablePublishError(err) {
		t.Fatalf("want retryable publish error got %v", err)
	}
}

func TestPublishDigestReturnsAmbiguousErrorWhenPublishFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(postEnvelope{
				Metadata: metadata{Name: "daily-digest-1"},
			})
			return
		}

		http.Error(w, "publish failed", http.StatusBadGateway)
	}))
	defer server.Close()

	p := New(server.URL, "pat-token")
	_, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{Title: "Daily Digest", ContentMarkdown: "# Digest"})
	if !adapterpublisher.IsAmbiguousPublishError(err) {
		t.Fatalf("want ambiguous publish error got %v", err)
	}
}

func TestPublishDigestReturnsAmbiguousErrorOnInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"metadata":{"name":"daily-digest-1"`))
			return
		}

		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	p := New(server.URL, "pat-token")
	_, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{Title: "Daily Digest", ContentMarkdown: "# Digest"})
	if !adapterpublisher.IsAmbiguousPublishError(err) {
		t.Fatalf("want ambiguous publish error got %v", err)
	}
}

func TestPublishDigestUsesBasicAuthorizationWhenTokenHasBasicPrefix(t *testing.T) {
	t.Helper()

	wantCredential := base64.StdEncoding.EncodeToString([]byte("admin:halo-secret"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Basic "+wantCredential {
			t.Fatalf("want basic auth header got %q", got)
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/apis/api.console.halo.run/v1alpha1/posts":
			_ = json.NewEncoder(w).Encode(postEnvelope{
				Metadata: metadata{Name: "daily-digest-1"},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/apis/api.console.halo.run/v1alpha1/posts/daily-digest-1/publish":
			_ = json.NewEncoder(w).Encode(postEnvelope{
				Metadata: metadata{Name: "daily-digest-1"},
				Status:   postStatus{Permalink: "https://blog.example.com/basic-auth"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	p := New(server.URL, "basic:"+wantCredential)
	result, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{
		Title:           "Daily Digest",
		ContentMarkdown: "# Digest",
	})
	if err != nil {
		t.Fatalf("PublishDigest() error = %v", err)
	}
	if result.RemoteURL != "https://blog.example.com/basic-auth" {
		t.Fatalf("want basic auth publish url got %q", result.RemoteURL)
	}
}

func TestBuildPostIdentityUsesASCIIOnlySlugAndSeparateName(t *testing.T) {
	at := time.Date(2026, 4, 13, 7, 0, 0, 0, time.UTC)
	name, slug := buildPostIdentity("今日 AI 日报", at)

	if name != fmt.Sprintf("fluxdigest-%d", at.UnixNano()) {
		t.Fatalf("unexpected name %q", name)
	}
	if slug != "ai-20260413-070000" {
		t.Fatalf("unexpected slug %q", slug)
	}
	if strings.ContainsRune(name, '今') || strings.ContainsRune(slug, '今') {
		t.Fatalf("want ascii-only identity got name=%q slug=%q", name, slug)
	}
}
