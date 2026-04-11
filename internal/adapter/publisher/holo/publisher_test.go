package holo_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	adapterpublisher "rss-platform/internal/adapter/publisher"
	"rss-platform/internal/adapter/publisher/holo"
)

func TestPublishDigestPostsMarkdownToHolo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("want POST got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer blog-token" {
			t.Fatalf("want bearer token got %s", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["title"] != "今日 AI 日报" {
			t.Fatalf("want title in payload got %#v", body["title"])
		}
		if body["content_markdown"] != "# 内容" {
			t.Fatalf("want content markdown in payload got %#v", body["content_markdown"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"id": "remote-1", "url": "https://blog.example.com/digest"})
	}))
	defer server.Close()

	p := holo.New(server.URL, "blog-token")
	result, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{Title: "今日 AI 日报", ContentMarkdown: "# 内容"})
	if err != nil {
		t.Fatal(err)
	}
	if result.RemoteURL == "" {
		t.Fatal("expected remote url")
	}
}

func TestPublishDigestReturnsRetryableErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	p := holo.New(server.URL, "blog-token")
	_, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{Title: "今日 AI 日报", ContentMarkdown: "# 内容"})
	if !adapterpublisher.IsRetryablePublishError(err) {
		t.Fatalf("want retryable publish error got %v", err)
	}
}

func TestPublishDigestReturnsAmbiguousErrorOnDecodeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	p := holo.New(server.URL, "blog-token")
	_, err := p.PublishDigest(context.Background(), adapterpublisher.PublishDigestRequest{Title: "今日 AI 日报", ContentMarkdown: "# 内容"})
	if !adapterpublisher.IsAmbiguousPublishError(err) {
		t.Fatalf("want ambiguous publish error got %v", err)
	}
}
