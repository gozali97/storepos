BEGIN;
ALTER TABLE sales ADD COLUMN receipt_header_snapshot JSONB;
-- Old receipts stay NULL: do not pretend today's configuration was historical.
COMMIT;
