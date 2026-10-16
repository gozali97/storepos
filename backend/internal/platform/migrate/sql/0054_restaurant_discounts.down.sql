BEGIN;
ALTER TABLE restaurant_order_lines DROP COLUMN discount_kind,DROP COLUMN discount_value;
ALTER TABLE restaurant_orders DROP COLUMN discount_kind,DROP COLUMN discount_value,DROP COLUMN discount_reason;
COMMIT;
