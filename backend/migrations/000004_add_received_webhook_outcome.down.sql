-- Rolling back fails if any row still carries the provisional outcome, which is
-- the correct behaviour: those rows would become unreadable by the old contract.
ALTER TABLE webhook_events DROP CONSTRAINT webhook_events_outcome_check;

ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_outcome_check CHECK (outcome IN (
    'applied',
    'duplicate',
    'amount_mismatch',
    'reference_not_found',
    'conflict_cancelled',
    'conflict_other_transaction',
    'paid_after_expiration'
));
