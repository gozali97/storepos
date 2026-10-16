BEGIN;
ALTER TABLE sale_lines ADD COLUMN cost_cents BIGINT CHECK(cost_cents>=0), ADD COLUMN product_id UUID, ADD COLUMN category_id UUID;
UPDATE sale_lines l SET product_id=v.product_id,category_id=p.category_id FROM variants v JOIN products p ON p.business_id=v.business_id AND p.id=v.product_id WHERE v.business_id=l.business_id AND v.id=l.variant_id;
-- Historical HPP remains NULL; current HPP must not be substituted for historical cost.
ALTER TABLE sale_return_lines ADD COLUMN restock_cost_cents BIGINT CHECK(restock_cost_cents>=0), ADD COLUMN returned_cost_cents BIGINT CHECK(returned_cost_cents>=0);
COMMIT;
