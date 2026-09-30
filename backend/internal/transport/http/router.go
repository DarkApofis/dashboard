// Package httpapi assembles the HTTP surface: it wires middlewares, routes and
// handlers. It lives in internal/transport/http but is named httpapi so it does
// not shadow the standard net/http package at every call site.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	appbusiness "paymentlinks/internal/application/business"
	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/transport/http/handler"
	"paymentlinks/internal/transport/http/middleware"
	"paymentlinks/internal/transport/http/respond"
)

// Deps is what the HTTP layer needs from the outside: use cases, never
// repositories. The transport layer must not know that persistence exists.
type Deps struct {
	Logger         *slog.Logger
	CORSOrigin     string
	Version        string
	RequestTimeout time.Duration

	// DB backs GET /health only, through a one-method interface.
	DB handler.Pinger

	PaymentReferences PaymentReferenceUseCases
	Businesses        BusinessUseCases
	Webhooks          WebhookUseCases

	// WebhookSecret enables HMAC verification on the webhook group. Empty disables
	// it; see middleware.WebhookSignature.
	WebhookSecret string
}

// PaymentReferenceUseCases groups the use cases behind /payment-references. A
// struct instead of four parameters so adding one (the webhook, the expiration job)
// does not ripple through every caller.
type PaymentReferenceUseCases struct {
	Create *apppaymentref.CreateReference
	Get    *apppaymentref.GetReference
	List   *apppaymentref.ListReferences
	Cancel *apppaymentref.CancelReference
}

// BusinessUseCases groups the use cases behind /businesses.
type BusinessUseCases struct {
	List *appbusiness.ListBusinesses
}

// WebhookUseCases groups the use cases behind /webhooks.
type WebhookUseCases struct {
	RegisterPayment *apppaymentref.RegisterPayment
}

// NewRouter builds the HTTP handler for the whole API.
func NewRouter(deps Deps) http.Handler {
	router := chi.NewRouter()

	// Order matters: the id is assigned first so the log line carries it, the
	// logger comes next so a recovered panic is logged with request context, and
	// Recover sits inside so the access log still sees the resulting 500.
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger(deps.Logger))
	router.Use(middleware.Recover)
	router.Use(middleware.CORS(deps.CORSOrigin))
	if deps.RequestTimeout > 0 {
		router.Use(chimw.Timeout(deps.RequestTimeout))
	}

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		respond.ErrorStatus(w, r, http.StatusNotFound, "route_not_found", "the requested route does not exist")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		respond.ErrorStatus(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "the method is not allowed on this route")
	})

	health := handler.NewHealth(deps.DB, deps.Version)
	router.Get("/health", health.Get)

	references := handler.NewPaymentReference(
		deps.PaymentReferences.Create,
		deps.PaymentReferences.Get,
		deps.PaymentReferences.List,
		deps.PaymentReferences.Cancel,
	)
	router.Route("/payment-references", func(r chi.Router) {
		r.Post("/", references.Create)
		r.Get("/", references.List)
		r.Get("/{id}", references.Get)
		r.Post("/{id}/cancel", references.Cancel)
	})

	businesses := handler.NewBusiness(deps.Businesses.List)
	router.Get("/businesses", businesses.List)

	// The webhook is its own group so the signature middleware applies to every
	// route in it: verification cannot be forgotten on an individual route.
	webhooks := handler.NewWebhook(deps.Webhooks.RegisterPayment)
	router.Route("/webhooks", func(r chi.Router) {
		r.Use(middleware.WebhookSignature(deps.WebhookSecret, deps.Logger))
		r.Post("/payments", webhooks.RegisterPayment)
	})

	return router
}
