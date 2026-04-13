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
	seeders  []DefaultsSeeder
}

func NewRuntimeBootstrapService(migrator BootstrapMigrator, seeders ...DefaultsSeeder) *RuntimeBootstrapService {
	return &RuntimeBootstrapService{
		migrator: migrator,
		seeders:  seeders,
	}
}

func (s *RuntimeBootstrapService) Ensure(ctx context.Context) error {
	return s.migrator.WithLock(ctx, func(ctx context.Context) error {
		if err := s.migrator.Migrate(ctx); err != nil {
			return err
		}

		for _, seeder := range s.seeders {
			if seeder == nil {
				continue
			}
			if err := seeder.SeedDefaults(ctx); err != nil {
				return err
			}
		}

		return nil
	})
}
