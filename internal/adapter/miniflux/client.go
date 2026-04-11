package miniflux

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Entry struct {
	ID          int64     `json:"id"`
	FeedID      int64     `json:"feed_id"`
	FeedTitle   string    `json:"feed_title"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	URL         string    `json:"url"`
	Content     string    `json:"content"`
	PublishedAt time.Time `json:"published_at"`
}

type Client struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

func NewClient(baseURL, authToken string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		authToken:  authToken,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) ListEntries(ctx context.Context, windowStart, windowEnd time.Time) ([]Entry, error) {
	endpoint, err := url.Parse(c.baseURL + "/v1/entries")
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	query := endpoint.Query()
	query.Set("published_after", strconv.FormatInt(windowStart.UTC().Unix(), 10))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Auth-Token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request entries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("miniflux returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Entries []struct {
			ID          int64     `json:"id"`
			FeedID      int64     `json:"feed_id"`
			Title       string    `json:"title"`
			Author      string    `json:"author"`
			URL         string    `json:"url"`
			Content     string    `json:"content"`
			PublishedAt time.Time `json:"published_at"`
			Feed        struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
			} `json:"feed"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode entries response: %w", err)
	}

	entries := make([]Entry, 0, len(payload.Entries))
	for _, raw := range payload.Entries {
		if !raw.PublishedAt.IsZero() {
			if raw.PublishedAt.Before(windowStart) {
				continue
			}
			if !windowEnd.IsZero() && !raw.PublishedAt.Before(windowEnd) {
				continue
			}
		}

		feedID := raw.Feed.ID
		if feedID == 0 {
			feedID = raw.FeedID
		}
		entries = append(entries, Entry{
			ID:          raw.ID,
			FeedID:      feedID,
			FeedTitle:   raw.Feed.Title,
			Title:       raw.Title,
			Author:      raw.Author,
			URL:         raw.URL,
			Content:     raw.Content,
			PublishedAt: raw.PublishedAt,
		})
	}

	return entries, nil
}
