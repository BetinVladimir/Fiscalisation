package main

import (
	"testing"

	"fiscalisation/fiscal-backend/internal/config"
)

func TestMainRefusesMemoryRepoOutsideDev(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		dbURL   string
		baseURL string
		wantErr bool
	}{
		{"dev+local+no-db: ok", "dev", "", "http://localhost:8080/public/v1", false},
		{"test+no-db: ok", "test", "", "http://localhost:8080/public/v1", false},
		{"prod+no-db: error", "prod", "", "http://localhost:8080/public/v1", true},
		{"staging+no-db: error", "staging", "", "http://localhost:8080/public/v1", true},
		{"dev+remote-url+no-db: error", "dev", "", "https://pay.example.com/public/v1", true},
		{"dev+remote-url+db-set: ok", "dev", "postgres://localhost/test", "https://pay.example.com/public/v1", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Config{
				AppEnv:        tc.env,
				DatabaseURL:   tc.dbURL,
				PublicBaseURL: tc.baseURL,
			}
			// Test only the guard logic (no I/O)
			err := requiresDatabaseURL(cfg)
			if (err != nil) != tc.wantErr {
				t.Errorf("requiresDatabaseURL(%q,db=%q,url=%q) error = %v, wantErr = %v",
					tc.env, tc.dbURL, tc.baseURL, err, tc.wantErr)
			}
		})
	}
}

func TestIsLocalURL(t *testing.T) {
	locals := []string{
		"http://localhost:8080/public/v1",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
	}
	remotes := []string{
		"https://pay.example.com/public/v1",
		"http://192.168.1.1:8080",
	}
	for _, u := range locals {
		if !isLocalURL(u) {
			t.Errorf("isLocalURL(%q) = false, want true", u)
		}
	}
	for _, u := range remotes {
		if isLocalURL(u) {
			t.Errorf("isLocalURL(%q) = true, want false", u)
		}
	}
}
