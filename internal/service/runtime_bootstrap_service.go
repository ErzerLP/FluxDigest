package service

import "context"

type BootstrapMigrator interface {
	WithLock(ctx context.Context, run func(context.Context) error) error
	Migrate(ctx context.Context) error
}

type DefaultsSeeder interface {
	SeedDefaults(ctx context.Context) error
}

type RuntimeBootstrapService struct {
	migrator BootstrapMigrator
	seeder   DefaultsSeeder
}

func NewRuntimeBootstrapService(migrator BootstrapMigrator, seeder DefaultsSeeder) *RuntimeBootstrapService {
	return &RuntimeBootstrapService{
		migrator: migrator,
		seeder:   seeder,
	}
}

func (s *RuntimeBootstrapService) Ensure(ctx context.Context) error {
	return s.migrator.WithLock(ctx, func(ctx context.Context) error {
		if err := s.migrator.Migrate(ctx); err != nil {
			return err
		}

		return s.seeder.SeedDefaults(ctx)
	})
}
