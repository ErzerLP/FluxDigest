package publisher

import (
	"context"
	"errors"
)

// PublishDigestRequest 表示日报发布请求。
type PublishDigestRequest struct {
	Title           string
	Subtitle        string
	ContentMarkdown string
	ContentHTML     string
	Tags            []string
}

// PublishDigestResult 表示日报发布结果。
type PublishDigestResult struct {
	RemoteID  string
	RemoteURL string
}

// PublishErrorKind 表示发布失败的副作用确定性分类。
type PublishErrorKind string

const (
	PublishErrorKindRetryable PublishErrorKind = "retryable"
	PublishErrorKindAmbiguous PublishErrorKind = "ambiguous"
)

// PublishError 为发布失败附带副作用分类。
type PublishError struct {
	Kind PublishErrorKind
	Err  error
}

func (e *PublishError) Error() string {
	if e == nil || e.Err == nil {
		return "publish failed"
	}
	return e.Err.Error()
}

func (e *PublishError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewRetryablePublishError(err error) error {
	if err == nil {
		return nil
	}
	return &PublishError{Kind: PublishErrorKindRetryable, Err: err}
}

func NewAmbiguousPublishError(err error) error {
	if err == nil {
		return nil
	}
	return &PublishError{Kind: PublishErrorKindAmbiguous, Err: err}
}

func IsRetryablePublishError(err error) bool {
	var target *PublishError
	return errors.As(err, &target) && target.Kind == PublishErrorKindRetryable
}

func IsAmbiguousPublishError(err error) bool {
	var target *PublishError
	return errors.As(err, &target) && target.Kind == PublishErrorKindAmbiguous
}

// Publisher 定义日报发布器最小能力。
type Publisher interface {
	Name() string
	PublishDigest(ctx context.Context, req PublishDigestRequest) (PublishDigestResult, error)
}
