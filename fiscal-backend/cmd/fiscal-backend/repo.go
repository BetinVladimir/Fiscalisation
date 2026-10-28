package main

import (
	"errors"
	"strings"

	"fiscalisation/fiscal-backend/internal/config"
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

func isLocalURL(raw string) bool {
	return strings.HasPrefix(raw, "http://localhost") ||
		strings.HasPrefix(raw, "http://127.") ||
		strings.HasPrefix(raw, "http://[::1]")
}
