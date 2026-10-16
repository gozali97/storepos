BEGIN;
ALTER TABLE restaurant_order_lines ADD COLUMN cancelled_at TIMESTAMPTZ,
 ADD COLUMN cancelled_by UUID REFERENCES users(id), ADD COLUMN cancel_approved_by UUID REFERENCES users(id),
 ADD COLUMN cancel_reason TEXT, ADD COLUMN cancelled_from_status TEXT;
CREATE INDEX restaurant_cancel_report ON restaurant_order_lines(business_id,cancelled_at,id) WHERE cancelled_at IS NOT NULL;
COMMIT;
