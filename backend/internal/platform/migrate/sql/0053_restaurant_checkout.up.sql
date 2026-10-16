BEGIN;
ALTER TABLE sale_lines ADD COLUMN modifiers JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE sales ADD COLUMN restaurant_order_id UUID,ADD COLUMN order_type TEXT,ADD COLUMN service_charge_rate_bps INT,ADD COLUMN service_charge_in_tax_base BOOLEAN;
ALTER TABLE sales ADD CONSTRAINT sales_restaurant_order_fk FOREIGN KEY(business_id,restaurant_order_id) REFERENCES restaurant_orders(business_id,id);
CREATE UNIQUE INDEX sales_one_restaurant_order ON sales(business_id,restaurant_order_id) WHERE restaurant_order_id IS NOT NULL;
COMMIT;
