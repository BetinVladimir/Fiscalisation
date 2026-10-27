package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
	"fiscalisation/fiscal-backend/internal/persistence"
	"fiscalisation/fiscal-backend/internal/startup"
)

// requiresDatabaseURL returns an error if the configuration demands a database
// URL but none is provided.  Called before any I/O so it can be unit-tested.
func requiresDatabaseURL(cfg config.Config) error {
	needsDB := cfg.AppEnv != "dev" && cfg.AppEnv != "test"
	devWithRemoteURL := cfg.AppEnv == "dev" && !isLocalURL(cfg.PublicBaseURL)
	if (needsDB || devWithRemoteURL) && cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL required outside dev/test (or for dev with remote PublicBaseURL)")
	}
	return nil
}

// selectRepository chooses between MemoryRepository and PersistentRepository
// based on configuration. Fails fast when a non-dev/test environment has no
// DatabaseURL, or when a dev environment exposes a non-local PublicBaseURL
// without a database (which would indicate a staging-like deployment using
// the unsafe memory store).
func selectRepository(cfg config.Config) (domain.Repository, func(), error) {
	noop := func() {}
	if err := requiresDatabaseURL(cfg); err != nil {
		return nil, noop, err
	}

	if cfg.DatabaseURL == "" {
		return domain.NewMemoryRepository(), noop, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := persistence.RunMigrations(ctx, cfg.DatabaseURL); err != nil {
		cancel()
		return nil, noop, err
	}
	store, err := startup.Retry(ctx, 500*time.Millisecond, func() (*persistence.Postgres, error) {
		return persistence.OpenWithReader(cfg.DatabaseURL, cfg.RLSDatabaseURL)
	})
	if err != nil {
		cancel()
		return nil, noop, err
	}
	repo, err := domain.NewPersistentRepository(store)
	if err != nil {
		cancel()
		store.Close()
		return nil, noop, err
	}
	return repo, func() { cancel(); store.Close() }, nil
}

func isLocalURL(raw string) bool {
	return strings.HasPrefix(raw, "http://localhost") ||
		strings.HasPrefix(raw, "http://127.") ||
		strings.HasPrefix(raw, "http://[::1]")
}
