BEGIN;
-- Rollback refuses to erase posted movement history.
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE','SALE_VOID','RETURN_RESTOCK','RETURN_DAMAGED','ADJUSTMENT_IN','ADJUSTMENT_OUT'));
DROP TABLE stock_count_lines;
DROP TABLE stock_counts;
COMMIT;
