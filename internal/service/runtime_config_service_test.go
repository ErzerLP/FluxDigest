package service_test

import (
	"context"
	"testing"

	"rss-platform/internal/config"
	"rss-platform/internal/domain/profile"
	"rss-platform/internal/service"
)

func TestRuntimeConfigServiceUsesProfilePayloadBeforeEnvDefaults(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://db.llm.local/v1","model":"gpt-db","api_key":"db-token","timeout_ms":45000}`),
		},
		profile.TypeScheduler: {
			ProfileType: profile.TypeScheduler,
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"schedule_enabled":false,"schedule_time":"09:30","timezone":"UTC"}`),
		},
	}}
	defaults := &config.Config{}
	defaults.LLM.BaseURL = "https://env.llm.local/v1"
	defaults.LLM.Model = "gpt-env"
	defaults.LLM.APIKey = "env-token"
	defaults.LLM.TimeoutMS = 30000

	svc := service.NewRuntimeConfigService(repo, defaults)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.LLM.BaseURL != "https://db.llm.local/v1" {
		t.Fatalf("want db llm base_url got %q", snapshot.LLM.BaseURL)
	}
	if snapshot.LLM.Model != "gpt-db" {
		t.Fatalf("want db llm model got %q", snapshot.LLM.Model)
	}
	if snapshot.LLM.APIKey != "db-token" {
		t.Fatalf("want db llm api_key got %q", snapshot.LLM.APIKey)
	}
	if snapshot.LLM.TimeoutMS != 45000 {
		t.Fatalf("want db llm timeout 45000 got %d", snapshot.LLM.TimeoutMS)
	}
	if snapshot.Scheduler.Enabled {
		t.Fatal("want scheduler enabled=false from profile payload")
	}
	if snapshot.Scheduler.ScheduleTime != "09:30" {
		t.Fatalf("want schedule_time 09:30 got %q", snapshot.Scheduler.ScheduleTime)
	}
	if snapshot.Scheduler.Timezone != "UTC" {
		t.Fatalf("want timezone UTC got %q", snapshot.Scheduler.Timezone)
	}
}

func TestRuntimeConfigServiceKeepsFallbackForDefaultSeedEmptyValues(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Name:        "default-llm",
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"","model":"","api_key":""}`),
		},
	}}
	defaults := &config.Config{}
	defaults.LLM.BaseURL = "https://env.llm.local/v1"
	defaults.LLM.Model = "gpt-env"
	defaults.LLM.APIKey = "env-token"
	defaults.LLM.TimeoutMS = 30000

	svc := service.NewRuntimeConfigService(repo, defaults)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.LLM.BaseURL != "https://env.llm.local/v1" {
		t.Fatalf("want fallback base_url got %q", snapshot.LLM.BaseURL)
	}
	if snapshot.LLM.Model != "gpt-env" {
		t.Fatalf("want fallback model got %q", snapshot.LLM.Model)
	}
	if snapshot.LLM.APIKey != "env-token" {
		t.Fatalf("want fallback api_key got %q", snapshot.LLM.APIKey)
	}
	if snapshot.LLM.TimeoutMS != 30000 {
		t.Fatalf("want fallback timeout 30000 got %d", snapshot.LLM.TimeoutMS)
	}
}

func TestRuntimeConfigServiceAllowsAdminProfileToClearValues(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Name:        "admin-llm",
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"","model":"gpt-db","api_key":""}`),
		},
	}}
	defaults := &config.Config{}
	defaults.LLM.BaseURL = "https://env.llm.local/v1"
	defaults.LLM.Model = "gpt-env"
	defaults.LLM.APIKey = "env-token"

	svc := service.NewRuntimeConfigService(repo, defaults)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.LLM.BaseURL != "" {
		t.Fatalf("want cleared base_url got %q", snapshot.LLM.BaseURL)
	}
	if snapshot.LLM.APIKey != "" {
		t.Fatalf("want cleared api_key got %q", snapshot.LLM.APIKey)
	}
	if snapshot.LLM.Model != "gpt-db" {
		t.Fatalf("want db model got %q", snapshot.LLM.Model)
	}
}

func TestRuntimeConfigServiceAllowsAdminProfileToClearModel(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Name:        "admin-llm",
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://db.llm.local/v1","model":"","api_key":"db-token"}`),
		},
	}}
	defaults := &config.Config{}
	defaults.LLM.BaseURL = "https://env.llm.local/v1"
	defaults.LLM.Model = "gpt-env"
	defaults.LLM.APIKey = "env-token"

	svc := service.NewRuntimeConfigService(repo, defaults)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.LLM.Model != "" {
		t.Fatalf("want cleared model got %q", snapshot.LLM.Model)
	}
}

func TestRuntimeConfigServiceSnapshotLoadsPromptProfileVersions(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     4,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://db.llm.local/v1","model":"gpt-db","api_key":"db-token"}`),
		},
		profile.TypeScheduler: {
			ProfileType: profile.TypeScheduler,
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"schedule_enabled":true,"schedule_time":"07:00","timezone":"Asia/Shanghai"}`),
		},
		profile.TypePrompts: {
			ProfileType: profile.TypePrompts,
			Version:     6,
			IsActive:    true,
			PayloadJSON: []byte(`{"translation_prompt":"T","analysis_prompt":"A","dossier_prompt":"D","digest_prompt":"G"}`),
		},
	}}

	svc := service.NewRuntimeConfigService(repo, &config.Config{})
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LLM.Version != 4 {
		t.Fatalf("want llm version 4 got %d", snapshot.LLM.Version)
	}
	if snapshot.Prompts.TranslationVersion != 6 || snapshot.Prompts.DossierVersion != 6 || snapshot.Prompts.DigestVersion != 6 {
		t.Fatalf("unexpected prompt versions %+v", snapshot.Prompts)
	}
	if snapshot.Prompts.DossierPrompt != "D" {
		t.Fatalf("want dossier prompt D got %q", snapshot.Prompts.DossierPrompt)
	}
}
