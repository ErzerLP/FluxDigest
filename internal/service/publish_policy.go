package service

import "strings"

const (
	articlePublishModeDigestOnly = "digest_only"
	articlePublishModeSuggested  = "suggested"
	articlePublishModeAll        = "all"

	articleReviewModeManualReview = "manual_review"
	articleReviewModeAutoPublish  = "auto_publish"

	publishStatePendingReview = "pending_review"
	publishStateQueued        = "queued"
)

func normalizeArticlePublishMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case articlePublishModeAll:
		return articlePublishModeAll
	case articlePublishModeSuggested, "partial", "partial_review":
		return articlePublishModeSuggested
	default:
		return articlePublishModeDigestOnly
	}
}

func normalizeArticleReviewMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case articleReviewModeAutoPublish, "auto":
		return articleReviewModeAutoPublish
	default:
		return articleReviewModeManualReview
	}
}

func initialArticlePublishState(mode, review, suggestion string) string {
	normalizedMode := normalizeArticlePublishMode(mode)
	normalizedReview := normalizeArticleReviewMode(review)

	if normalizedMode == articlePublishModeDigestOnly {
		return defaultPublishState
	}

	if normalizedMode == articlePublishModeSuggested && suggestion != publishSuggestionSuggested {
		return defaultPublishState
	}

	if normalizedReview == articleReviewModeAutoPublish {
		return publishStateQueued
	}

	return publishStatePendingReview
}
