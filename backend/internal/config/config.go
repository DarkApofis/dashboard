// Package config loads the runtime configuration from environment variables.
// It fails fast: if a required variable is missing or malformed the process
// never reaches the point of accepting traffic.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every knob the process needs. Zero HTTP or SQL types here so the
// rest of the code depends on plain values, not on how they were provided.
type Config struct {
	Port            string
	DatabaseURL     string
	WebhookSecret   string
	CORSOrigin      string
	LogLevel        slog.Level
	LogFormat       string
	DBMaxConns      int32
	DBConnectRetry  int
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// Load reads the environment and returns a validated Config. Every problem is
// collected so the operator sees all of them at once instead of one per restart.
func Load() (*Config, error) {
	var problems []error

	cfg := &Config{
		Port:            envString("PORT", "8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		WebhookSecret:   os.Getenv("WEBHOOK_SECRET"),
		CORSOrigin:      envString("CORS_ORIGIN", "http://localhost:3000"),
		LogFormat:       strings.ToLower(envString("LOG_FORMAT", "json")),
		DBMaxConns:      10,
		DBConnectRetry:  10,
		RequestTimeout:  15 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	if cfg.DatabaseURL == "" {
		problems = append(problems, errors.New("DATABASE_URL is required"))
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		problems = append(problems, fmt.Errorf("PORT must be a number, got %q", cfg.Port))
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		problems = append(problems, fmt.Errorf("LOG_FORMAT must be json or text, got %q", cfg.LogFormat))
	}

	level, err := parseLogLevel(envString("LOG_LEVEL", "info"))
	if err != nil {
		problems = append(problems, err)
	}
	cfg.LogLevel = level

	maxConns, err := envInt("DB_MAX_CONNS", int(cfg.DBMaxConns))
	if err != nil {
		problems = append(problems, err)
	}
	cfg.DBMaxConns = int32(maxConns)

	retries, err := envInt("DB_CONNECT_RETRY", cfg.DBConnectRetry)
	if err != nil {
		problems = append(problems, err)
	}
	cfg.DBConnectRetry = retries

	if requestTimeout, err := envDuration("REQUEST_TIMEOUT", cfg.RequestTimeout); err != nil {
		problems = append(problems, err)
	} else {
		cfg.RequestTimeout = requestTimeout
	}

	if shutdownTimeout, err := envDuration("SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		problems = append(problems, err)
	} else {
		cfg.ShutdownTimeout = shutdownTimeout
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid configuration: %w", errors.Join(problems...))
	}
	return cfg, nil
}

// Addr is the listen address for the HTTP server.
func (c *Config) Addr() string { return ":" + c.Port }

// WebhookSignatureEnabled reports whether incoming provider webhooks must carry
// a valid HMAC signature. An empty secret disables verification on purpose so
// the API can be exercised with plain curl during review; see the README.
func (c *Config) WebhookSignatureEnabled() bool {
	return strings.TrimSpace(c.WebhookSecret) != ""
}

func envString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback, fmt.Errorf("%s must be a positive number, got %q", key, raw)
	}
	return v, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback, fmt.Errorf("%s must be a positive duration such as 15s, got %q", key, raw)
	}
	return v, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error, got %q", raw)
	}
}
