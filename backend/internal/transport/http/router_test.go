package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"paymentlinks/internal/transport/http/dto"
	"paymentlinks/internal/transport/http/handler"
	"paymentlinks/internal/transport/http/middleware"
)

// stubPinger stands in for the database in the health check.
type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

func newTestRouter(pinger handler.Pinger) http.Handler {
	return NewRouter(Deps{
		// Discard logs: the assertions are about the HTTP contract.
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		CORSOrigin: "http://localhost:3000",
		Version:    "test",
		DB:         pinger,
	})
}

func TestHealthOK(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	newTestRouter(stubPinger{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
	}

	var body dto.HealthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" || body.Checks["database"] != "ok" {
		t.Errorf("unexpected body: %+v", body)
	}
	if recorder.Header().Get(middleware.HeaderRequestID) == "" {
		t.Error("every response must carry a request id header")
	}
}

func TestHealthReportsUnavailableDatabase(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	newTestRouter(stubPinger{err: errors.New("connection refused")}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", recorder.Code)
	}

	var body dto.HealthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Checks["database"] != "unavailable" {
		t.Errorf("database check: got %q", body.Checks["database"])
	}
}

func TestRequestIDIsReusedWhenProvided(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(middleware.HeaderRequestID, "trace-123")

	newTestRouter(stubPinger{}).ServeHTTP(recorder, request)

	if got := recorder.Header().Get(middleware.HeaderRequestID); got != "trace-123" {
		t.Errorf("request id: got %q, want trace-123", got)
	}
}

// Unknown routes must answer in the same envelope as everything else, so a client
// only ever has to parse one error shape.
func TestUnknownRouteUsesErrorEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/nope", nil)

	newTestRouter(stubPinger{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", recorder.Code)
	}

	var body dto.ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "route_not_found" || body.Error.Message == "" {
		t.Errorf("unexpected error body: %+v", body.Error)
	}
}

func TestCORSPreflight(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/health", nil)
	request.Header.Set("Origin", "http://localhost:3000")

	newTestRouter(stubPinger{}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("allow origin: got %q", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("Origin", "http://evil.example")

	newTestRouter(stubPinger{}).ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("an unconfigured origin must not be allowed, got %q", got)
	}
}
