BEGIN;
ALTER TABLE restaurant_order_lines DROP COLUMN modifiers,DROP COLUMN modifier_delta_cents;
COMMIT;
