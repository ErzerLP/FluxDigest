package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"rss-platform/internal/service"

	"gorm.io/gorm"
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
	return service.JobRunRecord{}, gorm.ErrRecordNotFound
}

type digestLatestStub struct {
	view service.DigestView
	err  error
}

func (s digestLatestStub) LatestDigest(_ context.Context) (service.DigestView, error) {
	if s.err != nil {
		return service.DigestView{}, s.err
	}
	return s.view, nil
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
	svc := service.NewAdminStatusServiceWithDigest(configs, jobs, digestLatestStub{view: service.DigestView{DigestDate: "2026-04-11", PublishState: "published"}})

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
	if status.Runtime.LatestDigestStatus != "published" {
		t.Fatalf("want published got %q", status.Runtime.LatestDigestStatus)
	}
}

func TestAdminStatusServiceIncludesMinifluxAndPublisherConfig(t *testing.T) {
	t.Run("halo publisher", func(t *testing.T) {
		configs := adminConfigSnapshotStub{snapshot: service.AdminConfigSnapshot{
			LLM: service.LLMConfigView{BaseURL: "https://llm.local/v1", APIKey: service.SecretView{IsSet: true}},
			Miniflux: service.MinifluxConfigView{
				BaseURL:  "https://miniflux.local",
				APIToken: service.SecretView{IsSet: true},
			},
			Publish: service.PublishConfigView{
				Provider:    "halo",
				HaloBaseURL: "https://halo.local",
				HaloToken:   service.SecretView{IsSet: true},
			},
		}}
		jobs := jobRunRepoStub{latestByType: map[string]service.JobRunRecord{
			"miniflux_test": {JobType: "miniflux_test", Status: "ok", FinishedAt: mustRFC3339("2026-04-11T18:30:00+08:00")},
			"publish_test":  {JobType: "publish_test", Status: "ok", FinishedAt: mustRFC3339("2026-04-11T18:31:00+08:00")},
		}}
		svc := service.NewAdminStatusServiceWithDigest(configs, jobs, digestLatestStub{err: gorm.ErrRecordNotFound})

		status, err := svc.GetStatus(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !status.Integrations.Miniflux.Configured {
			t.Fatalf("expected miniflux configured, got %+v", status.Integrations.Miniflux)
		}
		if status.Integrations.Miniflux.LastTestStatus != "ok" {
			t.Fatalf("want miniflux last test ok got %+v", status.Integrations.Miniflux)
		}
		if !status.Integrations.Publisher.Configured {
			t.Fatalf("expected publisher configured, got %+v", status.Integrations.Publisher)
		}
		if status.Integrations.Publisher.LastTestStatus != "ok" {
			t.Fatalf("want publish last test ok got %+v", status.Integrations.Publisher)
		}
	})

	t.Run("markdown export publisher", func(t *testing.T) {
		configs := adminConfigSnapshotStub{snapshot: service.AdminConfigSnapshot{
			Publish: service.PublishConfigView{
				Provider:  "markdown_export",
				OutputDir: "D:/digests",
			},
		}}
		svc := service.NewAdminStatusServiceWithDigest(configs, jobRunRepoStub{}, digestLatestStub{err: gorm.ErrRecordNotFound})

		status, err := svc.GetStatus(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !status.Integrations.Publisher.Configured {
			t.Fatalf("expected markdown export publisher configured, got %+v", status.Integrations.Publisher)
		}
	})
}

func TestAdminStatusServiceAllowsNotFound(t *testing.T) {
	configs := adminConfigSnapshotStub{snapshot: service.AdminConfigSnapshot{LLM: service.LLMConfigView{BaseURL: "https://llm.local/v1", APIKey: service.SecretView{IsSet: true}}}}
	svc := service.NewAdminStatusServiceWithDigest(configs, jobRunRepoStub{}, digestLatestStub{err: gorm.ErrRecordNotFound})

	status, err := svc.GetStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Runtime.LatestDigestStatus != "" {
		t.Fatalf("expected empty digest status got %q", status.Runtime.LatestDigestStatus)
	}
}

func TestAdminStatusServiceReturnsErrorOnRepoFailure(t *testing.T) {
	configs := adminConfigSnapshotStub{snapshot: service.AdminConfigSnapshot{LLM: service.LLMConfigView{BaseURL: "https://llm.local/v1", APIKey: service.SecretView{IsSet: true}}}}
	err := errors.New("db down")
	svc := service.NewAdminStatusServiceWithDigest(configs, jobRunRepoStub{err: err}, digestLatestStub{})

	_, got := svc.GetStatus(context.Background())
	if !errors.Is(got, err) {
		t.Fatalf("expected error %v got %v", err, got)
	}
}
