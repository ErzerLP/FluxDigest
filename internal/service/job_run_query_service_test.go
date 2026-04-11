package service

import (
	"context"
	"testing"
	"time"

	"rss-platform/internal/repository/postgres/models"
)

func TestJobRunQueryServiceListLatest(t *testing.T) {
	db := newQueryTestDB(t)
	if err := db.AutoMigrate(&models.JobRunModel{}); err != nil {
		t.Fatalf("auto migrate job runs: %v", err)
	}

	ctx := context.Background()
	firstRequestedAt := time.Date(2026, 4, 11, 7, 0, 0, 0, time.UTC)
	secondRequestedAt := firstRequestedAt.Add(time.Minute)
	digestDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	records := []models.JobRunModel{
		{
			ID:          "run-1",
			JobType:     "daily_digest_run",
			Status:      "succeeded",
			DigestDate:  &digestDate,
			DetailJSON:  []byte(`{"message":"ok"}`),
			RequestedAt: firstRequestedAt,
		},
		{
			ID:          "run-2",
			JobType:     "llm_test",
			Status:      "ok",
			DetailJSON:  []byte(`{"message":"ok","latency_ms":123}`),
			RequestedAt: secondRequestedAt,
			FinishedAt:  ptrTime(secondRequestedAt),
		},
	}

	for _, record := range records {
		if err := db.WithContext(ctx).Create(&record).Error; err != nil {
			t.Fatalf("create job run: %v", err)
		}
	}

	svc := NewJobRunQueryService(db)
	runs, err := svc.ListLatest(ctx, JobRunListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs got %d", len(runs))
	}
	if runs[0].JobType != "llm_test" {
		t.Fatalf("want latest llm_test got %s", runs[0].JobType)
	}
	if runs[0].ID != "run-2" {
		t.Fatalf("want latest id run-2 got %s", runs[0].ID)
	}
	if runs[1].DigestDate != "2026-04-10" {
		t.Fatalf("want digest date 2026-04-10 got %s", runs[1].DigestDate)
	}
	if runs[0].Detail["latency_ms"] != float64(123) {
		t.Fatalf("unexpected detail latency: %#v", runs[0].Detail["latency_ms"])
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
