BEGIN;
ALTER TABLE sale_returns DROP COLUMN delivery_fee_cents;
COMMIT;
