BEGIN;
-- Fails if adjustment history exists; never silently erase stock history.
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE','SALE_VOID','RETURN_RESTOCK','RETURN_DAMAGED'));
ALTER TABLE stock_movements DROP COLUMN reason,DROP COLUMN note,DROP COLUMN approved_by;
COMMIT;
