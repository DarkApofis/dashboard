-- Payment references are what a business shares with its customer so that the
-- financial provider can collect a specific amount.
CREATE TABLE payment_references (
    id                  BIGSERIAL PRIMARY KEY,
    business_id         BIGINT         NOT NULL REFERENCES businesses (id) ON DELETE RESTRICT,
    external_reference  TEXT           NOT NULL,
    -- NUMERIC, never a float: 125000.50 is not representable in binary, which
    -- would break both the webhook amount comparison and any SUM over the column.
    amount              NUMERIC(14, 2) NOT NULL CHECK (amount > 0),
    description         TEXT           CHECK (description IS NULL OR length(description) <= 500),
    status              TEXT           NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'paid', 'expired', 'cancelled')),
    expires_at          TIMESTAMPTZ    NOT NULL,
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ    NOT NULL DEFAULT now(),

    -- Audit columns: without them a status cannot be explained after the fact.
    paid_at             TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    paid_transaction_id TEXT,
    -- Set when a notification was accepted but needs human reconciliation
    -- (payment after expiration, amount mismatch, payment for a cancelled one).
    requires_review     BOOLEAN        NOT NULL DEFAULT FALSE,
    review_reason       TEXT,

    -- Globally unique: the provider webhook only carries external_reference, with
    -- no business_id, so it must resolve to exactly one row. If the provider ever
    -- sends the business too, this relaxes to UNIQUE (business_id, external_reference).
    CONSTRAINT payment_references_external_reference_key UNIQUE (external_reference),

    -- The database also enforces the two invariants the entity guarantees, so a
    -- bug in a future code path cannot leave an unexplainable row behind.
    CONSTRAINT payment_references_paid_consistency
        CHECK ((status = 'paid') = (paid_at IS NOT NULL)),
    CONSTRAINT payment_references_cancelled_consistency
        CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL))
);

-- Listing filtered by business and status, newest first: the dashboard default.
CREATE INDEX payment_references_business_status_created_idx
    ON payment_references (business_id, status, created_at DESC);

-- Listing filtered by status alone, across every business.
CREATE INDEX payment_references_status_created_idx
    ON payment_references (status, created_at DESC);

-- Feeds the expiration job: only pending rows are ever scanned.
CREATE INDEX payment_references_pending_expires_at_idx
    ON payment_references (expires_at)
    WHERE status = 'pending';

-- Lookup by external_reference (webhook and dashboard filter) is already served
-- by the unique index above, so no extra index is created for it.
