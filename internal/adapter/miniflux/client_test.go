package miniflux_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rss-platform/internal/adapter/miniflux"
)

func TestClientListEntriesSendsAuthHeaderAndPublishedAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != "secret-token" {
			t.Fatal("missing auth token")
		}
		if r.URL.Query().Get("published_after") == "" {
			t.Fatal("expected published_after")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"entries": []map[string]any{{
				"id":           1001,
				"title":        "New Model",
				"url":          "https://example.com/a",
				"content":      "<p>Hello</p>",
				"published_at": time.Now().UTC().Format(time.RFC3339),
			}},
		})
	}))
	defer server.Close()

	client := miniflux.NewClient(server.URL, "secret-token")
	entries, err := client.ListEntries(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 got %d", len(entries))
	}
}
