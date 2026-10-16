BEGIN;
DROP INDEX sales_one_restaurant_order;
ALTER TABLE sales DROP COLUMN restaurant_order_id,DROP COLUMN order_type,DROP COLUMN service_charge_rate_bps,DROP COLUMN service_charge_in_tax_base;
ALTER TABLE sale_lines DROP COLUMN modifiers;
COMMIT;
