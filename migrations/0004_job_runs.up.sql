CREATE TABLE job_runs (
  id VARCHAR(36) PRIMARY KEY,
  job_type TEXT NOT NULL,
  trigger_source TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  digest_date DATE,
  detail_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  error_message TEXT NOT NULL DEFAULT '',
  requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);

CREATE INDEX idx_job_runs_type_requested_at ON job_runs (job_type, requested_at DESC);
