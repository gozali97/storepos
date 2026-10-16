BEGIN;
ALTER TABLE restaurant_order_lines DROP COLUMN category_id,DROP COLUMN category_snapshot_legacy;
COMMIT;
