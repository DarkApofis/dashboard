package inmemory

import (
	"context"
	"sort"
	"sync"
	"time"

	appbusiness "paymentlinks/internal/application/business"
	apppaymentref "paymentlinks/internal/application/paymentref"
	domain "paymentlinks/internal/domain/business"
)

// BusinessRepository is an in-memory business repository.
type BusinessRepository struct {
	mu     sync.Mutex
	byID   map[int64]domain.Business
	nextID int64
}

var (
	_ appbusiness.Repository       = (*BusinessRepository)(nil)
	_ apppaymentref.BusinessFinder = (*BusinessRepository)(nil)
)

// NewBusinessRepository builds an empty repository.
func NewBusinessRepository() *BusinessRepository {
	return &BusinessRepository{byID: make(map[int64]domain.Business), nextID: 1}
}

// Seed inserts businesses and returns their assigned ids, in the order given.
func (r *BusinessRepository) Seed(names ...string) []int64 {
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		business, err := domain.New(name, time.Now().UTC())
		if err != nil {
			panic("inmemory: invalid seed business name " + name)
		}
		if err := r.Create(context.Background(), business); err != nil {
			panic("inmemory: could not seed business " + name)
		}
		ids = append(ids, business.ID)
	}
	return ids
}

// Create stores a copy and assigns the next id.
func (r *BusinessRepository) Create(_ context.Context, business *domain.Business) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	business.ID = r.nextID
	r.nextID++
	r.byID[business.ID] = *business
	return nil
}

// GetByID returns a copy, or domain.ErrNotFound.
func (r *BusinessRepository) GetByID(_ context.Context, id int64) (*domain.Business, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &stored, nil
}

// ExistsByID reports whether the business exists.
func (r *BusinessRepository) ExistsByID(_ context.Context, id int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.byID[id]
	return ok, nil
}

// List returns every business, alphabetically.
func (r *BusinessRepository) List(_ context.Context) ([]*domain.Business, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	businesses := make([]*domain.Business, 0, len(r.byID))
	for _, stored := range r.byID {
		copied := stored
		businesses = append(businesses, &copied)
	}
	sort.Slice(businesses, func(i, j int) bool { return businesses[i].Name < businesses[j].Name })
	return businesses, nil
}
