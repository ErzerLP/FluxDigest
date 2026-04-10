package service

import (
	"context"
	"testing"
	"time"
)

type enqueueStub struct {
	dates []string
}

func (s *enqueueStub) EnqueueDailyDigest(_ context.Context, digestDate string) error {
	s.dates = append(s.dates, digestDate)
	return nil
}

func TestJobServiceSkipsDuplicateDigestDate(t *testing.T) {
	queue := &enqueueStub{}
	svc := NewJobService(queue)
	now := time.Date(2026, 4, 10, 7, 0, 0, 0, time.FixedZone("CST", 8*3600))

	if err := svc.TriggerDailyDigest(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err := svc.TriggerDailyDigest(context.Background(), now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if len(queue.dates) != 1 {
		t.Fatalf("want queue called once got %d", len(queue.dates))
	}
	if queue.dates[0] != "2026-04-10" {
		t.Fatalf("want digest date 2026-04-10 got %s", queue.dates[0])
	}
}
