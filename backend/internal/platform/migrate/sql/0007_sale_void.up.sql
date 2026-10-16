BEGIN;
ALTER TABLE sales DROP CONSTRAINT sales_status_check;
ALTER TABLE sales ADD CONSTRAINT sales_status_check CHECK(status IN ('completed','voided'));
ALTER TABLE sales ADD COLUMN voided_at TIMESTAMPTZ,
 ADD COLUMN voided_by UUID REFERENCES users(id),
 ADD COLUMN void_approved_by UUID REFERENCES users(id),
 ADD COLUMN void_reason TEXT;
ALTER TABLE payments ADD COLUMN status TEXT NOT NULL DEFAULT 'confirmed' CHECK(status IN ('confirmed','reversed')),
 ADD COLUMN manual_refund_required BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE','SALE_VOID'));
COMMIT;
