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

CREATE TABLE daily_digests (
  id VARCHAR(36) PRIMARY KEY,
  digest_date DATE NOT NULL UNIQUE,
  title TEXT NOT NULL,
  subtitle TEXT NOT NULL,
  content_markdown TEXT NOT NULL,
  content_html TEXT NOT NULL,
  remote_url TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
