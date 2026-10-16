BEGIN;
CREATE TABLE restaurant_service_charges (
 business_id UUID NOT NULL,
 branch_id UUID NOT NULL,
 order_type TEXT NOT NULL CHECK(order_type IN ('DINE_IN','TAKEAWAY','DELIVERY_MANUAL')),
 rate_bps INT NOT NULL CHECK(rate_bps BETWEEN 0 AND 10000),
 include_in_tax_base BOOLEAN NOT NULL,
 version INT NOT NULL DEFAULT 1,
 PRIMARY KEY(business_id,branch_id,order_type),
 FOREIGN KEY(business_id,branch_id) REFERENCES branches(business_id,id)
);
ALTER TABLE restaurant_orders ADD COLUMN closed_at TIMESTAMPTZ,ADD COLUMN cancel_reason TEXT,ADD COLUMN cancelled_by UUID REFERENCES users(id);
ALTER TABLE restaurant_tables ADD COLUMN cleaned_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp();
COMMIT;
