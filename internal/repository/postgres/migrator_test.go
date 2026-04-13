package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigratorExecutesUpMigrationsInOrderAndSkipsAppliedFiles(t *testing.T) {
	tempDir := t.TempDir()
	db := openSQLiteDB(t, filepath.Join(tempDir, "runtime.db"))

	if err := writeMigrationFile(tempDir, "0001_create_widgets.up.sql", `
CREATE TABLE widgets (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);
`); err != nil {
		t.Fatal(err)
	}
	if err := writeMigrationFile(tempDir, "0002_seed_widgets.up.sql", `
INSERT INTO widgets (name) VALUES ('runtime-bootstrap');
`); err != nil {
		t.Fatal(err)
	}
	if err := writeMigrationFile(tempDir, "0002_seed_widgets.down.sql", `
DELETE FROM widgets;
`); err != nil {
		t.Fatal(err)
	}

	migrator := NewMigrator(db, tempDir)
	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM widgets`).Scan(&count); err != nil {
		t.Fatalf("count widgets: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 seeded row got %d", count)
	}

	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if applied != 2 {
		t.Fatalf("want 2 applied migrations got %d", applied)
	}
}

func TestMigratorWithLockSerializesSQLiteFallback(t *testing.T) {
	tempDir := t.TempDir()
	db := openSQLiteDB(t, filepath.Join(tempDir, "runtime.db"))
	migrator := NewMigrator(db, tempDir)

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var active int32
	var maxActive int32
	var wg sync.WaitGroup
	wg.Add(2)

	run := func() {
		defer wg.Done()
		err := migrator.WithLock(context.Background(), func(context.Context) error {
			current := atomic.AddInt32(&active, 1)
			defer atomic.AddInt32(&active, -1)
			for {
				seen := atomic.LoadInt32(&maxActive)
				if current <= seen || atomic.CompareAndSwapInt32(&maxActive, seen, current) {
					break
				}
			}
			started <- struct{}{}
			<-release
			return nil
		})
		if err != nil {
			t.Errorf("with lock: %v", err)
		}
	}

	go run()
	<-started
	go run()

	select {
	case <-started:
		t.Fatal("second callback entered lock before first released")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	wg.Wait()

	if maxActive != 1 {
		t.Fatalf("want max active callback 1 got %d", maxActive)
	}
}

func TestMigratorAppliesRealRuntimeStateMigration(t *testing.T) {
	tempDir := t.TempDir()
	db := openSQLiteDB(t, filepath.Join(tempDir, "runtime.db"))
	migrator := NewMigrator(db, projectMigrationsDir(t))

	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate project files: %v", err)
	}

	processingColumns := tableInfoByName(t, db, "article_processings")
	keyPointsColumn, ok := processingColumns["key_points_json"]
	if !ok {
		t.Fatal("missing article_processings.key_points_json")
	}
	if !strings.EqualFold(keyPointsColumn.Type, "JSONB") {
		t.Fatalf("want key_points_json type JSONB got %q", keyPointsColumn.Type)
	}
	processingCreatedAt := processingColumns["created_at"]
	if !strings.EqualFold(processingCreatedAt.DefaultValue, "CURRENT_TIMESTAMP") {
		t.Fatalf("want article_processings.created_at default CURRENT_TIMESTAMP got %q", processingCreatedAt.DefaultValue)
	}

	digestColumns := tableInfoByName(t, db, "daily_digests")
	digestDateColumn, ok := digestColumns["digest_date"]
	if !ok {
		t.Fatal("missing daily_digests.digest_date")
	}
	if !strings.EqualFold(digestDateColumn.Type, "DATE") {
		t.Fatalf("want digest_date type DATE got %q", digestDateColumn.Type)
	}
	remoteURLColumn := digestColumns["remote_url"]
	if remoteURLColumn.DefaultValue != "''" {
		t.Fatalf("want remote_url default '' got %q", remoteURLColumn.DefaultValue)
	}
	publishStateColumn := digestColumns["publish_state"]
	if publishStateColumn.DefaultValue != "'failed'" {
		t.Fatalf("want publish_state default 'failed' got %q", publishStateColumn.DefaultValue)
	}
	publishErrorColumn := digestColumns["publish_error"]
	if publishErrorColumn.DefaultValue != "''" {
		t.Fatalf("want publish_error default '' got %q", publishErrorColumn.DefaultValue)
	}
	remoteIDColumn := digestColumns["remote_id"]
	if remoteIDColumn.DefaultValue != "''" {
		t.Fatalf("want remote_id default '' got %q", remoteIDColumn.DefaultValue)
	}
	updatedAtColumn := digestColumns["updated_at"]
	if !strings.EqualFold(updatedAtColumn.DefaultValue, "CURRENT_TIMESTAMP") {
		t.Fatalf("want updated_at default CURRENT_TIMESTAMP got %q", updatedAtColumn.DefaultValue)
	}
	digestCreatedAt := digestColumns["created_at"]
	if !strings.EqualFold(digestCreatedAt.DefaultValue, "CURRENT_TIMESTAMP") {
		t.Fatalf("want daily_digests.created_at default CURRENT_TIMESTAMP got %q", digestCreatedAt.DefaultValue)
	}

	if _, err := db.Exec(`
INSERT INTO article_processings (
  id, article_id, title_translated, summary_translated, content_translated, core_summary,
  key_points_json, topic_category, importance_score
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, "proc-1", "article-1", "标题", "摘要", "全文", "核心总结", `["k1","k2"]`, "ai", 0.8); err != nil {
		t.Fatalf("insert article_processings: %v", err)
	}

	if _, err := db.Exec(`
INSERT INTO daily_digests (
  id, digest_date, title, subtitle, content_markdown, content_html
) VALUES (?, ?, ?, ?, ?, ?)
`, "digest-1", "2026-04-11", "日报", "副标题", "# 摘要", "<p>摘要</p>"); err != nil {
		t.Fatalf("insert daily_digests: %v", err)
	}

	var remoteURL string
	var publishState string
	var createdAt string
	var updatedAt string
	if err := db.QueryRow(`SELECT remote_url, publish_state, created_at, updated_at FROM daily_digests WHERE id = ?`, "digest-1").Scan(&remoteURL, &publishState, &createdAt, &updatedAt); err != nil {
		t.Fatalf("query daily_digests defaults: %v", err)
	}
	if remoteURL != "" {
		t.Fatalf("want empty remote_url got %q", remoteURL)
	}
	if publishState != "failed" {
		t.Fatalf("want failed publish_state got %q", publishState)
	}
	if createdAt == "" || updatedAt == "" {
		t.Fatal("want timestamp defaults, got empty")
	}
}

func TestMigratorAppliesContentAssetFoundationMigration(t *testing.T) {
	tempDir := t.TempDir()
	db := openSQLiteDB(t, filepath.Join(tempDir, "runtime.db"))
	migrator := NewMigrator(db, projectMigrationsDir(t))

	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate project files: %v", err)
	}

	dossierColumns := tableInfoByName(t, db, "article_dossiers")
	if _, ok := dossierColumns["processing_id"]; !ok {
		t.Fatal("missing article_dossiers.processing_id")
	}
	if _, ok := dossierColumns["content_polished_markdown"]; !ok {
		t.Fatal("missing article_dossiers.content_polished_markdown")
	}
	if _, ok := dossierColumns["dossier_prompt_version"]; !ok {
		t.Fatal("missing article_dossiers.dossier_prompt_version")
	}

	publishColumns := tableInfoByName(t, db, "article_publish_states")
	if _, ok := publishColumns["dossier_id"]; !ok {
		t.Fatal("missing article_publish_states.dossier_id")
	}
	if _, ok := publishColumns["remote_url"]; !ok {
		t.Fatal("missing article_publish_states.remote_url")
	}

	itemColumns := tableInfoByName(t, db, "daily_digest_items")
	if _, ok := itemColumns["digest_id"]; !ok {
		t.Fatal("missing daily_digest_items.digest_id")
	}
	if _, ok := itemColumns["dossier_id"]; !ok {
		t.Fatal("missing daily_digest_items.dossier_id")
	}

	processingColumns := tableInfoByName(t, db, "article_processings")
	if _, ok := processingColumns["translation_prompt_version"]; !ok {
		t.Fatal("missing article_processings.translation_prompt_version")
	}
	if _, ok := processingColumns["analysis_prompt_version"]; !ok {
		t.Fatal("missing article_processings.analysis_prompt_version")
	}
	if _, ok := processingColumns["processed_at"]; !ok {
		t.Fatal("missing article_processings.processed_at")
	}

	digestColumns := tableInfoByName(t, db, "daily_digests")
	if _, ok := digestColumns["digest_prompt_version"]; !ok {
		t.Fatal("missing daily_digests.digest_prompt_version")
	}
	if _, ok := digestColumns["llm_profile_version"]; !ok {
		t.Fatal("missing daily_digests.llm_profile_version")
	}
}

func TestMigratorAppliesAdminUsersMigration(t *testing.T) {
	tempDir := t.TempDir()
	db := openSQLiteDB(t, filepath.Join(tempDir, "runtime.db"))
	migrator := NewMigrator(db, projectMigrationsDir(t))

	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate project files: %v", err)
	}

	adminColumns := tableInfoByName(t, db, "admin_users")
	if _, ok := adminColumns["id"]; !ok {
		t.Fatal("missing admin_users.id")
	}
	if _, ok := adminColumns["username"]; !ok {
		t.Fatal("missing admin_users.username")
	}
	if _, ok := adminColumns["password_hash"]; !ok {
		t.Fatal("missing admin_users.password_hash")
	}
	if _, ok := adminColumns["must_change_password"]; !ok {
		t.Fatal("missing admin_users.must_change_password")
	}
	if _, ok := adminColumns["last_login_at"]; !ok {
		t.Fatal("missing admin_users.last_login_at")
	}
	createdAtColumn, ok := adminColumns["created_at"]
	if !ok {
		t.Fatal("missing admin_users.created_at")
	}
	if !strings.EqualFold(createdAtColumn.DefaultValue, "CURRENT_TIMESTAMP") {
		t.Fatalf("want created_at default CURRENT_TIMESTAMP got %q", createdAtColumn.DefaultValue)
	}
	updatedAtColumn, ok := adminColumns["updated_at"]
	if !ok {
		t.Fatal("missing admin_users.updated_at")
	}
	if !strings.EqualFold(updatedAtColumn.DefaultValue, "CURRENT_TIMESTAMP") {
		t.Fatalf("want updated_at default CURRENT_TIMESTAMP got %q", updatedAtColumn.DefaultValue)
	}
}

func TestMigratorBackfillsPublishedDigestStateFromLegacyRemoteURL(t *testing.T) {
	tempDir := t.TempDir()
	db := openSQLiteDB(t, filepath.Join(tempDir, "runtime.db"))

	if _, err := db.Exec(`
CREATE TABLE schema_migrations (
  filename TEXT PRIMARY KEY,
  applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_migrations (filename) VALUES ('0001_init.up.sql');
INSERT INTO schema_migrations (filename) VALUES ('0002_runtime_state.up.sql');

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

INSERT INTO daily_digests (
  id, digest_date, title, subtitle, content_markdown, content_html, remote_url
) VALUES (
  'digest-legacy', '2026-04-11', '旧日报', '旧副标题', '# legacy', '<p>legacy</p>', 'https://example.com/already-published'
);
`); err != nil {
		t.Fatalf("prepare legacy digest table: %v", err)
	}

	migrator := NewMigrator(db, projectMigrationsDir(t))
	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate legacy digest table: %v", err)
	}

	var publishState string
	if err := db.QueryRow(`SELECT publish_state FROM daily_digests WHERE id = ?`, "digest-legacy").Scan(&publishState); err != nil {
		t.Fatalf("query backfilled publish_state: %v", err)
	}
	if publishState != "published" {
		t.Fatalf("want published publish_state got %q", publishState)
	}
}

type pragmaColumn struct {
	Name         string
	Type         string
	NotNull      int
	DefaultValue string
	PrimaryKey   int
}

func openSQLiteDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func tableInfoByName(t *testing.T, db *sql.DB, table string) map[string]pragmaColumn {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("pragma table_info(%s): %v", table, err)
	}
	defer rows.Close()

	columns := make(map[string]pragmaColumn)
	for rows.Next() {
		var (
			cid        int
			column     pragmaColumn
			defaultVal sql.NullString
		)
		if err := rows.Scan(&cid, &column.Name, &column.Type, &column.NotNull, &defaultVal, &column.PrimaryKey); err != nil {
			t.Fatalf("scan pragma %s: %v", table, err)
		}
		if defaultVal.Valid {
			column.DefaultValue = defaultVal.String
		}
		columns[column.Name] = column
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma %s: %v", table, err)
	}

	return columns
}

func projectMigrationsDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
}

func writeMigrationFile(dir string, name string, contents string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644)
}

func TestWithPostgresAdvisoryLockUsesSingleSessionConnection(t *testing.T) {
	conn := &fakeAdvisoryConn{unlockResult: true}
	migrator := &Migrator{
		openAdvisoryConn: func(context.Context) (advisoryConn, error) {
			return conn, nil
		},
	}

	if err := migrator.withPostgresAdvisoryLock(context.Background(), func(context.Context) error {
		if conn.closed {
			t.Fatal("connection closed before callback finished")
		}
		conn.events = append(conn.events, "run")
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	want := []string{"lock", "run", "unlock", "close"}
	if strings.Join(conn.events, ",") != strings.Join(want, ",") {
		t.Fatalf("want events %v got %v", want, conn.events)
	}
}

func TestWithPostgresAdvisoryLockReturnsErrorWhenUnlockFails(t *testing.T) {
	migrator := &Migrator{
		openAdvisoryConn: func(context.Context) (advisoryConn, error) {
			return &fakeAdvisoryConn{unlockResult: false}, nil
		},
	}

	err := migrator.withPostgresAdvisoryLock(context.Background(), func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("want unlock error")
	}
	if !strings.Contains(err.Error(), "bootstrap advisory unlock returned false") {
		t.Fatalf("unexpected error %v", err)
	}
}

type fakeAdvisoryConn struct {
	events       []string
	closed       bool
	lockErr      error
	unlockErr    error
	unlockResult bool
}

func (c *fakeAdvisoryConn) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	if strings.Contains(query, "pg_advisory_lock") {
		c.events = append(c.events, "lock")
		return driver.RowsAffected(1), c.lockErr
	}
	return nil, errors.New("unexpected exec query")
}

func (c *fakeAdvisoryConn) QueryRowContext(_ context.Context, query string, _ ...any) rowScanner {
	if strings.Contains(query, "pg_advisory_unlock") {
		c.events = append(c.events, "unlock")
		return fakeRow{value: c.unlockResult, err: c.unlockErr}
	}
	return fakeRow{err: errors.New("unexpected query")}
}

func (c *fakeAdvisoryConn) Close() error {
	c.closed = true
	c.events = append(c.events, "close")
	return nil
}

type fakeRow struct {
	value bool
	err   error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	ptr := dest[0].(*bool)
	*ptr = r.value
	return nil
}
