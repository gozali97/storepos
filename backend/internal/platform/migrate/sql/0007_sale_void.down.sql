BEGIN;
-- Rollback fails safely if voided transactions exist; never rewrite their history.
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE'));
ALTER TABLE payments DROP COLUMN status, DROP COLUMN manual_refund_required;
ALTER TABLE sales DROP CONSTRAINT sales_status_check;
ALTER TABLE sales ADD CONSTRAINT sales_status_check CHECK(status='completed');
ALTER TABLE sales DROP COLUMN voided_at,DROP COLUMN voided_by,DROP COLUMN void_approved_by,DROP COLUMN void_reason;
COMMIT;
