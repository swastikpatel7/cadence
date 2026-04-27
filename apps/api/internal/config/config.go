// Package config loads runtime configuration from environment variables
// into a typed Config struct. Env keys are constants — no inline strings
// elsewhere in the codebase.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Env keys. Constants only.
const (
	EnvAppEnv      = "APP_ENV"
	EnvLogLevel    = "LOG_LEVEL"
	EnvPortAPI     = "PORT_API"
	EnvDatabaseURL = "DATABASE_URL"

	// Auth (Phase 2)
	EnvClerkJWKSURL       = "CLERK_JWKS_URL"
	EnvClerkIssuer        = "CLERK_ISSUER"
	EnvClerkWebhookSecret = "CLERK_WEBHOOK_SECRET"

	// Strava (Phase 3)
	EnvStravaClientID           = "STRAVA_CLIENT_ID"
	EnvStravaClientSecret       = "STRAVA_CLIENT_SECRET"
	EnvStravaCallbackURL        = "STRAVA_CALLBACK_URL"
	EnvStravaWebhookVerifyToken = "STRAVA_WEBHOOK_VERIFY_TOKEN"

	// Service URLs
	EnvWebBaseURL = "WEB_BASE_URL"

	// AI coach (Phase 9)
	EnvAnthropicAPIKey = "ANTHROPIC_API_KEY"
	EnvAnthropicModel  = "ANTHROPIC_MODEL"

	// Coach v1 model overrides (optional). If unset, fall back to the
	// per-model defaults wired in apps/api/internal/coach/anthropic.go.
	EnvAnthropicModelOpus   = "ANTHROPIC_MODEL_OPUS"
	EnvAnthropicModelSonnet = "ANTHROPIC_MODEL_SONNET"
	EnvAnthropicModelHaiku  = "ANTHROPIC_MODEL_HAIKU"

	// Crypto (used Phase 3 onward)
	EnvEncryptionKey = "ENCRYPTION_KEY"
)

// Config is the typed runtime configuration. Fields not yet relevant to
// the current phase are loaded but may be empty.
type Config struct {
	AppEnv      string
	LogLevel    string
	PortAPI     int
	DatabaseURL string

	ClerkJWKSURL       string
	ClerkIssuer        string
	ClerkWebhookSecret string

	StravaClientID           string
	StravaClientSecret       string
	StravaCallbackURL        string
	StravaWebhookVerifyToken string

	WebBaseURL string

	AnthropicAPIKey     string
	AnthropicModel      string
	AnthropicModelOpus   string
	AnthropicModelSonnet string
	AnthropicModelHaiku  string

	EncryptionKey string
}

// Load reads env vars into a Config and validates required fields.
// In hello-world (Phase 1) the only required field is PortAPI.
func Load() (*Config, error) {
	c := &Config{
		AppEnv:                   getEnv(EnvAppEnv, "development"),
		LogLevel:                 getEnv(EnvLogLevel, "debug"),
		DatabaseURL:              os.Getenv(EnvDatabaseURL),
		ClerkJWKSURL:             os.Getenv(EnvClerkJWKSURL),
		ClerkIssuer:              os.Getenv(EnvClerkIssuer),
		ClerkWebhookSecret:       os.Getenv(EnvClerkWebhookSecret),
		StravaClientID:           os.Getenv(EnvStravaClientID),
		StravaClientSecret:       os.Getenv(EnvStravaClientSecret),
		StravaCallbackURL:        os.Getenv(EnvStravaCallbackURL),
		StravaWebhookVerifyToken: os.Getenv(EnvStravaWebhookVerifyToken),
		WebBaseURL:               getEnv(EnvWebBaseURL, "http://localhost:3000"),
		AnthropicAPIKey:          os.Getenv(EnvAnthropicAPIKey),
		AnthropicModel:           getEnv(EnvAnthropicModel, "claude-sonnet-4-6"),
		AnthropicModelOpus:       os.Getenv(EnvAnthropicModelOpus),
		AnthropicModelSonnet:     os.Getenv(EnvAnthropicModelSonnet),
		AnthropicModelHaiku:      os.Getenv(EnvAnthropicModelHaiku),
		EncryptionKey:            os.Getenv(EnvEncryptionKey),
	}

	portStr := getEnv(EnvPortAPI, "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("config: invalid %s=%q: %w", EnvPortAPI, portStr, err)
	}
	c.PortAPI = port

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks the invariants we want guaranteed at startup.
// DB is required (the API can't run without it). Other fields stay
// optional until their phase wires them in.
func (c *Config) Validate() error {
	if c.PortAPI <= 0 || c.PortAPI > 65535 {
		return errors.New("config: PORT_API out of range")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: %s is required", EnvDatabaseURL)
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
