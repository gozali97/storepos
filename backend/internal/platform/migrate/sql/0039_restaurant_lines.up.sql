BEGIN;
CREATE TABLE restaurant_order_lines (
 id UUID PRIMARY KEY,
 business_id UUID NOT NULL,
 order_id UUID NOT NULL,
 variant_id UUID NOT NULL,
 qty_milli BIGINT NOT NULL CHECK(qty_milli>0),
 unit_price_cents BIGINT NOT NULL CHECK(unit_price_cents>=0),
 product_name TEXT NOT NULL,
 variant_name TEXT NOT NULL,
 base_unit TEXT NOT NULL,
 note TEXT NOT NULL DEFAULT '' CHECK(char_length(note)<=140),
 kitchen_status TEXT NOT NULL DEFAULT 'NOT_SENT' CHECK(kitchen_status IN ('NOT_SENT','SENT','IN_PROGRESS','READY','SERVED','CANCELLED')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(business_id,id),
 FOREIGN KEY(business_id,order_id) REFERENCES restaurant_orders(business_id,id),
 FOREIGN KEY(business_id,variant_id) REFERENCES variants(business_id,id)
);
CREATE INDEX restaurant_order_lines_order ON restaurant_order_lines(business_id,order_id,id);
COMMIT;
