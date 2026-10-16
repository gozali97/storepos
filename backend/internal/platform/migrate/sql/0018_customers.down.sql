BEGIN;
ALTER TABLE sales DROP COLUMN customer_id;
DROP TABLE customers;
COMMIT;
