BEGIN;
ALTER TABLE sale_return_lines DROP COLUMN restock_cost_cents,DROP COLUMN returned_cost_cents;
ALTER TABLE sale_lines DROP COLUMN cost_cents,DROP COLUMN product_id,DROP COLUMN category_id;
COMMIT;
