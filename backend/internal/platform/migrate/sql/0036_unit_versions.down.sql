BEGIN;
ALTER TABLE purchase_units DROP COLUMN version;
COMMIT;
