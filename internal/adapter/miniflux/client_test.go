package miniflux_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"rss-platform/internal/adapter/miniflux"
)

func TestClientListEntriesSendsAuthHeaderAndPublishedAfterUnixAndParsesNestedFeedTitle(t *testing.T) {
	since := time.Unix(1712803200, 0).UTC()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != "secret-token" {
			t.Fatal("missing auth token")
		}

		gotPublishedAfter := r.URL.Query().Get("published_after")
		wantPublishedAfter := strconv.FormatInt(since.Unix(), 10)
		if gotPublishedAfter != wantPublishedAfter {
			t.Fatalf("published_after mismatch: want %s got %s", wantPublishedAfter, gotPublishedAfter)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"entries": []map[string]any{{
				"id":           1001,
				"title":        "New Model",
				"url":          "https://example.com/a",
				"content":      "<p>Hello</p>",
				"published_at": time.Now().UTC().Format(time.RFC3339),
				"feed": map[string]any{
					"id":    321,
					"title": "AI Weekly",
				},
			}},
		})
	}))
	defer server.Close()

	client := miniflux.NewClient(server.URL, "secret-token")
	entries, err := client.ListEntries(context.Background(), since)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 got %d", len(entries))
	}
	if entries[0].FeedTitle != "AI Weekly" {
		t.Fatalf("want feed title AI Weekly got %q", entries[0].FeedTitle)
	}
	if entries[0].FeedID != 321 {
		t.Fatalf("want feed id 321 got %d", entries[0].FeedID)
	}
}
