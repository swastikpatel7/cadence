// Package config loads worker runtime configuration from environment
// variables. Mirrors apps/api's config but only carries the keys the
// worker actually uses.
package config

import (
	"fmt"
	"os"
)

const (
	EnvAppEnv      = "APP_ENV"
	EnvLogLevel    = "LOG_LEVEL"
	EnvDatabaseURL = "DATABASE_URL"
	EnvRedisURL    = "REDIS_URL"

	EnvStravaClientID     = "STRAVA_CLIENT_ID"
	EnvStravaClientSecret = "STRAVA_CLIENT_SECRET"

	EnvAnthropicAPIKey = "ANTHROPIC_API_KEY"
	EnvEncryptionKey   = "ENCRYPTION_KEY"
)

type Config struct {
	AppEnv      string
	LogLevel    string
	DatabaseURL string
	RedisURL    string

	StravaClientID     string
	StravaClientSecret string

	AnthropicAPIKey string
	EncryptionKey   string
}

func Load() (*Config, error) {
	c := &Config{
		AppEnv:             getEnv(EnvAppEnv, "development"),
		LogLevel:           getEnv(EnvLogLevel, "debug"),
		DatabaseURL:        os.Getenv(EnvDatabaseURL),
		RedisURL:           os.Getenv(EnvRedisURL),
		StravaClientID:     os.Getenv(EnvStravaClientID),
		StravaClientSecret: os.Getenv(EnvStravaClientSecret),
		AnthropicAPIKey:    os.Getenv(EnvAnthropicAPIKey),
		EncryptionKey:      os.Getenv(EnvEncryptionKey),
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate enforces the invariants the worker needs at startup.
// DATABASE_URL is required because River cannot run without Postgres.
// ENCRYPTION_KEY, STRAVA_CLIENT_ID, and STRAVA_CLIENT_SECRET are required
// because the Strava sync worker needs them to decrypt tokens and refresh
// against Strava — failing fast at startup is preferable to crashing on
// the first job.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: %s is required", EnvDatabaseURL)
	}
	if c.EncryptionKey == "" {
		return fmt.Errorf("config: %s is required", EnvEncryptionKey)
	}
	if c.StravaClientID == "" {
		return fmt.Errorf("config: %s is required", EnvStravaClientID)
	}
	if c.StravaClientSecret == "" {
		return fmt.Errorf("config: %s is required", EnvStravaClientSecret)
	}
	return nil
}

// IsDevelopment reports whether AppEnv is "development".
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
