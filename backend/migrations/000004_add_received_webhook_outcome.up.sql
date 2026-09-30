-- A delivery is recorded before it is evaluated, so that the UNIQUE index on
-- event_id decides whether the delivery is new before any work is done. That
-- first INSERT needs a provisional outcome.
--
-- 'received' is transient by construction: the insert and the final outcome happen
-- in the same transaction, so a committed row always carries a final outcome. A
-- committed 'received' row would mean the transaction boundary was broken.
ALTER TABLE webhook_events DROP CONSTRAINT webhook_events_outcome_check;

ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_outcome_check CHECK (outcome IN (
    'received',
    'applied',
    'duplicate',
    'amount_mismatch',
    'reference_not_found',
    'conflict_cancelled',
    'conflict_other_transaction',
    'paid_after_expiration'
));
