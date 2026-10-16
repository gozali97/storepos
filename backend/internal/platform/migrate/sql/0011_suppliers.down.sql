BEGIN;
ALTER TABLE goods_receipts DROP COLUMN supplier_id;
DROP TABLE suppliers;
COMMIT;
