BEGIN;
DROP TABLE goods_receipt_lines;
DROP TABLE goods_receipts;
DROP TABLE purchase_units;
DROP FUNCTION purchase_units_lock_factor();
ALTER TABLE inventory_balances DROP COLUMN average_cost_cents;
COMMIT;
