BEGIN;
ALTER TABLE sales DROP COLUMN delivery_fee_cents;
COMMIT;
