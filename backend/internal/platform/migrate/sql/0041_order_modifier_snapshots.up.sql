BEGIN;
ALTER TABLE restaurant_order_lines ADD COLUMN modifiers JSONB NOT NULL DEFAULT '[]'::jsonb CHECK(jsonb_typeof(modifiers)='array'), ADD COLUMN modifier_delta_cents BIGINT NOT NULL DEFAULT 0 CHECK(modifier_delta_cents>=0);
COMMIT;
