-- Demo businesses so the dashboard is usable right after the first start.
--
-- The spec defines no endpoint for creating businesses, so they are seeded. The
-- script is idempotent: running it again inserts nothing.
INSERT INTO businesses (name)
SELECT name
FROM (VALUES
    ('Acme Logistics S.A.S.'),
    ('Panaderia La Esquina'),
    ('Clinica Santa Marta')
) AS seed (name)
WHERE NOT EXISTS (
    SELECT 1 FROM businesses b WHERE b.name = seed.name
);
