-- Businesses are the parties that issue payment references.
CREATE TABLE businesses (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL CHECK (length(btrim(name)) > 0 AND length(name) <= 120),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
