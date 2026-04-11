package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const bootstrapAdvisoryLockID int64 = 676344581687673420

var bootstrapFallbackLocks sync.Map

type Migrator struct {
	db               *sql.DB
	migrationsDir    string
	openAdvisoryConn func(ctx context.Context) (advisoryConn, error)
}

func NewMigrator(db *sql.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
		openAdvisoryConn: func(ctx context.Context) (advisoryConn, error) {
			conn, err := db.Conn(ctx)
			if err != nil {
				return nil, err
			}
			return &sqlAdvisoryConn{conn: conn}, nil
		},
	}
}

func (m *Migrator) WithLock(ctx context.Context, run func(context.Context) error) error {
	err := m.withPostgresAdvisoryLock(ctx, run)
	if err == nil {
		return nil
	}
	if !isAdvisoryLockUnsupported(err) {
		return err
	}

	return m.withProcessLocalLock(ctx, run)
}

func (m *Migrator) Migrate(ctx context.Context) error {
	if err := m.ensureSchemaMigrationsTable(ctx); err != nil {
		return err
	}

	applied, err := m.loadAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	files, err := m.listMigrationFiles()
	if err != nil {
		return err
	}

	for _, name := range files {
		if applied[name] {
			continue
		}
		if err := m.applyMigration(ctx, name); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) withPostgresAdvisoryLock(ctx context.Context, run func(context.Context) error) error {
	conn, err := m.openAdvisoryConn(ctx)
	if err != nil {
		return fmt.Errorf("open bootstrap advisory connection: %w", err)
	}

	lockSQL := fmt.Sprintf("SELECT pg_advisory_lock(%d)", bootstrapAdvisoryLockID)
	if _, err := conn.ExecContext(ctx, lockSQL); err != nil {
		_ = conn.Close()
		return fmt.Errorf("acquire bootstrap advisory lock: %w", err)
	}

	runErr := run(ctx)

	unlockSQL := fmt.Sprintf("SELECT pg_advisory_unlock(%d)", bootstrapAdvisoryLockID)
	var unlocked bool
	if err := conn.QueryRowContext(ctx, unlockSQL).Scan(&unlocked); err != nil {
		unlockErr := fmt.Errorf("release bootstrap advisory lock: %w", err)
		if closeErr := conn.Close(); closeErr != nil {
			unlockErr = errors.Join(unlockErr, fmt.Errorf("close bootstrap advisory connection: %w", closeErr))
		}
		if runErr != nil {
			return errors.Join(runErr, unlockErr)
		}
		return unlockErr
	}
	if !unlocked {
		unlockErr := errors.New("bootstrap advisory unlock returned false")
		if closeErr := conn.Close(); closeErr != nil {
			unlockErr = errors.Join(unlockErr, fmt.Errorf("close bootstrap advisory connection: %w", closeErr))
		}
		if runErr != nil {
			return errors.Join(runErr, unlockErr)
		}
		return unlockErr
	}
	if err := conn.Close(); err != nil {
		closeErr := fmt.Errorf("close bootstrap advisory connection: %w", err)
		if runErr != nil {
			return errors.Join(runErr, closeErr)
		}
		return closeErr
	}

	return runErr
}

func (m *Migrator) withProcessLocalLock(ctx context.Context, run func(context.Context) error) error {
	_ = ctx

	key := filepath.Clean(m.migrationsDir)
	lock, _ := bootstrapFallbackLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	mutex.Lock()
	defer mutex.Unlock()

	return run(ctx)
}

func (m *Migrator) listMigrationFiles() ([]string, error) {
	entries, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			files = append(files, name)
		}
	}

	sort.Strings(files)
	return files, nil
}

func (m *Migrator) ensureSchemaMigrationsTable(ctx context.Context) error {
	const query = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  filename TEXT PRIMARY KEY,
  applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

	if _, err := m.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	return nil
}

func isAdvisoryLockUnsupported(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such function: pg_advisory_lock")
}

type rowScanner interface {
	Scan(dest ...any) error
}

type advisoryConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) rowScanner
	Close() error
}

type sqlAdvisoryConn struct {
	conn *sql.Conn
}

func (c *sqlAdvisoryConn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.conn.ExecContext(ctx, query, args...)
}

func (c *sqlAdvisoryConn) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	return c.conn.QueryRowContext(ctx, query, args...)
}

func (c *sqlAdvisoryConn) Close() error {
	return c.conn.Close()
}

func (m *Migrator) loadAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func (m *Migrator) applyMigration(ctx context.Context, name string) error {
	content, err := os.ReadFile(filepath.Join(m.migrationsDir, name))
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}
	defer tx.Rollback()

	if statement := strings.TrimSpace(string(content)); statement != "" {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}

	insert := fmt.Sprintf(
		"INSERT INTO schema_migrations (filename) VALUES ('%s')",
		strings.ReplaceAll(name, "'", "''"),
	)
	if _, err := tx.ExecContext(ctx, insert); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	return nil
}
