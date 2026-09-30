package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// ServerConfig configures the HTTP server. The timeouts are explicit because
// net/http defaults to none, which is how a single stuck client ends up holding a
// connection forever.
type ServerConfig struct {
	Addr            string
	Handler         http.Handler
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Server owns the listener lifecycle.
type Server struct {
	http            *http.Server
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

// NewServer builds the server with sane defaults for anything left unset.
func NewServer(cfg ServerConfig, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 15 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 20 * time.Second
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 60 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}

	return &Server{
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           cfg.Handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		shutdownTimeout: cfg.ShutdownTimeout,
		logger:          logger,
	}
}

// Run serves until ctx is cancelled, then drains in-flight requests within the
// shutdown timeout. Returning only on a real failure keeps main trivial.
func (s *Server) Run(ctx context.Context) error {
	serveErr := make(chan error, 1)

	go func() {
		s.logger.Info("http server listening", slog.String("addr", s.http.Addr))
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		s.logger.Info("shutdown signal received, draining connections",
			slog.Duration("timeout", s.shutdownTimeout),
		)

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.shutdownTimeout)
		defer cancel()

		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return err
		}
		s.logger.Info("http server stopped")
		return nil
	}
}
