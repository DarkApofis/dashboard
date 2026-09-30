package dto

import (
	"time"

	apppaymentref "paymentlinks/internal/application/paymentref"
	domain "paymentlinks/internal/domain/paymentref"
)

// CreatePaymentReferenceRequest is the body of POST /payment-references.
//
// No `validate` tags and no required-field pointers: the fields are decoded here
// and every rule is checked by the domain entity, so there is exactly one place
// where "what is a valid payment reference" is defined.
type CreatePaymentReferenceRequest struct {
	BusinessID        int64     `json:"business_id"`
	ExternalReference string    `json:"external_reference"`
	Amount            Amount    `json:"amount"`
	Description       string    `json:"description"`
	ExpiresAt         time.Time `json:"expires_at"`
}

// ToInput maps the request onto the use case input.
func (r CreatePaymentReferenceRequest) ToInput() apppaymentref.CreateInput {
	return apppaymentref.CreateInput{
		BusinessID:        r.BusinessID,
		ExternalReference: r.ExternalReference,
		Amount:            r.Amount.Value,
		Description:       r.Description,
		ExpiresAt:         r.ExpiresAt,
	}
}

// PaymentReferenceResponse is the wire representation of a reference.
//
// Status is the effective status: a pending reference past its expiration date is
// reported as expired even before the periodic job writes it, so a client never
// sees a state it would have to recompute itself.
type PaymentReferenceResponse struct {
	ID                int64      `json:"id"`
	BusinessID        int64      `json:"business_id"`
	ExternalReference string     `json:"external_reference"`
	Amount            Amount     `json:"amount"`
	Description       string     `json:"description"`
	Status            string     `json:"status"`
	ExpiresAt         time.Time  `json:"expires_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	PaidAt            *time.Time `json:"paid_at"`
	CancelledAt       *time.Time `json:"cancelled_at"`
	TransactionID     string     `json:"transaction_id,omitempty"`
	// RequiresReview flags a reference a human should look at (paid after
	// expiring, amount mismatch). The dashboard renders it as a warning badge.
	RequiresReview bool   `json:"requires_review"`
	ReviewReason   string `json:"review_reason,omitempty"`
}

// NewPaymentReferenceResponse maps a domain entity, resolving its status at the
// instant the use case observed.
func NewPaymentReferenceResponse(reference *domain.PaymentReference, observedAt time.Time) PaymentReferenceResponse {
	return PaymentReferenceResponse{
		ID:                reference.ID,
		BusinessID:        reference.BusinessID,
		ExternalReference: reference.ExternalReference,
		Amount:            NewAmount(reference.Amount),
		Description:       reference.Description,
		Status:            string(reference.EffectiveStatus(observedAt)),
		ExpiresAt:         reference.ExpiresAt,
		CreatedAt:         reference.CreatedAt,
		UpdatedAt:         reference.UpdatedAt,
		PaidAt:            reference.PaidAt,
		CancelledAt:       reference.CancelledAt,
		TransactionID:     reference.PaidTransactionID,
		RequiresReview:    reference.RequiresReview,
		ReviewReason:      reference.ReviewReason,
	}
}

// PaymentReferenceListResponse is the body of GET /payment-references. Total is the
// number of matches ignoring pagination, which is what the dashboard needs to draw
// the pager.
type PaymentReferenceListResponse struct {
	Items  []PaymentReferenceResponse `json:"items"`
	Total  int                        `json:"total"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
}

// NewPaymentReferenceListResponse maps a page of results.
func NewPaymentReferenceListResponse(result apppaymentref.ListResult) PaymentReferenceListResponse {
	// Never nil: an empty list must serialise as [] and not as null.
	items := make([]PaymentReferenceResponse, 0, len(result.Page.Items))
	for _, reference := range result.Page.Items {
		items = append(items, NewPaymentReferenceResponse(reference, result.ObservedAt))
	}

	return PaymentReferenceListResponse{
		Items:  items,
		Total:  result.Page.Total,
		Limit:  result.Page.Limit,
		Offset: result.Page.Offset,
	}
}
