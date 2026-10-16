BEGIN;
ALTER TABLE restaurant_orders DROP COLUMN contact_name,DROP COLUMN contact_phone,DROP COLUMN queue_label;
COMMIT;
