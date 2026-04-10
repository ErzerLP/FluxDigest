package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"rss-platform/internal/adapter/miniflux"
	"rss-platform/internal/domain/article"
)

type ArticleWriter interface {
	Upsert(ctx context.Context, input article.SourceArticle) error
}

type ArticleIngestionService struct {
	client *miniflux.Client
	repo   ArticleWriter
}

func NewArticleIngestionService(client *miniflux.Client, repo ArticleWriter) *ArticleIngestionService {
	return &ArticleIngestionService{client: client, repo: repo}
}

func (s *ArticleIngestionService) FetchAndPersist(ctx context.Context, since time.Time) error {
	entries, err := s.client.ListEntries(ctx, since)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		source := article.SourceArticle{
			MinifluxEntryID: entry.ID,
			FeedID:          entry.FeedID,
			FeedTitle:       entry.FeedTitle,
			Title:           entry.Title,
			Author:          entry.Author,
			URL:             entry.URL,
			ContentHTML:     entry.Content,
			ContentText:     htmlToText(entry.Content),
			Fingerprint:     fingerprint(entry),
		}
		if err := s.repo.Upsert(ctx, source); err != nil {
			return err
		}
	}

	return nil
}

var htmlTagRegex = regexp.MustCompile(`<[^>]+>`)

func htmlToText(content string) string {
	return strings.TrimSpace(htmlTagRegex.ReplaceAllString(content, " "))
}

func fingerprint(entry miniflux.Entry) string {
	hash := sha256.Sum256([]byte(entry.URL + "|" + entry.Title + "|" + entry.Content))
	return hex.EncodeToString(hash[:])
}
