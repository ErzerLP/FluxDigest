ALTER TABLE article_processings ADD COLUMN translation_prompt_version INT NOT NULL DEFAULT 1;
ALTER TABLE article_processings ADD COLUMN analysis_prompt_version INT NOT NULL DEFAULT 1;
ALTER TABLE article_processings ADD COLUMN llm_profile_version INT NOT NULL DEFAULT 1;
ALTER TABLE article_processings ADD COLUMN status TEXT NOT NULL DEFAULT 'completed';
ALTER TABLE article_processings ADD COLUMN error_message TEXT NOT NULL DEFAULT '';
ALTER TABLE article_processings ADD COLUMN processed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE daily_digests ADD COLUMN digest_prompt_version INT NOT NULL DEFAULT 1;
ALTER TABLE daily_digests ADD COLUMN llm_profile_version INT NOT NULL DEFAULT 1;

CREATE TABLE article_dossiers (
  id VARCHAR(36) PRIMARY KEY,
  article_id VARCHAR(36) NOT NULL,
  processing_id VARCHAR(36) NOT NULL,
  digest_date DATE NOT NULL,
  version INT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  title_translated TEXT NOT NULL,
  summary_polished TEXT NOT NULL,
  core_summary TEXT NOT NULL,
  key_points_json JSONB NOT NULL DEFAULT '[]',
  topic_category TEXT NOT NULL,
  importance_score DOUBLE PRECISION NOT NULL,
  recommendation_reason TEXT NOT NULL DEFAULT '',
  reading_value TEXT NOT NULL DEFAULT '',
  priority_level TEXT NOT NULL DEFAULT 'normal',
  content_polished_markdown TEXT NOT NULL,
  analysis_longform_markdown TEXT NOT NULL,
  background_context TEXT NOT NULL DEFAULT '',
  impact_analysis TEXT NOT NULL DEFAULT '',
  debate_points_json JSONB NOT NULL DEFAULT '[]',
  target_audience TEXT NOT NULL DEFAULT '',
  publish_suggestion TEXT NOT NULL DEFAULT 'draft',
  suggestion_reason TEXT NOT NULL DEFAULT '',
  suggested_channels_json JSONB NOT NULL DEFAULT '[]',
  suggested_tags_json JSONB NOT NULL DEFAULT '[]',
  suggested_categories_json JSONB NOT NULL DEFAULT '[]',
  translation_prompt_version INT NOT NULL,
  analysis_prompt_version INT NOT NULL,
  dossier_prompt_version INT NOT NULL,
  llm_profile_version INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_article_dossiers_article_version ON article_dossiers (article_id, version);
CREATE UNIQUE INDEX idx_article_dossiers_article_active ON article_dossiers (article_id) WHERE is_active = TRUE;
CREATE INDEX idx_article_dossiers_digest_date ON article_dossiers (digest_date, is_active);

CREATE TABLE article_publish_states (
  id VARCHAR(36) PRIMARY KEY,
  dossier_id VARCHAR(36) NOT NULL UNIQUE,
  state TEXT NOT NULL,
  approved_by TEXT NOT NULL DEFAULT '',
  decision_note TEXT NOT NULL DEFAULT '',
  publish_channel TEXT NOT NULL DEFAULT '',
  remote_id TEXT NOT NULL DEFAULT '',
  remote_url TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE daily_digest_items (
  id VARCHAR(36) PRIMARY KEY,
  digest_id VARCHAR(36) NOT NULL,
  dossier_id VARCHAR(36) NOT NULL,
  section_name TEXT NOT NULL,
  importance_bucket TEXT NOT NULL DEFAULT 'normal',
  position INT NOT NULL,
  is_featured BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
