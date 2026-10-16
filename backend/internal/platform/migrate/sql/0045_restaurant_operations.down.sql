BEGIN;
ALTER TABLE restaurant_tables DROP COLUMN cleaned_at;
ALTER TABLE restaurant_orders DROP COLUMN closed_at,DROP COLUMN cancel_reason,DROP COLUMN cancelled_by;
DROP TABLE restaurant_service_charges;
COMMIT;
