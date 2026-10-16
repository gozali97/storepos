BEGIN;
ALTER TABLE sales DROP COLUMN receipt_header_snapshot;
COMMIT;
