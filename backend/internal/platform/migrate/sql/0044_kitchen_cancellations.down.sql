BEGIN;
DROP INDEX restaurant_cancel_report;
ALTER TABLE restaurant_order_lines DROP COLUMN cancelled_at,DROP COLUMN cancelled_by,DROP COLUMN cancel_approved_by,DROP COLUMN cancel_reason,DROP COLUMN cancelled_from_status;
COMMIT;
