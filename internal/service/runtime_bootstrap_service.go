package service

import "context"

type Migrator interface {
	Migrate(ctx context.Context) error
}

type RuntimeBootstrapService struct {
	migrator       Migrator
	profileService *ProfileService
}

func NewRuntimeBootstrapService(migrator Migrator, profileService *ProfileService) *RuntimeBootstrapService {
	return &RuntimeBootstrapService{
		migrator:       migrator,
		profileService: profileService,
	}
}

func (s *RuntimeBootstrapService) Ensure(ctx context.Context) error {
	if err := s.migrator.Migrate(ctx); err != nil {
		return err
	}

	return s.profileService.SeedDefaults(ctx)
}
