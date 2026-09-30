package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	apppaymentref "paymentlinks/internal/application/paymentref"
	domain "paymentlinks/internal/domain/paymentref"
	"paymentlinks/internal/transport/http/dto"
	"paymentlinks/internal/transport/http/respond"
)

// PaymentReference holds the payment reference endpoints.
//
// Every method does the same three things and nothing else: translate the request
// into use case input, call the use case, translate the result (or the error) back
// into HTTP. There is no branching on business state here, which is why the state
// machine cannot be bypassed by adding an endpoint.
type PaymentReference struct {
	create *apppaymentref.CreateReference
	get    *apppaymentref.GetReference
	list   *apppaymentref.ListReferences
	cancel *apppaymentref.CancelReference
}

// NewPaymentReference wires the handler with the use cases it exposes.
func NewPaymentReference(
	create *apppaymentref.CreateReference,
	get *apppaymentref.GetReference,
	list *apppaymentref.ListReferences,
	cancel *apppaymentref.CancelReference,
) *PaymentReference {
	return &PaymentReference{create: create, get: get, list: list, cancel: cancel}
}

// Create handles POST /payment-references.
//
// 201 on success, 400 for a body that cannot be parsed, 422 for a body that breaks
// a rule (unknown business, non-positive amount, expiration in the past) and 409
// when external_reference is already taken.
func (h *PaymentReference) Create(w http.ResponseWriter, r *http.Request) {
	var request dto.CreatePaymentReferenceRequest
	if err := decodeJSON(r, &request); err != nil {
		respond.Error(w, r, err)
		return
	}

	result, err := h.create.Execute(r.Context(), request.ToInput())
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	w.Header().Set("Location", "/payment-references/"+formatInt64(result.Reference.ID))
	respond.JSON(w, r, http.StatusCreated, dto.NewPaymentReferenceResponse(result.Reference, result.ObservedAt))
}

// List handles GET /payment-references.
//
// Filters are optional: business_id, status and external_reference. The status
// filter matches the effective status, so ?status=expired also returns references
// still stored as pending whose date has passed.
func (h *PaymentReference) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// "business" is accepted as an alias of "business_id" because that is the name
	// the spec uses when describing the filters.
	businessID, err := queryInt64(query, "business_id", "business")
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	limit, err := queryInt(query, "limit", apppaymentref.DefaultLimit)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	offset, err := queryInt(query, "offset", 0)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	filter := apppaymentref.Filter{
		BusinessID:        businessID,
		ExternalReference: queryString(query, "external_reference"),
		Limit:             limit,
		Offset:            offset,
	}

	if rawStatus := queryString(query, "status"); rawStatus != nil {
		status, err := domain.ParseStatus(*rawStatus)
		if err != nil {
			respond.Error(w, r, err)
			return
		}
		filter.Status = &status
	}

	result, err := h.list.Execute(r.Context(), filter)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, r, http.StatusOK, dto.NewPaymentReferenceListResponse(result))
}

// Get handles GET /payment-references/{id}, answering 404 when it does not exist.
func (h *PaymentReference) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	result, err := h.get.Execute(r.Context(), id)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, r, http.StatusOK, dto.NewPaymentReferenceResponse(result.Reference, result.ObservedAt))
}

// Cancel handles POST /payment-references/{id}/cancel.
//
// 200 with the updated resource when it is cancelled, and also when it was already
// cancelled: cancelling is idempotent, so a retry from a flaky client is not an
// error. 409 for a paid or expired reference, 404 when it does not exist.
func (h *PaymentReference) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	result, err := h.cancel.Execute(r.Context(), id)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, r, http.StatusOK, dto.NewPaymentReferenceResponse(result.Reference, result.ObservedAt))
}
