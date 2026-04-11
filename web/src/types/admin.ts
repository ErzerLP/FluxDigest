export type SecretMode = 'keep' | 'replace' | 'clear';

export type RecordValue = Record<string, unknown>;

export interface SecretView {
  is_set?: boolean;
  masked_value?: string;
}

export interface LLMConfigView {
  base_url?: string;
  model?: string;
  api_key?: SecretView;
}

export interface AdminConfigSnapshot {
  llm?: LLMConfigView;
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
  };
}

export interface SecretInput {
  mode: SecretMode;
  value?: string;
}

export interface UpdateLLMConfigInput {
  base_url?: string;
  model?: string;
  api_key?: SecretInput;
}

export interface LLMTestDraft {
  base_url?: string;
  model?: string;
  api_key?: string;
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

export interface JobRunListResponse {
  items?: JobRunRecord[];
}
