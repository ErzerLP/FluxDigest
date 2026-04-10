package miniflux

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (c *Client) ListEntries(ctx context.Context, since time.Time) ([]Entry, error) {
	endpoint, err := url.Parse(c.baseURL + "/v1/entries")
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	query := endpoint.Query()
	query.Set("published_after", since.UTC().Format(time.RFC3339))
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
		Entries []Entry `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode entries response: %w", err)
	}

	return payload.Entries, nil
}
