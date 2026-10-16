BEGIN;
ALTER TABLE restaurant_order_lines ADD COLUMN category_id UUID;
-- Legacy unsubmitted lines may use current category, but label their provenance separately.
ALTER TABLE restaurant_order_lines ADD COLUMN category_snapshot_legacy BOOLEAN NOT NULL DEFAULT false;
UPDATE restaurant_order_lines l SET category_id=p.category_id,category_snapshot_legacy=true FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE l.business_id=v.business_id AND l.variant_id=v.id;
COMMIT;
