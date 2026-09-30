// Command api is the HTTP entry point of the platform.
//
// This file is the only place where the layers are stitched together: it reads the
// configuration, opens the database, builds the infrastructure adapters, hands
// them to the use cases and hands those to the HTTP layer. Dependency injection is
// manual and explicit on purpose. There is no container and no reflection, so the
// whole dependency graph of the process is readable top to bottom in one function.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	appbusiness "paymentlinks/internal/application/business"
	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/config"
	"paymentlinks/internal/infrastructure/clock"
	"paymentlinks/internal/infrastructure/postgres"
	httpapi "paymentlinks/internal/transport/http"
)

// version is overridable at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal error, shutting down", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	// A SIGINT/SIGTERM cancels this context, which unwinds the whole process:
	// the server drains, the pool closes.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting api",
		slog.String("version", version),
		slog.String("addr", cfg.Addr()),
		slog.Bool("webhook_signature_enabled", cfg.WebhookSignatureEnabled()),
	)

	// --- Infrastructure -----------------------------------------------------
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{
		URL:            cfg.DatabaseURL,
		MaxConns:       cfg.DBMaxConns,
		ConnectRetries: cfg.DBConnectRetry,
		RetryDelay:     time.Second,
	}, logger)
	if err != nil {
		return err
	}
	defer pool.Close()

	txManager := postgres.NewTxManager(pool)
	paymentReferences := postgres.NewPaymentReferenceRepository(pool)
	businesses := postgres.NewBusinessRepository(pool)
	webhookEvents := postgres.NewWebhookEventRepository(pool)
	systemClock := clock.New()

	// --- Application --------------------------------------------------------
	// Each use case receives only the ports it needs, which is what makes them
	// testable with in-memory fakes (see internal/infrastructure/inmemory).
	createReference := apppaymentref.NewCreateReference(paymentReferences, businesses, systemClock)
	getReference := apppaymentref.NewGetReference(paymentReferences, systemClock)
	listReferences := apppaymentref.NewListReferences(paymentReferences, systemClock)
	cancelReference := apppaymentref.NewCancelReference(txManager, paymentReferences, systemClock)
	registerPayment := apppaymentref.NewRegisterPayment(txManager, paymentReferences, webhookEvents, systemClock)
	listBusinesses := appbusiness.NewListBusinesses(businesses)

	// --- Transport ----------------------------------------------------------
	router := httpapi.NewRouter(httpapi.Deps{
		Logger:         logger,
		CORSOrigin:     cfg.CORSOrigin,
		Version:        version,
		RequestTimeout: cfg.RequestTimeout,
		DB:             pool,
		PaymentReferences: httpapi.PaymentReferenceUseCases{
			Create: createReference,
			Get:    getReference,
			List:   listReferences,
			Cancel: cancelReference,
		},
		Businesses:    httpapi.BusinessUseCases{List: listBusinesses},
		Webhooks:      httpapi.WebhookUseCases{RegisterPayment: registerPayment},
		WebhookSecret: cfg.WebhookSecret,
	})

	server := httpapi.NewServer(httpapi.ServerConfig{
		Addr:            cfg.Addr(),
		Handler:         router,
		ShutdownTimeout: cfg.ShutdownTimeout,
	}, logger)

	return server.Run(ctx)
}

// newLogger builds the process logger. Structured JSON by default so log lines are
// queryable; LOG_FORMAT=text is there for a readable local terminal.
func newLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}

	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler).With(slog.String("service", "payment-links-api"))
}
