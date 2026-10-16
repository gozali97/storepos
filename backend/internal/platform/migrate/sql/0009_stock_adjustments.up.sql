BEGIN;
ALTER TABLE stock_movements ADD COLUMN reason TEXT, ADD COLUMN note TEXT, ADD COLUMN approved_by UUID REFERENCES users(id);
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE','SALE_VOID','RETURN_RESTOCK','RETURN_DAMAGED','ADJUSTMENT_IN','ADJUSTMENT_OUT'));
COMMIT;
