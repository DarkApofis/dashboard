// Package handler holds the HTTP handlers. A handler decodes the request, calls a
// use case and serialises the result: it never talks to the database and never
// contains a business rule.
package handler

import (
	"context"
	"net/http"
	"time"

	"paymentlinks/internal/transport/http/dto"
	"paymentlinks/internal/transport/http/respond"
)

// Pinger is the liveness check the health handler needs. Declaring it here, as a
// one-method interface, keeps the handler unaware of pgx: *pgxpool.Pool satisfies
// it, and so does a fake in a test.
type Pinger interface {
	Ping(ctx context.Context) error
}

const healthCheckTimeout = 2 * time.Second

// Health serves GET /health.
type Health struct {
	db      Pinger
	version string
	started time.Time
}

// NewHealth builds the handler.
func NewHealth(db Pinger, version string) *Health {
	return &Health{db: db, version: version, started: time.Now()}
}

// Get reports whether the process can serve traffic. It answers 503 when the
// database is unreachable, because an API that cannot reach PostgreSQL cannot
// honour a single business request, and an orchestrator should know that.
func (h *Health) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	status := http.StatusOK
	body := dto.HealthResponse{
		Status:  "ok",
		Version: h.version,
		Uptime:  time.Since(h.started).Round(time.Second).String(),
		Checks:  map[string]string{"database": "ok"},
	}

	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			status = http.StatusServiceUnavailable
			body.Status = "degraded"
			body.Checks["database"] = "unavailable"
		}
	}

	respond.JSON(w, r, status, body)
}
