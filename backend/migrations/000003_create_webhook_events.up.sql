-- Every notification received from the financial provider is recorded here,
-- including the ones that changed nothing: those are precisely the ones worth
-- auditing. The unique index on event_id is the idempotency mechanism, so a
-- redelivery is rejected by the database instead of by application locking.
CREATE TABLE webhook_events (
    id                   BIGSERIAL PRIMARY KEY,
    event_id             TEXT           NOT NULL,
    external_reference   TEXT           NOT NULL,
    payment_reference_id BIGINT         REFERENCES payment_references (id) ON DELETE SET NULL,
    amount               NUMERIC(14, 2),
    transaction_id       TEXT,
    paid_at              TIMESTAMPTZ,
    -- Raw body, kept verbatim so an event can be audited or replayed even if our
    -- parsing rules change later.
    payload              JSONB          NOT NULL,
    outcome              TEXT           NOT NULL CHECK (outcome IN (
                                'applied',
                                'duplicate',
                                'amount_mismatch',
                                'reference_not_found',
                                'conflict_cancelled',
                                'conflict_other_transaction',
                                'paid_after_expiration'
                            )),
    received_at          TIMESTAMPTZ    NOT NULL DEFAULT now(),

    CONSTRAINT webhook_events_event_id_key UNIQUE (event_id)
);

-- Reconciling the history of one reference, by either identifier.
CREATE INDEX webhook_events_external_reference_idx ON webhook_events (external_reference);
CREATE INDEX webhook_events_payment_reference_id_idx ON webhook_events (payment_reference_id);

-- Finding what needs manual review.
CREATE INDEX webhook_events_outcome_received_idx ON webhook_events (outcome, received_at DESC);
