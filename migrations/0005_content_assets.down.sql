DROP TABLE IF EXISTS daily_digest_items;
DROP TABLE IF EXISTS article_publish_states;
DROP TABLE IF EXISTS article_dossiers;

ALTER TABLE daily_digests RENAME TO daily_digests_with_content_assets;

CREATE TABLE daily_digests (
  id VARCHAR(36) PRIMARY KEY,
  digest_date DATE NOT NULL UNIQUE,
  title TEXT NOT NULL,
  subtitle TEXT NOT NULL,
  content_markdown TEXT NOT NULL,
  content_html TEXT NOT NULL,
  remote_id TEXT NOT NULL DEFAULT '',
  remote_url TEXT NOT NULL DEFAULT '',
  publish_state TEXT NOT NULL DEFAULT 'failed',
  publish_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO daily_digests (
  id,
  digest_date,
  title,
  subtitle,
  content_markdown,
  content_html,
  remote_id,
  remote_url,
  publish_state,
  publish_error,
  created_at,
  updated_at
)
SELECT
  id,
  digest_date,
  title,
  subtitle,
  content_markdown,
  content_html,
  remote_id,
  remote_url,
  publish_state,
  publish_error,
  created_at,
  updated_at
FROM daily_digests_with_content_assets;

DROP TABLE daily_digests_with_content_assets;

ALTER TABLE article_processings RENAME TO article_processings_with_content_assets;

CREATE TABLE article_processings (
  id VARCHAR(36) PRIMARY KEY,
  article_id VARCHAR(36) NOT NULL,
  title_translated TEXT NOT NULL,
  summary_translated TEXT NOT NULL,
  content_translated TEXT NOT NULL,
  core_summary TEXT NOT NULL,
  key_points_json JSONB NOT NULL,
  topic_category TEXT NOT NULL,
  importance_score DOUBLE PRECISION NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO article_processings (
  id,
  article_id,
  title_translated,
  summary_translated,
  content_translated,
  core_summary,
  key_points_json,
  topic_category,
  importance_score,
  created_at
)
SELECT
  id,
  article_id,
  title_translated,
  summary_translated,
  content_translated,
  core_summary,
  key_points_json,
  topic_category,
  importance_score,
  created_at
FROM article_processings_with_content_assets;

DROP TABLE article_processings_with_content_assets;
