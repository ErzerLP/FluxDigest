package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type enqueueStub struct {
	dates               []string
	dailyDigestForces   []bool
	errs                []error
	reprocessArticleIDs []string
	reprocessForceFlags []bool
	reprocessErrs       []error
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

func (s *enqueueStub) EnqueueDailyDigestWithOptions(_ context.Context, digestDate string, opts DailyDigestTriggerOptions) error {
	s.dates = append(s.dates, digestDate)
	s.dailyDigestForces = append(s.dailyDigestForces, opts.Force)
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

func (s *enqueueStub) EnqueueArticleReprocess(_ context.Context, articleID string, force bool) error {
	s.reprocessArticleIDs = append(s.reprocessArticleIDs, articleID)
	s.reprocessForceFlags = append(s.reprocessForceFlags, force)
	if len(s.reprocessErrs) == 0 {
		return nil
	}
	err := s.reprocessErrs[0]
	s.reprocessErrs = s.reprocessErrs[1:]
	return err
}

func TestJobServiceSkipsDuplicateDigestDate(t *testing.T) {
	queue := &enqueueStub{errs: []error{ErrDailyDigestAlreadyQueued}}
	svc := NewJobService(queue, nil)
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
	svc := NewJobService(queue, nil)
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
	svc := NewJobService(queue, nil)
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
	svc := NewJobService(queue, nil)
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

func TestJobServiceTriggerArticleReprocessAccepted(t *testing.T) {
	queue := &enqueueStub{}
	svc := NewJobService(nil, queue)

	result, err := svc.TriggerArticleReprocess(context.Background(), "art-1", true)
	if err != nil {
		t.Fatal(err)
	}

	if len(queue.reprocessArticleIDs) != 1 {
		t.Fatalf("want queue called once got %d", len(queue.reprocessArticleIDs))
	}
	if queue.reprocessArticleIDs[0] != "art-1" {
		t.Fatalf("want article id art-1 got %s", queue.reprocessArticleIDs[0])
	}
	if len(queue.reprocessForceFlags) != 1 || !queue.reprocessForceFlags[0] {
		t.Fatalf("want force=true got %#v", queue.reprocessForceFlags)
	}
	if result.Status != "accepted" {
		t.Fatalf("want accepted status got %s", result.Status)
	}
}

func TestJobServiceTriggerArticleReprocessRequiresQueue(t *testing.T) {
	svc := NewJobService(nil, nil)

	_, err := svc.TriggerArticleReprocess(context.Background(), "art-1", false)
	if !errors.Is(err, errArticleReprocessQueueRequired) {
		t.Fatalf("want errArticleReprocessQueueRequired got %v", err)
	}
}

func TestJobServiceTriggerDailyDigestWithForce(t *testing.T) {
	queue := &enqueueStub{}
	svc := NewJobService(queue, nil)
	now := time.Date(2026, 4, 9, 16, 30, 0, 0, time.UTC)

	result, err := svc.TriggerDailyDigestWithOptions(context.Background(), now, DailyDigestTriggerOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(queue.dates) != 1 || queue.dates[0] != "2026-04-10" {
		t.Fatalf("want digest date 2026-04-10 got %#v", queue.dates)
	}
	if len(queue.dailyDigestForces) != 1 || !queue.dailyDigestForces[0] {
		t.Fatalf("want force=true got %#v", queue.dailyDigestForces)
	}
	if result.Status != "accepted" {
		t.Fatalf("want accepted status got %s", result.Status)
	}
}

func TestJobServiceSkipsDuplicateDigestDateWhenForceTaskAlreadyQueued(t *testing.T) {
	queue := &enqueueStub{errs: []error{ErrDailyDigestAlreadyQueued}}
	svc := NewJobService(queue, nil)
	now := time.Date(2026, 4, 10, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))

	result, err := svc.TriggerDailyDigestWithOptions(context.Background(), now, DailyDigestTriggerOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(queue.dates) != 1 || queue.dates[0] != "2026-04-10" {
		t.Fatalf("want digest date 2026-04-10 got %#v", queue.dates)
	}
	if len(queue.dailyDigestForces) != 1 || !queue.dailyDigestForces[0] {
		t.Fatalf("want force=true got %#v", queue.dailyDigestForces)
	}
	if result.Status != "skipped" {
		t.Fatalf("want skipped status got %s", result.Status)
	}
}

func TestJobServiceSkipsDuplicateArticleReprocessWhenNotForce(t *testing.T) {
	queue := &enqueueStub{reprocessErrs: []error{ErrArticleReprocessAlreadyQueued}}
	svc := NewJobService(nil, queue)

	result, err := svc.TriggerArticleReprocess(context.Background(), "art-1", false)
	if err != nil {
		t.Fatal(err)
	}

	if result.Status != "skipped" {
		t.Fatalf("want skipped status got %s", result.Status)
	}
}
