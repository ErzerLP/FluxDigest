CREATE TABLE source_articles (
  id VARCHAR(36) PRIMARY KEY,
  miniflux_entry_id BIGINT UNIQUE NOT NULL,
  feed_id BIGINT NOT NULL,
  feed_title TEXT NOT NULL,
  title TEXT NOT NULL,
  author TEXT NOT NULL,
  url TEXT NOT NULL,
  content_html TEXT NOT NULL,
  content_text TEXT NOT NULL,
  fingerprint TEXT UNIQUE NOT NULL
);

CREATE TABLE profile_versions (
  id VARCHAR(36) PRIMARY KEY,
  profile_type TEXT NOT NULL,
  name TEXT NOT NULL,
  version INT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT FALSE,
  payload_json JSONB NOT NULL
);

CREATE INDEX idx_profile_versions_profile_type ON profile_versions (profile_type);
