package handler

import (
	"net/http"

	appbusiness "paymentlinks/internal/application/business"
	"paymentlinks/internal/transport/http/dto"
	"paymentlinks/internal/transport/http/respond"
)

// Business holds the business endpoints. Only the listing is exposed: the spec
// defines no way to create a business, so they are seeded (see seeds/).
type Business struct {
	list *appbusiness.ListBusinesses
}

// NewBusiness wires the handler with its use case.
func NewBusiness(list *appbusiness.ListBusinesses) *Business {
	return &Business{list: list}
}

// List handles GET /businesses, which the dashboard uses to fill the selector on
// the create form.
func (h *Business) List(w http.ResponseWriter, r *http.Request) {
	businesses, err := h.list.Execute(r.Context())
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, r, http.StatusOK, dto.NewBusinessListResponse(businesses))
}
