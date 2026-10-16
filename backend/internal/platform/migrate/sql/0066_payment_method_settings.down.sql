BEGIN;
ALTER TABLE businesses DROP COLUMN payment_methods_version;
COMMIT;
