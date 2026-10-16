BEGIN;
ALTER TABLE inventory_balances ADD COLUMN reserved_milli BIGINT NOT NULL DEFAULT 0 CHECK(reserved_milli>=0);
CREATE TABLE restaurant_stock_reservations (
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 line_id UUID NOT NULL,
 variant_id UUID NOT NULL,
 qty_milli BIGINT NOT NULL CHECK(qty_milli>0),
 status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','released','consumed')),
 PRIMARY KEY(business_id,line_id),
 FOREIGN KEY(business_id,line_id) REFERENCES restaurant_order_lines(business_id,id),
 FOREIGN KEY(business_id,branch_id,variant_id) REFERENCES inventory_balances(business_id,branch_id,variant_id)
);
COMMIT;
