ALTER TABLE daily_digests RENAME TO daily_digests_legacy;

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
  '',
  remote_url,
  CASE
    WHEN remote_url <> '' THEN 'published'
    ELSE 'failed'
  END,
  '',
  created_at,
  created_at
FROM daily_digests_legacy;

DROP TABLE daily_digests_legacy;
