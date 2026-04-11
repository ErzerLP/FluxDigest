package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"rss-platform/internal/repository/postgres"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigratorExecutesUpMigrationsInOrderAndSkipsAppliedFiles(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "runtime.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

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

	migrator := postgres.NewMigrator(db, tempDir)
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

func writeMigrationFile(dir string, name string, contents string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644)
}
