ALTER TABLE daily_digests RENAME TO daily_digests_with_publish_state;

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

INSERT INTO daily_digests (
  id,
  digest_date,
  title,
  subtitle,
  content_markdown,
  content_html,
  remote_url,
  created_at
)
SELECT
  id,
  digest_date,
  title,
  subtitle,
  content_markdown,
  content_html,
  remote_url,
  created_at
FROM daily_digests_with_publish_state;

DROP TABLE daily_digests_with_publish_state;
