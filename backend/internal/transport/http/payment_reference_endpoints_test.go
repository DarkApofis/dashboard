package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	appbusiness "paymentlinks/internal/application/business"
	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/infrastructure/clock"
	"paymentlinks/internal/infrastructure/inmemory"
	httpapi "paymentlinks/internal/transport/http"
	"paymentlinks/internal/transport/http/dto"
	"paymentlinks/internal/transport/http/middleware"
)

var reference = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// api is the whole HTTP surface wired over in-memory ports: the assertions below
// are about the contract a client sees, status codes included.
type api struct {
	handler    http.Handler
	clock      *clock.Fixed
	events     *inmemory.WebhookEventRepository
	businessID int64
}

func newAPI(t *testing.T) *api { return newAPIWithSecret(t, "") }

// newAPIWithSecret builds the API with HMAC verification enabled on the webhook.
func newAPIWithSecret(t *testing.T, webhookSecret string) *api {
	t.Helper()

	references := inmemory.NewPaymentReferenceRepository()
	businesses := inmemory.NewBusinessRepository()
	events := inmemory.NewWebhookEventRepository()
	tx := inmemory.NewTxManager()
	fixed := clock.NewFixed(reference)
	ids := businesses.Seed("Acme Logistics", "Panaderia La Esquina")

	handler := httpapi.NewRouter(httpapi.Deps{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		CORSOrigin: "http://localhost:3000",
		Version:    "test",
		PaymentReferences: httpapi.PaymentReferenceUseCases{
			Create: apppaymentref.NewCreateReference(references, businesses, fixed),
			Get:    apppaymentref.NewGetReference(references, fixed),
			List:   apppaymentref.NewListReferences(references, fixed),
			Cancel: apppaymentref.NewCancelReference(tx, references, fixed),
		},
		Businesses:    httpapi.BusinessUseCases{List: appbusiness.NewListBusinesses(businesses)},
		Webhooks:      httpapi.WebhookUseCases{RegisterPayment: apppaymentref.NewRegisterPayment(tx, references, events, fixed)},
		WebhookSecret: webhookSecret,
	})

	return &api{handler: handler, clock: fixed, events: events, businessID: ids[0]}
}

// doSigned sends a request with an explicit signature header.
func (a *api) doSigned(t *testing.T, method, target, body, signature string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if signature != "" {
		request.Header.Set(middleware.HeaderSignature, signature)
	}

	recorder := httptest.NewRecorder()
	a.handler.ServeHTTP(recorder, request)
	return recorder
}

func (a *api) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	a.handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeReference(t *testing.T, recorder *httptest.ResponseRecorder) dto.PaymentReferenceResponse {
	t.Helper()

	var body dto.PaymentReferenceResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode reference: %v (body %s)", err, recorder.Body.String())
	}
	return body
}

func errorCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var body dto.ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope: %v (body %s)", err, recorder.Body.String())
	}
	if body.Error.Message == "" {
		t.Error("an error response must carry a message")
	}
	return body.Error.Code
}

func (a *api) createBody(externalReference string) string {
	return `{"business_id":` + itoa(a.businessID) +
		`,"external_reference":"` + externalReference +
		`","amount":125000.50,"description":"Factura","expires_at":"2026-10-01T23:59:59Z"}`
}

func itoa(value int64) string { return strconv.FormatInt(value, 10) }

func TestCreatePaymentReference(t *testing.T) {
	t.Run("201 with the created resource", func(t *testing.T) {
		a := newAPI(t)

		recorder := a.do(t, http.MethodPost, "/payment-references", a.createBody("INV-10023"))
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status: got %d, want 201 (body %s)", recorder.Code, recorder.Body.String())
		}

		body := decodeReference(t, recorder)
		if body.ID == 0 || body.Status != "pending" {
			t.Errorf("unexpected body: %+v", body)
		}
		// The amount must survive the round trip exactly, as a JSON number.
		if !strings.Contains(recorder.Body.String(), `"amount":125000.50`) {
			t.Errorf("amount must serialise as 125000.50, got %s", recorder.Body.String())
		}
		if got := recorder.Header().Get("Location"); got == "" {
			t.Error("a created resource must carry a Location header")
		}
	})

	t.Run("409 on a duplicate external reference", func(t *testing.T) {
		a := newAPI(t)
		if code := a.do(t, http.MethodPost, "/payment-references", a.createBody("INV-10023")).Code; code != http.StatusCreated {
			t.Fatalf("first create: got %d", code)
		}

		recorder := a.do(t, http.MethodPost, "/payment-references", a.createBody("INV-10023"))
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status: got %d, want 409", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "duplicate_external_reference" {
			t.Errorf("error code: got %q", code)
		}
	})

	t.Run("422 on an unknown business", func(t *testing.T) {
		a := newAPI(t)
		body := `{"business_id":9999,"external_reference":"INV-1","amount":10.00,"expires_at":"2026-10-01T23:59:59Z"}`

		recorder := a.do(t, http.MethodPost, "/payment-references", body)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status: got %d, want 422", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "business_not_found" {
			t.Errorf("error code: got %q", code)
		}
	})

	t.Run("422 on a non positive amount", func(t *testing.T) {
		a := newAPI(t)
		body := `{"business_id":` + itoa(a.businessID) + `,"external_reference":"INV-1","amount":0,"expires_at":"2026-10-01T23:59:59Z"}`

		recorder := a.do(t, http.MethodPost, "/payment-references", body)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status: got %d, want 422 (body %s)", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("422 on an expiration in the past", func(t *testing.T) {
		a := newAPI(t)
		body := `{"business_id":` + itoa(a.businessID) + `,"external_reference":"INV-1","amount":10.00,"expires_at":"2020-01-01T00:00:00Z"}`

		recorder := a.do(t, http.MethodPost, "/payment-references", body)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status: got %d, want 422", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "invalid_expires_at" {
			t.Errorf("error code: got %q", code)
		}
	})

	t.Run("422 on an amount with more than two decimals", func(t *testing.T) {
		a := newAPI(t)
		body := `{"business_id":` + itoa(a.businessID) + `,"external_reference":"INV-1","amount":10.005,"expires_at":"2026-10-01T23:59:59Z"}`

		recorder := a.do(t, http.MethodPost, "/payment-references", body)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status: got %d, want 422 (body %s)", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("400 on a malformed body", func(t *testing.T) {
		a := newAPI(t)

		recorder := a.do(t, http.MethodPost, "/payment-references", `{"business_id":`)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400", recorder.Code)
		}
	})

	t.Run("400 on an unknown field", func(t *testing.T) {
		a := newAPI(t)
		body := `{"business_id":` + itoa(a.businessID) + `,"external_reference":"INV-1","amount":10.00,"expires_at":"2026-10-01T23:59:59Z","amount_cents":1000}`

		recorder := a.do(t, http.MethodPost, "/payment-references", body)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "unknown_field" {
			t.Errorf("error code: got %q", code)
		}
	})

	t.Run("400 on an empty body", func(t *testing.T) {
		a := newAPI(t)

		recorder := a.do(t, http.MethodPost, "/payment-references", "")
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400", recorder.Code)
		}
	})
}

func TestListPaymentReferences(t *testing.T) {
	a := newAPI(t)
	for _, external := range []string{"INV-1", "INV-2"} {
		if code := a.do(t, http.MethodPost, "/payment-references", a.createBody(external)).Code; code != http.StatusCreated {
			t.Fatalf("create %s: got %d", external, code)
		}
	}

	t.Run("200 with items and total", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", recorder.Code)
		}

		var body dto.PaymentReferenceListResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Total != 2 || len(body.Items) != 2 {
			t.Errorf("got total=%d items=%d, want 2 and 2", body.Total, len(body.Items))
		}
		if body.Limit != apppaymentref.DefaultLimit {
			t.Errorf("limit: got %d, want the default %d", body.Limit, apppaymentref.DefaultLimit)
		}
	})

	t.Run("an empty result is an empty array, not null", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references?status=paid", "")
		if !strings.Contains(recorder.Body.String(), `"items":[]`) {
			t.Errorf("got %s", recorder.Body.String())
		}
	})

	t.Run("400 on an unknown status filter", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references?status=refunded", "")
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "invalid_status" {
			t.Errorf("error code: got %q", code)
		}
	})

	t.Run("400 on a non numeric limit", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references?limit=many", "")
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400", recorder.Code)
		}
	})

	t.Run("filters by external reference", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references?external_reference=INV-2", "")

		var body dto.PaymentReferenceListResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Total != 1 || body.Items[0].ExternalReference != "INV-2" {
			t.Errorf("unexpected body: %+v", body)
		}
	})
}

func TestGetPaymentReference(t *testing.T) {
	a := newAPI(t)
	created := decodeReference(t, a.do(t, http.MethodPost, "/payment-references", a.createBody("INV-1")))

	t.Run("200 with the resource", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references/"+itoa(created.ID), "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", recorder.Code)
		}
	})

	t.Run("404 when it does not exist", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references/4242", "")
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status: got %d, want 404", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "payment_reference_not_found" {
			t.Errorf("error code: got %q", code)
		}
	})

	t.Run("400 on a non numeric id", func(t *testing.T) {
		recorder := a.do(t, http.MethodGet, "/payment-references/abc", "")
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want 400", recorder.Code)
		}
	})

	t.Run("reports expired once the date has passed", func(t *testing.T) {
		a.clock.Advance(72 * time.Hour)
		defer a.clock.Advance(-72 * time.Hour)

		body := decodeReference(t, a.do(t, http.MethodGet, "/payment-references/"+itoa(created.ID), ""))
		if body.Status != "expired" {
			t.Errorf("status: got %q, want expired", body.Status)
		}
	})
}

func TestCancelPaymentReference(t *testing.T) {
	t.Run("200 and cancelled", func(t *testing.T) {
		a := newAPI(t)
		created := decodeReference(t, a.do(t, http.MethodPost, "/payment-references", a.createBody("INV-1")))

		recorder := a.do(t, http.MethodPost, "/payment-references/"+itoa(created.ID)+"/cancel", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200 (body %s)", recorder.Code, recorder.Body.String())
		}
		if body := decodeReference(t, recorder); body.Status != "cancelled" || body.CancelledAt == nil {
			t.Errorf("unexpected body: %+v", body)
		}
	})

	t.Run("200 again when already cancelled", func(t *testing.T) {
		a := newAPI(t)
		created := decodeReference(t, a.do(t, http.MethodPost, "/payment-references", a.createBody("INV-1")))
		path := "/payment-references/" + itoa(created.ID) + "/cancel"
		if code := a.do(t, http.MethodPost, path, "").Code; code != http.StatusOK {
			t.Fatalf("first cancel: got %d", code)
		}

		recorder := a.do(t, http.MethodPost, path, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("second cancel: got %d, want 200 (cancelling is idempotent)", recorder.Code)
		}
	})

	t.Run("409 when expired", func(t *testing.T) {
		a := newAPI(t)
		created := decodeReference(t, a.do(t, http.MethodPost, "/payment-references", a.createBody("INV-1")))
		a.clock.Advance(72 * time.Hour)

		recorder := a.do(t, http.MethodPost, "/payment-references/"+itoa(created.ID)+"/cancel", "")
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status: got %d, want 409", recorder.Code)
		}
		if code := errorCode(t, recorder); code != "cannot_cancel_expired_reference" {
			t.Errorf("error code: got %q", code)
		}
	})

	t.Run("404 when it does not exist", func(t *testing.T) {
		a := newAPI(t)

		recorder := a.do(t, http.MethodPost, "/payment-references/4242/cancel", "")
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status: got %d, want 404", recorder.Code)
		}
	})
}

func TestListBusinesses(t *testing.T) {
	a := newAPI(t)

	recorder := a.do(t, http.MethodGet, "/businesses", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", recorder.Code)
	}

	var body dto.BusinessListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items: got %d, want 2", len(body.Items))
	}
	if body.Items[0].Name != "Acme Logistics" {
		t.Errorf("businesses must come back alphabetically, got %q first", body.Items[0].Name)
	}
}
