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
	defaults.LLM.FallbackModels = []string{"mimo-v2-pro"}

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
	if len(snapshot.LLM.FallbackModels) != 1 || snapshot.LLM.FallbackModels[0] != "mimo-v2-pro" {
		t.Fatalf("want default fallback model kept got %#v", snapshot.LLM.FallbackModels)
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
	defaults.LLM.FallbackModels = []string{"mimo-v2-pro"}

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
	if len(snapshot.LLM.FallbackModels) != 1 || snapshot.LLM.FallbackModels[0] != "mimo-v2-pro" {
		t.Fatalf("want fallback model mimo-v2-pro got %#v", snapshot.LLM.FallbackModels)
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
			PayloadJSON: []byte(`{"base_url":"https://db.llm.local/v1","model":"gpt-db","api_key":"db-token","fallback_models":["mimo-v2-pro","kimi-k2.5"]}`),
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
	if len(snapshot.LLM.FallbackModels) != 2 || snapshot.LLM.FallbackModels[0] != "mimo-v2-pro" || snapshot.LLM.FallbackModels[1] != "kimi-k2.5" {
		t.Fatalf("unexpected fallback models %#v", snapshot.LLM.FallbackModels)
	}
	if snapshot.Prompts.TranslationVersion != 6 || snapshot.Prompts.DossierVersion != 6 || snapshot.Prompts.DigestVersion != 6 {
		t.Fatalf("unexpected prompt versions %+v", snapshot.Prompts)
	}
	if snapshot.Prompts.DossierPrompt != "D" {
		t.Fatalf("want dossier prompt D got %q", snapshot.Prompts.DossierPrompt)
	}
}

func TestRuntimeConfigServiceDecryptsEncryptedLLMSecret(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeLLM: {
			ProfileType: profile.TypeLLM,
			Version:     2,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://db.llm.local/v1","model":"gpt-db","api_key":"` + encryptedValue(t, "db-token") + `"}`),
		},
	}}
	defaults := &config.Config{}
	defaults.Security.SecretKey = adminConfigTestSecretKey

	svc := service.NewRuntimeConfigService(repo, defaults)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LLM.APIKey != "db-token" {
		t.Fatalf("want decrypted api_key got %q", snapshot.LLM.APIKey)
	}
}

func TestRuntimeConfigServiceIncludesMinifluxAndPublishOverlay(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypeMiniflux: {
			ProfileType: profile.TypeMiniflux,
			Name:        "admin-miniflux",
			Version:     3,
			IsActive:    true,
			PayloadJSON: []byte(`{"base_url":"https://db.miniflux.local","api_token":"db-miniflux-token","fetch_limit":120,"lookback_hours":48}`),
		},
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Name:        "admin-publish",
			Version:     4,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_base_url":"https://db.halo.local","halo_token":"db-halo-token","output_dir":"D:/db-output"}`),
		},
	}}
	defaults := &config.Config{}
	defaults.Miniflux.BaseURL = "https://env.miniflux.local"
	defaults.Miniflux.AuthToken = "env-miniflux-token"
	defaults.Publish.OutputDir = "D:/env-output"

	svc := service.NewRuntimeConfigService(repo, defaults)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.Miniflux.BaseURL != "https://db.miniflux.local" {
		t.Fatalf("want db miniflux base_url got %q", snapshot.Miniflux.BaseURL)
	}
	if snapshot.Miniflux.AuthToken != "db-miniflux-token" {
		t.Fatalf("want db miniflux token got %q", snapshot.Miniflux.AuthToken)
	}
	if snapshot.Miniflux.FetchLimit != 120 || snapshot.Miniflux.LookbackHours != 48 {
		t.Fatalf("unexpected miniflux snapshot %+v", snapshot.Miniflux)
	}
	if snapshot.Publish.Provider != "halo" {
		t.Fatalf("want halo publish provider got %q", snapshot.Publish.Provider)
	}
	if snapshot.Publish.HaloBaseURL != "https://db.halo.local" || snapshot.Publish.HaloToken != "db-halo-token" {
		t.Fatalf("unexpected publish halo config %+v", snapshot.Publish)
	}
	if snapshot.Publish.OutputDir != "D:/db-output" {
		t.Fatalf("want db output_dir got %q", snapshot.Publish.OutputDir)
	}
}

func TestRuntimeConfigServiceTreatsDefaultPublishSeedAsFallbackForEnvMarkdownExport(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Name:        "default-publish",
			Version:     1,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_base_url":"","halo_token":"","output_dir":""}`),
		},
	}}
	defaults := &config.Config{}
	defaults.Publish.OutputDir = "D:/env-output"

	svc := service.NewRuntimeConfigService(repo, defaults)
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if snapshot.Publish.Provider != "markdown_export" {
		t.Fatalf("want markdown_export provider got %q", snapshot.Publish.Provider)
	}
	if snapshot.Publish.OutputDir != "D:/env-output" {
		t.Fatalf("want env output_dir got %q", snapshot.Publish.OutputDir)
	}
	if snapshot.Publish.HaloBaseURL != "" || snapshot.Publish.HaloToken != "" {
		t.Fatalf("default publish seed should not force halo config %+v", snapshot.Publish)
	}
}

func TestRuntimeConfigServiceIncludesArticlePublishFlowConfig(t *testing.T) {
	repo := &profileRepoStub{active: map[string]profile.Version{
		profile.TypePublish: {
			ProfileType: profile.TypePublish,
			Name:        "admin-publish",
			Version:     5,
			IsActive:    true,
			PayloadJSON: []byte(`{"provider":"halo","halo_base_url":"https://db.halo.local","halo_token":"db-halo-token","output_dir":"D:/db-output","article_publish_mode":"all","article_review_mode":"manual_review"}`),
		},
	}}

	svc := service.NewRuntimeConfigService(repo, &config.Config{})
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Publish.ArticlePublishMode != "all" {
		t.Fatalf("want article_publish_mode all got %q", snapshot.Publish.ArticlePublishMode)
	}
	if snapshot.Publish.ArticleReviewMode != "manual_review" {
		t.Fatalf("want article_review_mode manual_review got %q", snapshot.Publish.ArticleReviewMode)
	}
}
