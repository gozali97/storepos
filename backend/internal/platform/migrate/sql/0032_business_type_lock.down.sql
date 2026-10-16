BEGIN;
DROP TRIGGER businesses_type_lock ON businesses;
DROP FUNCTION businesses_lock_type();
ALTER TABLE businesses DROP COLUMN org_version;
-- Historical first_sale_completed_at backfill is intentionally preserved.
COMMIT;
