BEGIN;
-- No history rewriting: rollback fails if return movements exist.
DROP TABLE refunds;
DROP TABLE sale_return_lines;
DROP TABLE sale_returns;
DROP INDEX sale_lines_business_id_id;
ALTER TABLE sales DROP COLUMN return_status;
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_kind_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_kind_check CHECK(kind IN ('RECEIPT','SALE','SALE_VOID'));
COMMIT;
