// Package postgres implements the application ports on top of PostgreSQL using
// pgx. It is the only package that knows SQL, and it is responsible for
// translating driver errors into domain errors so that no layer above it ever
// sees a *pgconn.PgError.
package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds the connection settings, kept free of our config package so
// this layer stays reusable.
type PoolConfig struct {
	URL            string
	MaxConns       int32
	ConnectRetries int
	RetryDelay     time.Duration
}

// NewPool opens the connection pool and waits until the database answers.
//
// The retry loop exists because the API and PostgreSQL usually start together
// (docker compose, a fresh local install): failing on the first refused
// connection would just make the process crash-loop for a few seconds.
func NewPool(ctx context.Context, cfg PoolConfig, logger *slog.Logger) (*pgxpool.Pool, error) {
	if logger == nil {
		logger = slog.Default()
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	retries := cfg.ConnectRetries
	if retries <= 0 {
		retries = 1
	}
	delay := cfg.RetryDelay
	if delay <= 0 {
		delay = time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = pool.Ping(pingCtx)
		cancel()
		if lastErr == nil {
			logger.Info("database connection established", slog.Int("attempt", attempt))
			return pool, nil
		}
		logger.Warn("database not ready yet",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", retries),
			slog.String("error", lastErr.Error()),
		)
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	pool.Close()
	return nil, fmt.Errorf("database unreachable after %d attempts: %w", retries, lastErr)
}
