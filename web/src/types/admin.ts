export type SecretMode = 'keep' | 'replace' | 'clear';

export type RecordValue = Record<string, unknown>;
export type PublishProvider = 'halo' | 'markdown_export';

export interface SecretView {
  is_set?: boolean;
  masked_value?: string;
}

export interface LLMConfigView {
  base_url?: string;
  model?: string;
  timeout_ms?: number;
  api_key?: SecretView;
}

export interface MinifluxConfigView {
  base_url?: string;
  fetch_limit?: number;
  lookback_hours?: number;
  api_token?: SecretView;
}

export interface PublishConfigView {
  provider?: PublishProvider;
  halo_base_url?: string;
  halo_token?: SecretView;
  output_dir?: string;
}

export interface PromptConfigView {
  target_language?: string;
  translation_prompt?: string;
  analysis_prompt?: string;
  dossier_prompt?: string;
  digest_prompt?: string;
}

export interface AdminConfigSnapshot {
  llm?: LLMConfigView;
  miniflux?: MinifluxConfigView;
  publish?: PublishConfigView;
  prompts?: PromptConfigView;
}

export interface IntegrationState {
  configured?: boolean;
  last_test_status?: string;
  last_test_at?: string;
}

export interface AdminStatus {
  system?: {
    api?: string;
    db?: string;
    redis?: string;
  };
  integrations?: {
    llm?: IntegrationState;
    miniflux?: IntegrationState;
    publisher?: IntegrationState;
  };
  runtime?: {
    latest_digest_date?: string;
    latest_digest_status?: string;
    latest_job_status?: string;
    latest_job_id?: string;
  };
}

export interface SecretInput {
  mode: SecretMode;
  value?: string;
}

export interface UpdateLLMConfigInput {
  base_url?: string;
  model?: string;
  timeout_ms?: number;
  api_key?: SecretInput;
}

export interface UpdateMinifluxConfigInput {
  base_url?: string;
  fetch_limit?: number;
  lookback_hours?: number;
  api_token?: SecretInput;
}

export interface UpdatePublishConfigInput {
  provider?: PublishProvider;
  halo_base_url?: string;
  halo_token?: SecretInput;
  output_dir?: string;
}

export interface UpdatePromptConfigInput {
  target_language?: string;
  translation_prompt?: string;
  analysis_prompt?: string;
  dossier_prompt?: string;
  digest_prompt?: string;
}

export interface LLMTestDraft {
  base_url?: string;
  model?: string;
  api_key?: string;
  timeout_ms?: number;
}

export interface ConnectivityTestResult {
  status?: string;
  message?: string;
  latency_ms?: number;
}

export interface ProfileVersion {
  id?: string;
  profile_type?: string;
  name?: string;
  version?: number;
  is_active?: boolean;
  ok?: boolean;
}

export interface JobRunRecord {
  id?: string;
  job_type?: string;
  trigger_source?: string;
  status?: string;
  digest_date?: string;
  detail?: RecordValue;
  error_message?: string;
  requested_at?: string;
  started_at?: string;
  finished_at?: string;
}

export interface JobRunDetail extends JobRunRecord {}

export interface JobRunListResponse {
  items?: JobRunRecord[];
}

export interface RunJobResponse {
  ok?: boolean;
  job_id?: string;
  status?: string;
}
