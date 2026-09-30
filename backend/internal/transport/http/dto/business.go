package dto

import (
	"time"

	domain "paymentlinks/internal/domain/business"
)

// BusinessResponse is the wire representation of a business.
type BusinessResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// BusinessListResponse is the body of GET /businesses. It carries no pagination
// fields because the endpoint is not paginated.
type BusinessListResponse struct {
	Items []BusinessResponse `json:"items"`
}

// NewBusinessListResponse maps the businesses for the dashboard selector.
func NewBusinessListResponse(businesses []*domain.Business) BusinessListResponse {
	items := make([]BusinessResponse, 0, len(businesses))
	for _, business := range businesses {
		items = append(items, BusinessResponse{
			ID:        business.ID,
			Name:      business.Name,
			CreatedAt: business.CreatedAt,
		})
	}
	return BusinessListResponse{Items: items}
}
