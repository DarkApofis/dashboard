package postgres_test

// Integration tests for the PostgreSQL adapters.
//
// The database comes from TestMain (see main_test.go): TEST_DATABASE_URL when it is
// set, a throwaway container otherwise. Nothing to start by hand.
//
// They exist because the in-memory fakes cannot catch what actually breaks in this
// layer: SQL that does not compile, a parameter PostgreSQL refuses to type, a
// constraint whose name drifted away from the error translation, a transaction that
// does not roll back.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	apppaymentref "paymentlinks/internal/application/paymentref"
	"paymentlinks/internal/domain/business"
	"paymentlinks/internal/domain/money"
	domain "paymentlinks/internal/domain/paymentref"
	"paymentlinks/internal/infrastructure/postgres"
)

var reference = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

type suite struct {
	pool       *pgxpool.Pool
	references *postgres.PaymentReferenceRepository
	businesses *postgres.BusinessRepository
	events     *postgres.WebhookEventRepository
	tx         *postgres.TxManager
	businessID int64
}

func newSuite(t *testing.T) *suite {
	t.Helper()

	url := requireDatabase(t)

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{URL: url, MaxConns: 4, ConnectRetries: 3}, nil)
	if err != nil {
		t.Fatalf("connect to the test database: %v", err)
	}
	t.Cleanup(pool.Close)

	// A clean slate per test, so ordering and totals are deterministic.
	if _, err := pool.Exec(ctx, `TRUNCATE webhook_events, payment_references, businesses RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	s := &suite{
		pool:       pool,
		references: postgres.NewPaymentReferenceRepository(pool),
		businesses: postgres.NewBusinessRepository(pool),
		events:     postgres.NewWebhookEventRepository(pool),
		tx:         postgres.NewTxManager(pool),
	}

	seed, err := business.New("Acme Logistics", reference)
	if err != nil {
		t.Fatalf("build business: %v", err)
	}
	if err := s.businesses.Create(ctx, seed); err != nil {
		t.Fatalf("seed business: %v", err)
	}
	s.businessID = seed.ID
	return s
}

func (s *suite) insert(t *testing.T, externalReference string, expiresAt time.Time) *domain.PaymentReference {
	t.Helper()

	ref, err := domain.New(domain.NewParams{
		BusinessID:        s.businessID,
		ExternalReference: externalReference,
		Amount:            money.MustFromString("125000.50"),
		Description:       "Factura " + externalReference,
		ExpiresAt:         expiresAt,
		Now:               reference,
	})
	if err != nil {
		t.Fatalf("build reference %s: %v", externalReference, err)
	}
	if err := s.references.Create(context.Background(), ref); err != nil {
		t.Fatalf("insert reference %s: %v", externalReference, err)
	}
	return ref
}

func TestCreateAndReadRoundTrip(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	created := s.insert(t, "INV-10023", reference.Add(24*time.Hour))
	if created.ID == 0 {
		t.Fatal("the insert must assign an id")
	}

	found, err := s.references.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	// The amount is the reason NUMERIC and a decimal type are used: it must come
	// back bit for bit, not merely close.
	if found.Amount.String() != "125000.50" {
		t.Errorf("amount: got %q, want 125000.50", found.Amount.String())
	}
	if found.Status != domain.StatusPending {
		t.Errorf("status: got %q, want pending", found.Status)
	}
	if found.Description != "Factura INV-10023" {
		t.Errorf("description: got %q", found.Description)
	}
	if found.CreatedAt.Location() != time.UTC || created.CreatedAt.Location() != time.UTC {
		t.Errorf("timestamps must be UTC, got %v on read and %v on write",
			found.CreatedAt.Location(), created.CreatedAt.Location())
	}
	if found.PaidAt != nil || found.CancelledAt != nil || found.RequiresReview {
		t.Errorf("a new reference must have no audit marks: %+v", found)
	}
}

func TestCreateTranslatesConstraintViolations(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	s.insert(t, "INV-DUP", reference.Add(24*time.Hour))

	t.Run("duplicate external reference", func(t *testing.T) {
		duplicate, err := domain.New(domain.NewParams{
			BusinessID:        s.businessID,
			ExternalReference: "INV-DUP",
			Amount:            money.MustFromString("1.00"),
			ExpiresAt:         reference.Add(24 * time.Hour),
			Now:               reference,
		})
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		if err := s.references.Create(ctx, duplicate); !errors.Is(err, domain.ErrDuplicateReference) {
			t.Fatalf("got error %v, want ErrDuplicateReference", err)
		}
	})

	t.Run("unknown business", func(t *testing.T) {
		orphan, err := domain.New(domain.NewParams{
			BusinessID:        999999,
			ExternalReference: "INV-ORPHAN",
			Amount:            money.MustFromString("1.00"),
			ExpiresAt:         reference.Add(24 * time.Hour),
			Now:               reference,
		})
		if err != nil {
			t.Fatalf("build: %v", err)
		}

		if err := s.references.Create(ctx, orphan); !errors.Is(err, business.ErrNotFound) {
			t.Fatalf("got error %v, want business.ErrNotFound", err)
		}
	})
}

func TestGetByIDNotFound(t *testing.T) {
	s := newSuite(t)

	if _, err := s.references.GetByID(context.Background(), 424242); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got error %v, want ErrNotFound", err)
	}
}

// Every filter combination is exercised because the WHERE clause and its
// placeholder numbering are built dynamically: a combination nobody runs is a
// combination that does not work.
func TestListFilterCombinations(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	s.insert(t, "INV-1", reference.Add(24*time.Hour))
	s.insert(t, "INV-2", reference.Add(48*time.Hour))
	overdue := s.insert(t, "INV-OLD", reference.Add(time.Hour))

	otherBusiness, err := business.New("Panaderia La Esquina", reference)
	if err != nil {
		t.Fatalf("build business: %v", err)
	}
	if err := s.businesses.Create(ctx, otherBusiness); err != nil {
		t.Fatalf("create business: %v", err)
	}

	pending := domain.StatusPending
	expired := domain.StatusExpired
	externalRef := "INV-2"
	unknownRef := "INV-NOPE"

	// after is late enough that INV-OLD is past due but the other two are not.
	after := reference.Add(2 * time.Hour)

	cases := []struct {
		name      string
		filter    apppaymentref.Filter
		now       time.Time
		wantTotal int
	}{
		{name: "no filters at all", filter: apppaymentref.Filter{}, now: reference, wantTotal: 3},
		{name: "business only", filter: apppaymentref.Filter{BusinessID: &s.businessID}, now: reference, wantTotal: 3},
		{name: "other business only", filter: apppaymentref.Filter{BusinessID: &otherBusiness.ID}, now: reference, wantTotal: 0},
		{name: "status only", filter: apppaymentref.Filter{Status: &pending}, now: reference, wantTotal: 3},
		{name: "external reference only", filter: apppaymentref.Filter{ExternalReference: &externalRef}, now: reference, wantTotal: 1},
		{name: "unknown external reference", filter: apppaymentref.Filter{ExternalReference: &unknownRef}, now: reference, wantTotal: 0},
		{name: "business and status", filter: apppaymentref.Filter{BusinessID: &s.businessID, Status: &pending}, now: reference, wantTotal: 3},
		{name: "status and external reference", filter: apppaymentref.Filter{Status: &pending, ExternalReference: &externalRef}, now: reference, wantTotal: 1},
		{name: "all three", filter: apppaymentref.Filter{BusinessID: &s.businessID, Status: &pending, ExternalReference: &externalRef}, now: reference, wantTotal: 1},
		{name: "effective expiration without the job", filter: apppaymentref.Filter{Status: &expired}, now: after, wantTotal: 1},
		{name: "pending excludes the overdue one", filter: apppaymentref.Filter{Status: &pending}, now: after, wantTotal: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := s.references.List(ctx, tc.filter, tc.now)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if page.Total != tc.wantTotal {
				t.Errorf("total: got %d, want %d", page.Total, tc.wantTotal)
			}
			if len(page.Items) != tc.wantTotal {
				t.Errorf("items: got %d, want %d", len(page.Items), tc.wantTotal)
			}
		})
	}

	t.Run("expired filter returns the overdue reference", func(t *testing.T) {
		page, err := s.references.List(ctx, apppaymentref.Filter{Status: &expired}, after)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(page.Items) != 1 || page.Items[0].ID != overdue.ID {
			t.Fatalf("got %+v, want the overdue reference id=%d", page.Items, overdue.ID)
		}
		// The stored status is untouched: reading never writes.
		if page.Items[0].Status != domain.StatusPending {
			t.Errorf("stored status: got %q, want pending", page.Items[0].Status)
		}
	})
}

func TestListPaginationAndOrder(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	for _, external := range []string{"INV-1", "INV-2", "INV-3"} {
		s.insert(t, external, reference.Add(24*time.Hour))
	}

	page, err := s.references.List(ctx, apppaymentref.Filter{Limit: 2}, reference)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("items: got %d, want 2", len(page.Items))
	}
	if page.Total != 3 {
		t.Errorf("total must ignore pagination: got %d, want 3", page.Total)
	}

	second, err := s.references.List(ctx, apppaymentref.Filter{Limit: 2, Offset: 2}, reference)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(second.Items) != 1 {
		t.Errorf("second page: got %d items, want 1", len(second.Items))
	}

	// All three share created_at, so the id tiebreaker decides: newest first.
	if page.Items[0].ID < page.Items[1].ID {
		t.Errorf("order must be newest first, got ids %d then %d", page.Items[0].ID, page.Items[1].ID)
	}
}

func TestUpdateWithStatusGuard(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	created := s.insert(t, "INV-1", reference.Add(24*time.Hour))

	t.Run("applies the update when the status still matches", func(t *testing.T) {
		if err := created.Cancel(reference); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		if err := s.references.UpdateWithStatusGuard(ctx, created, domain.StatusPending); err != nil {
			t.Fatalf("UpdateWithStatusGuard: %v", err)
		}

		found, err := s.references.GetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if found.Status != domain.StatusCancelled || found.CancelledAt == nil {
			t.Errorf("got status %q cancelled_at %v", found.Status, found.CancelledAt)
		}
	})

	t.Run("refuses the update when the status moved", func(t *testing.T) {
		// The row is cancelled now, so a writer that still believes it is pending
		// must be rejected rather than overwrite a decision it did not see.
		err := s.references.UpdateWithStatusGuard(ctx, created, domain.StatusPending)
		if !errors.Is(err, domain.ErrConcurrentUpdate) {
			t.Fatalf("got error %v, want ErrConcurrentUpdate", err)
		}
	})
}

func TestExpireDue(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	overdue := s.insert(t, "INV-OLD", reference.Add(time.Hour))
	future := s.insert(t, "INV-NEW", reference.Add(72*time.Hour))

	after := reference.Add(2 * time.Hour)
	moved, err := s.references.ExpireDue(ctx, after, 100)
	if err != nil {
		t.Fatalf("ExpireDue: %v", err)
	}
	if moved != 1 {
		t.Fatalf("moved: got %d, want 1", moved)
	}

	if found, _ := s.references.GetByID(ctx, overdue.ID); found.Status != domain.StatusExpired {
		t.Errorf("overdue reference: got status %q, want expired", found.Status)
	}
	if found, _ := s.references.GetByID(ctx, future.ID); found.Status != domain.StatusPending {
		t.Errorf("future reference: got status %q, want pending", found.Status)
	}

	// Running it again must be a no-op: the job has to be safe to re-run.
	again, err := s.references.ExpireDue(ctx, after, 100)
	if err != nil {
		t.Fatalf("second ExpireDue: %v", err)
	}
	if again != 0 {
		t.Errorf("second run moved %d rows, want 0", again)
	}
}

// The transaction boundary is the whole point of the TxManager, and it is the one
// thing the in-memory double cannot verify.
func TestTxManagerRollsBack(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	created := s.insert(t, "INV-1", reference.Add(24*time.Hour))

	sentinel := errors.New("use case failed after writing")

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		locked, err := s.references.LockByID(txCtx, created.ID)
		if err != nil {
			return err
		}
		if err := locked.Cancel(reference); err != nil {
			return err
		}
		if err := s.references.UpdateWithStatusGuard(txCtx, locked, domain.StatusPending); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got error %v, want the sentinel", err)
	}

	found, err := s.references.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.Status != domain.StatusPending {
		t.Errorf("the cancellation must have been rolled back, got status %q", found.Status)
	}
}

func TestTxManagerCommits(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	created := s.insert(t, "INV-1", reference.Add(24*time.Hour))

	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		locked, err := s.references.LockByID(txCtx, created.ID)
		if err != nil {
			return err
		}
		if err := locked.Cancel(reference); err != nil {
			return err
		}
		return s.references.UpdateWithStatusGuard(txCtx, locked, domain.StatusPending)
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	found, err := s.references.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if found.Status != domain.StatusCancelled {
		t.Errorf("status: got %q, want cancelled", found.Status)
	}
}

func TestWebhookEventIdempotency(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	created := s.insert(t, "INV-10023", reference.Add(24*time.Hour))

	event := &domain.WebhookEvent{
		EventID:            "evt_983829",
		ExternalReference:  created.ExternalReference,
		PaymentReferenceID: &created.ID,
		Amount:             money.MustFromString("125000.50"),
		TransactionID:      "tx_883721",
		PaidAt:             &reference,
		Payload:            []byte(`{"event_id":"evt_983829"}`),
		Outcome:            domain.OutcomeApplied,
		ReceivedAt:         reference,
	}

	if err := s.events.Insert(ctx, event); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// The same event_id again is what the provider sends when it did not get our
	// response. The unique index, not application code, is what stops it.
	redelivery := *event
	redelivery.ID = 0
	if err := s.events.Insert(ctx, &redelivery); !errors.Is(err, domain.ErrDuplicateEvent) {
		t.Fatalf("got error %v, want ErrDuplicateEvent", err)
	}

	stored, err := s.events.GetByEventID(ctx, "evt_983829")
	if err != nil {
		t.Fatalf("GetByEventID: %v", err)
	}
	if stored.Amount.String() != "125000.50" || stored.Outcome != domain.OutcomeApplied {
		t.Errorf("unexpected stored event: %+v", stored)
	}
}

func TestBusinessRepository(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	t.Run("ExistsByID", func(t *testing.T) {
		exists, err := s.businesses.ExistsByID(ctx, s.businessID)
		if err != nil || !exists {
			t.Fatalf("got exists=%v err=%v, want true", exists, err)
		}

		exists, err = s.businesses.ExistsByID(ctx, 999999)
		if err != nil || exists {
			t.Fatalf("got exists=%v err=%v, want false", exists, err)
		}
	})

	t.Run("List is alphabetical", func(t *testing.T) {
		second, err := business.New("AAA Primero", reference)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if err := s.businesses.Create(ctx, second); err != nil {
			t.Fatalf("create: %v", err)
		}

		businesses, err := s.businesses.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(businesses) != 2 || businesses[0].Name != "AAA Primero" {
			t.Errorf("unexpected order: %+v", businesses)
		}
	})

	t.Run("GetByID reports a missing business", func(t *testing.T) {
		if _, err := s.businesses.GetByID(ctx, 999999); !errors.Is(err, business.ErrNotFound) {
			t.Fatalf("got error %v, want business.ErrNotFound", err)
		}
	})
}

// The regression this guards: COUNT(*) OVER () rides along with the rows, so a page
// past the end used to come back with a total of zero, and a client could not tell
// "nothing matches" from "that page does not exist".
func TestListReportsTheTotalOnAnEmptyPage(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	for _, external := range []string{"INV-1", "INV-2", "INV-3"} {
		s.insert(t, external, reference.Add(24*time.Hour))
	}

	pending := domain.StatusPending

	cases := []struct {
		name      string
		filter    apppaymentref.Filter
		wantItems int
		wantTotal int
	}{
		{name: "first page", filter: apppaymentref.Filter{Limit: 2}, wantItems: 2, wantTotal: 3},
		{name: "last partial page", filter: apppaymentref.Filter{Limit: 2, Offset: 2}, wantItems: 1, wantTotal: 3},
		{name: "one past the end", filter: apppaymentref.Filter{Limit: 2, Offset: 4}, wantItems: 0, wantTotal: 3},
		{name: "far past the end", filter: apppaymentref.Filter{Limit: 20, Offset: 500}, wantItems: 0, wantTotal: 3},
		{name: "past the end, filtered", filter: apppaymentref.Filter{Status: &pending, Offset: 500}, wantItems: 0, wantTotal: 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := s.references.List(ctx, tc.filter, reference)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(page.Items) != tc.wantItems {
				t.Errorf("items: got %d, want %d", len(page.Items), tc.wantItems)
			}
			if page.Total != tc.wantTotal {
				t.Errorf("total: got %d, want %d", page.Total, tc.wantTotal)
			}
		})
	}

	t.Run("a filter that matches nothing still reports zero", func(t *testing.T) {
		unknown := "INV-NOPE"
		page, err := s.references.List(ctx, apppaymentref.Filter{ExternalReference: &unknown}, reference)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if page.Total != 0 {
			t.Errorf("total: got %d, want 0", page.Total)
		}
	})
}
