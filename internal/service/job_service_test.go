package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type enqueueStub struct {
	dates []string
	errs  []error
}

func (s *enqueueStub) EnqueueDailyDigest(_ context.Context, digestDate string) error {
	s.dates = append(s.dates, digestDate)
	if len(s.errs) == 0 {
		return nil
	}

	err := s.errs[0]
	s.errs = s.errs[1:]
	if err != nil {
		return err
	}

	return nil
}

func TestJobServiceSkipsDuplicateDigestDate(t *testing.T) {
	queue := &enqueueStub{errs: []error{ErrDailyDigestAlreadyQueued}}
	svc := NewJobService(queue)
	now := time.Date(2026, 4, 10, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))

	result, err := svc.TriggerDailyDigest(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}

	if len(queue.dates) != 1 {
		t.Fatalf("want queue called once got %d", len(queue.dates))
	}
	if queue.dates[0] != "2026-04-10" {
		t.Fatalf("want digest date 2026-04-10 got %s", queue.dates[0])
	}
	if result.Status != "skipped" {
		t.Fatalf("want skipped status got %s", result.Status)
	}
}

func TestJobServiceNormalizesDigestDateToShanghai(t *testing.T) {
	queue := &enqueueStub{}
	svc := NewJobService(queue)
	now := time.Date(2026, 4, 9, 16, 30, 0, 0, time.UTC)

	result, err := svc.TriggerDailyDigest(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}

	if len(queue.dates) != 1 {
		t.Fatalf("want queue called once got %d", len(queue.dates))
	}
	if queue.dates[0] != "2026-04-10" {
		t.Fatalf("want digest date 2026-04-10 got %s", queue.dates[0])
	}
	if result.DigestDate != "2026-04-10" {
		t.Fatalf("want digest date 2026-04-10 got %s", result.DigestDate)
	}
}

func TestJobServiceRetriesSameDateAfterEnqueueFailure(t *testing.T) {
	queue := &enqueueStub{errs: []error{errors.New("redis down"), nil}}
	svc := NewJobService(queue)
	now := time.Date(2026, 4, 10, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))

	if _, err := svc.TriggerDailyDigest(context.Background(), now); err == nil || err.Error() != "redis down" {
		t.Fatalf("want redis down got %v", err)
	}

	result, err := svc.TriggerDailyDigest(context.Background(), now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	if len(queue.dates) != 2 {
		t.Fatalf("want queue called twice got %d", len(queue.dates))
	}
	if result.Status != "accepted" {
		t.Fatalf("want retry status accepted got %s", result.Status)
	}
}

func TestJobServiceAllowsSameDigestDateWhenQueueAcceptsAgain(t *testing.T) {
	queue := &enqueueStub{}
	svc := NewJobService(queue)
	now := time.Date(2026, 4, 10, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))

	first, err := svc.TriggerDailyDigest(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.TriggerDailyDigest(context.Background(), now.Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	if len(queue.dates) != 2 {
		t.Fatalf("want queue called twice got %d", len(queue.dates))
	}
	if first.Status != "accepted" || second.Status != "accepted" {
		t.Fatalf("want accepted/accepted got %s/%s", first.Status, second.Status)
	}
}
