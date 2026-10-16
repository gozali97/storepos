BEGIN;
DROP INDEX products_demo;
ALTER TABLE products DROP COLUMN is_demo;
COMMIT;
