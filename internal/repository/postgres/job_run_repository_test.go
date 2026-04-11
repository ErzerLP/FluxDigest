package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"rss-platform/internal/repository/postgres"
	"rss-platform/internal/repository/postgres/models"
	"rss-platform/internal/service"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newJobRunTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s-jobruns?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func TestJobRunRepositoryCreateAndListLatest(t *testing.T) {
	db := newJobRunTestDB(t)
	if err := db.AutoMigrate(&models.JobRunModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := postgres.NewJobRunRepository(db)

	if err := repo.Create(context.Background(), service.JobRunRecord{JobType: "daily_digest_run", Status: "succeeded", DigestDate: "2026-04-11"}); err != nil {
		t.Fatal(err)
	}
	runs, err := repo.ListLatest(context.Background(), service.JobRunListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].JobType != "daily_digest_run" {
		t.Fatalf("unexpected runs %#v", runs)
	}
	if runs[0].ID == "" {
		t.Fatalf("expected generated id, got empty")
	}
}

func TestJobRunRepositoryCreatePreservesID(t *testing.T) {
	db := newJobRunTestDB(t)
	if err := db.AutoMigrate(&models.JobRunModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := postgres.NewJobRunRepository(db)

	input := service.JobRunRecord{ID: "run-123", JobType: "llm_test", Status: "ok"}
	if err := repo.Create(context.Background(), input); err != nil {
		t.Fatal(err)
	}

	runs, err := repo.ListLatest(context.Background(), service.JobRunListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != "run-123" {
		t.Fatalf("unexpected runs %#v", runs)
	}
}
