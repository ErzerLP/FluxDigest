package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"rss-platform/internal/service"
)

type adminConfigSnapshotStub struct {
	snapshot service.AdminConfigSnapshot
	err      error
}

func (s adminConfigSnapshotStub) GetSnapshot(_ context.Context) (service.AdminConfigSnapshot, error) {
	if s.err != nil {
		return service.AdminConfigSnapshot{}, s.err
	}
	return s.snapshot, nil
}

type jobRunRepoStub struct {
	latestByType map[string]service.JobRunRecord
	err          error
}

func (s jobRunRepoStub) LatestByType(_ context.Context, jobType string) (service.JobRunRecord, error) {
	if s.err != nil {
		return service.JobRunRecord{}, s.err
	}
	if s.latestByType != nil {
		if record, ok := s.latestByType[jobType]; ok {
			return record, nil
		}
	}
	return service.JobRunRecord{}, errors.New("not found")
}

func mustRFC3339(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func TestAdminStatusServiceBuildsDashboardState(t *testing.T) {
	configs := adminConfigSnapshotStub{snapshot: service.AdminConfigSnapshot{LLM: service.LLMConfigView{BaseURL: "https://llm.local/v1", APIKey: service.SecretView{IsSet: true}}}}
	jobs := jobRunRepoStub{latestByType: map[string]service.JobRunRecord{
		"llm_test":         {JobType: "llm_test", Status: "ok", FinishedAt: mustRFC3339("2026-04-11T18:00:00+08:00")},
		"daily_digest_run": {JobType: "daily_digest_run", Status: "succeeded", DigestDate: "2026-04-11"},
	}}
	svc := service.NewAdminStatusService(configs, jobs)

	status, err := svc.GetStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Integrations.LLM.Configured {
		t.Fatal("expected llm to be configured")
	}
	if status.Runtime.LatestJobStatus != "succeeded" {
		t.Fatalf("want succeeded got %q", status.Runtime.LatestJobStatus)
	}
}
